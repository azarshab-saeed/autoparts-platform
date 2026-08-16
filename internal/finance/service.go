package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) ListCustomerBalances(ctx context.Context, tenantID, storeID uuid.UUID, q string, limit, offset int) ([]PartyBalance, error) {
	like := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	rows, err := s.db.Query(ctx, `
		SELECT c.id, COALESCE(c.code,''), c.name, COALESCE(c.phone,''),
		       COALESCE(SUM(e.debit-e.credit),0)::bigint AS balance
		FROM customers c
		LEFT JOIN party_ledger_entries e
		  ON e.tenant_id=c.tenant_id AND e.store_id=c.store_id AND e.customer_id=c.id
		WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.deleted_at IS NULL
		  AND ($3='%%' OR lower(c.name) LIKE $3 OR COALESCE(c.phone,'') LIKE $3 OR lower(COALESCE(c.code,'')) LIKE $3)
		GROUP BY c.id,c.code,c.name,c.phone
		ORDER BY balance DESC, c.name
		LIMIT $4 OFFSET $5`, tenantID, storeID, like, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PartyBalance, 0)
	for rows.Next() {
		var x PartyBalance
		if err := rows.Scan(&x.ID, &x.Code, &x.Name, &x.Phone, &x.Balance); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) ListSupplierBalances(ctx context.Context, tenantID, storeID uuid.UUID, q string, limit, offset int) ([]PartyBalance, error) {
	like := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	rows, err := s.db.Query(ctx, `
		SELECT sp.id, COALESCE(sp.code,''), sp.name, COALESCE(sp.phone,''),
		       COALESCE(SUM(e.credit-e.debit),0)::bigint AS balance
		FROM suppliers sp
		LEFT JOIN party_ledger_entries e
		  ON e.tenant_id=sp.tenant_id AND e.store_id=sp.store_id AND e.supplier_id=sp.id
		WHERE sp.tenant_id=$1 AND sp.store_id=$2 AND sp.deleted_at IS NULL
		  AND ($3='%%' OR lower(sp.name) LIKE $3 OR COALESCE(sp.phone,'') LIKE $3 OR lower(COALESCE(sp.code,'')) LIKE $3)
		GROUP BY sp.id,sp.code,sp.name,sp.phone
		ORDER BY balance DESC, sp.name
		LIMIT $4 OFFSET $5`, tenantID, storeID, like, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PartyBalance, 0)
	for rows.Next() {
		var x PartyBalance
		if err := rows.Scan(&x.ID, &x.Code, &x.Name, &x.Phone, &x.Balance); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) CreateSettlement(ctx context.Context, cmd SettlementCommand) (Settlement, error) {
	if cmd.PartyType != "customer" && cmd.PartyType != "supplier" {
		return Settlement{}, errors.New("party_type must be customer or supplier")
	}
	if cmd.Method != "cash" && cmd.Method != "card" {
		return Settlement{}, errors.New("method must be cash or card")
	}
	if cmd.Amount <= 0 {
		return Settlement{}, errors.New("amount must be greater than zero")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return Settlement{}, errors.New("idempotency key is required")
	}
	if cmd.PartyType == "customer" && (cmd.CustomerID == nil || *cmd.CustomerID == uuid.Nil) {
		return Settlement{}, errors.New("customer_id is required")
	}
	if cmd.PartyType == "supplier" && (cmd.SupplierID == nil || *cmd.SupplierID == uuid.Nil) {
		return Settlement{}, errors.New("supplier_id is required")
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Settlement{}, err
	}
	defer tx.Rollback(ctx)

	var existing Settlement
	var customerRaw, supplierRaw string
	err = tx.QueryRow(ctx, `
		SELECT id,party_type,COALESCE(customer_id::text,''),COALESCE(supplier_id::text,''),method,amount
		FROM settlements WHERE tenant_id=$1 AND idempotency_key=$2`, cmd.TenantID, cmd.IdempotencyKey,
	).Scan(&existing.ID, &existing.PartyType, &customerRaw, &supplierRaw, &existing.Method, &existing.Amount)
	if err == nil {
		var existingCustomer, existingSupplier *uuid.UUID
		if customerRaw != "" {
			id, parseErr := uuid.Parse(customerRaw)
			if parseErr != nil {
				return Settlement{}, parseErr
			}
			existingCustomer = &id
		}
		if supplierRaw != "" {
			id, parseErr := uuid.Parse(supplierRaw)
			if parseErr != nil {
				return Settlement{}, parseErr
			}
			existingSupplier = &id
		}
		balance, balanceErr := partyBalanceTx(ctx, tx, cmd.TenantID, cmd.StoreID, existing.PartyType, existingCustomer, existingSupplier)
		if balanceErr != nil {
			return Settlement{}, balanceErr
		}
		existing.Balance, existing.Status = balance, "posted"
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Settlement{}, err
	}

	if err := ensureParty(ctx, tx, cmd); err != nil {
		return Settlement{}, err
	}
	balance, err := partyBalanceTx(ctx, tx, cmd.TenantID, cmd.StoreID, cmd.PartyType, cmd.CustomerID, cmd.SupplierID)
	if err != nil {
		return Settlement{}, err
	}
	if balance <= 0 {
		return Settlement{}, errors.New("party has no outstanding balance")
	}
	if cmd.Amount > balance {
		return Settlement{}, fmt.Errorf("amount exceeds outstanding balance (%d)", balance)
	}

	id := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO settlements(id,tenant_id,store_id,party_type,customer_id,supplier_id,method,amount,note,idempotency_key)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10)`,
		id, cmd.TenantID, cmd.StoreID, cmd.PartyType, cmd.CustomerID, cmd.SupplierID, cmd.Method, cmd.Amount, strings.TrimSpace(cmd.Note), cmd.IdempotencyKey)
	if err != nil {
		return Settlement{}, err
	}

	accounts, err := ensureAccounts(ctx, tx, cmd.TenantID)
	if err != nil {
		return Settlement{}, err
	}
	journalID := uuid.New()
	referenceType := "customer_receipt"
	if cmd.PartyType == "supplier" {
		referenceType = "supplier_payment"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,$3,$4)`, journalID, cmd.TenantID, referenceType, id); err != nil {
		return Settlement{}, err
	}
	moneyCode := "CASH"
	if cmd.Method == "card" {
		moneyCode = "BANK_CARD"
	}
	if cmd.PartyType == "customer" {
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts[moneyCode], cmd.Amount, 0); err != nil {
			return Settlement{}, err
		}
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["AR"], 0, cmd.Amount); err != nil {
			return Settlement{}, err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,customer_id,entry_type,reference_id,debit,credit)
			VALUES($1,$2,'customer',$3,'receipt',$4,0,$5)`, cmd.TenantID, cmd.StoreID, cmd.CustomerID, id, cmd.Amount)
	} else {
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["AP"], cmd.Amount, 0); err != nil {
			return Settlement{}, err
		}
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts[moneyCode], 0, cmd.Amount); err != nil {
			return Settlement{}, err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,supplier_id,entry_type,reference_id,debit,credit)
			VALUES($1,$2,'supplier',$3,'payment',$4,$5,0)`, cmd.TenantID, cmd.StoreID, cmd.SupplierID, id, cmd.Amount)
	}
	if err != nil {
		return Settlement{}, err
	}

	payload, _ := json.Marshal(map[string]any{"settlement_id": id, "party_type": cmd.PartyType, "amount": cmd.Amount})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'settlement',$2,$3,$4)`, cmd.TenantID, id, "settlement.created", payload); err != nil {
		return Settlement{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Settlement{}, err
	}
	return Settlement{ID: id, PartyType: cmd.PartyType, Method: cmd.Method, Amount: cmd.Amount, Balance: balance - cmd.Amount, Status: "posted"}, nil
}

func ensureParty(ctx context.Context, tx pgx.Tx, cmd SettlementCommand) error {
	var ok bool
	if cmd.PartyType == "customer" {
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND deleted_at IS NULL)`, *cmd.CustomerID, cmd.TenantID, cmd.StoreID).Scan(&ok); err != nil {
			return err
		}
	} else {
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM suppliers WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND deleted_at IS NULL)`, *cmd.SupplierID, cmd.TenantID, cmd.StoreID).Scan(&ok); err != nil {
			return err
		}
	}
	if !ok {
		return errors.New("party does not belong to authenticated store")
	}
	return nil
}

func partyBalanceTx(ctx context.Context, tx pgx.Tx, tenantID, storeID uuid.UUID, partyType string, customerID, supplierID *uuid.UUID) (int64, error) {
	var balance int64
	if partyType == "customer" {
		if customerID == nil {
			return 0, errors.New("customer_id is required")
		}
		err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(debit-credit),0)::bigint FROM party_ledger_entries WHERE tenant_id=$1 AND store_id=$2 AND customer_id=$3`, tenantID, storeID, *customerID).Scan(&balance)
		return balance, err
	}
	if supplierID == nil {
		return 0, errors.New("supplier_id is required")
	}
	err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(credit-debit),0)::bigint FROM party_ledger_entries WHERE tenant_id=$1 AND store_id=$2 AND supplier_id=$3`, tenantID, storeID, *supplierID).Scan(&balance)
	return balance, err
}

func ensureAccounts(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (map[string]uuid.UUID, error) {
	defs := []struct{ code, name, typ string }{
		{"CASH", "Cash", "asset"}, {"BANK_CARD", "Card Clearing", "asset"},
		{"AR", "Accounts Receivable", "asset"}, {"AP", "Accounts Payable", "liability"},
	}
	out := make(map[string]uuid.UUID, len(defs))
	for _, d := range defs {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO accounts(tenant_id,code,name,type) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,code) DO UPDATE SET name=EXCLUDED.name RETURNING id`, tenantID, d.code, d.name, d.typ).Scan(&id); err != nil {
			return nil, err
		}
		out[d.code] = id
	}
	return out, nil
}

func entry(ctx context.Context, tx pgx.Tx, tenantID, journalID, accountID uuid.UUID, debit, credit int64) error {
	_, err := tx.Exec(ctx, `INSERT INTO journal_entries(tenant_id,journal_id,account_id,debit,credit) VALUES($1,$2,$3,$4,$5)`, tenantID, journalID, accountID, debit, credit)
	return err
}
