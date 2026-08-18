package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BankAccountCommand struct {
	TenantID       uuid.UUID `json:"-"`
	StoreID        uuid.UUID `json:"-"`
	Name           string    `json:"name"`
	BankName       string    `json:"bank_name"`
	AccountNumber  string    `json:"account_number,omitempty"`
	CardNumber     string    `json:"card_number,omitempty"`
	IBAN           string    `json:"iban,omitempty"`
	OpeningBalance int64     `json:"opening_balance"`
	IsDefault      bool      `json:"is_default"`
	IdempotencyKey string    `json:"-"`
}

type BankAccount struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	BankName       string    `json:"bank_name"`
	AccountNumber  string    `json:"account_number,omitempty"`
	CardNumber     string    `json:"card_number,omitempty"`
	IBAN           string    `json:"iban,omitempty"`
	OpeningBalance int64     `json:"opening_balance"`
	Balance        int64     `json:"balance"`
	Active         bool      `json:"active"`
	IsDefault      bool      `json:"is_default"`
	CreatedAt      string    `json:"created_at"`
}

type BankLedgerLine struct {
	ID            uuid.UUID `json:"id"`
	JournalID     uuid.UUID `json:"journal_id"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   uuid.UUID `json:"reference_id"`
	Debit         int64     `json:"debit"`
	Credit        int64     `json:"credit"`
	Change        int64     `json:"change"`
	Balance       int64     `json:"balance"`
	PostedAt      string    `json:"posted_at"`
}

type BankLedger struct {
	Account BankAccount      `json:"account"`
	Items   []BankLedgerLine `json:"items"`
}

type CheckCommand struct {
	TenantID       uuid.UUID  `json:"-"`
	StoreID        uuid.UUID  `json:"-"`
	Direction      string     `json:"-"`
	CustomerID     *uuid.UUID `json:"customer_id,omitempty"`
	SupplierID     *uuid.UUID `json:"supplier_id,omitempty"`
	CheckNumber    string     `json:"check_number"`
	SayadID        string     `json:"sayad_id,omitempty"`
	BankName       string     `json:"bank_name,omitempty"`
	BranchName     string     `json:"branch_name,omitempty"`
	Amount         int64      `json:"amount"`
	IssueDate      string     `json:"issue_date"`
	DueDate        string     `json:"due_date"`
	Note           string     `json:"note,omitempty"`
	IdempotencyKey string     `json:"-"`
	ActorUserID    uuid.UUID  `json:"-"`
}

type Check struct {
	ID                 uuid.UUID  `json:"id"`
	Direction          string     `json:"direction"`
	CustomerID         *uuid.UUID `json:"customer_id,omitempty"`
	CustomerName       string     `json:"customer_name,omitempty"`
	SupplierID         *uuid.UUID `json:"supplier_id,omitempty"`
	SupplierName       string     `json:"supplier_name,omitempty"`
	CheckNumber        string     `json:"check_number"`
	SayadID            string     `json:"sayad_id,omitempty"`
	BankName           string     `json:"bank_name,omitempty"`
	BranchName         string     `json:"branch_name,omitempty"`
	Amount             int64      `json:"amount"`
	IssueDate          string     `json:"issue_date"`
	DueDate            string     `json:"due_date"`
	Status             string     `json:"status"`
	BankAccountID      *uuid.UUID `json:"bank_account_id,omitempty"`
	BankAccountName    string     `json:"bank_account_name,omitempty"`
	EndorsedSupplierID *uuid.UUID `json:"endorsed_supplier_id,omitempty"`
	EndorsedSupplier   string     `json:"endorsed_supplier_name,omitempty"`
	Note               string     `json:"note,omitempty"`
	CreatedAt          string     `json:"created_at"`
	UpdatedAt          string     `json:"updated_at"`
}

type CheckTransitionCommand struct {
	TenantID       uuid.UUID  `json:"-"`
	StoreID        uuid.UUID  `json:"-"`
	ActorUserID    uuid.UUID  `json:"-"`
	ActorRole      string     `json:"-"`
	CheckID        uuid.UUID  `json:"-"`
	Action         string     `json:"action"`
	BankAccountID  *uuid.UUID `json:"bank_account_id,omitempty"`
	SupplierID     *uuid.UUID `json:"supplier_id,omitempty"`
	Note           string     `json:"note,omitempty"`
	IdempotencyKey string     `json:"-"`
}

type CheckSummary struct {
	ReceivableOpenAmount int64 `json:"receivable_open_amount"`
	PayableOpenAmount    int64 `json:"payable_open_amount"`
	DueTodayCount        int64 `json:"due_today_count"`
	DueNext7Count        int64 `json:"due_next_7_count"`
	OverdueCount         int64 `json:"overdue_count"`
	BouncedCount         int64 `json:"bounced_count"`
}

func normalizeDigits(value string) string {
	replacer := strings.NewReplacer(
		"۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4",
		"۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
		"٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4",
		"٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9",
	)
	return replacer.Replace(strings.TrimSpace(value))
}

func parseCheckDate(value, field string) (time.Time, error) {
	value = strings.ReplaceAll(normalizeDigits(value), "/", "-")
	if value == "" {
		return time.Time{}, fmt.Errorf("%s is required", field)
	}
	parts := strings.Split(value, "-")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD or Persian YYYY/MM/DD", field)
	}
	var y, m, d int
	if _, err := fmt.Sscanf(value, "%d-%d-%d", &y, &m, &d); err != nil {
		return time.Time{}, fmt.Errorf("%s has an invalid date", field)
	}
	if y >= 1200 && y <= 1600 {
		gy, gm, gd, ok := jalaliToGregorian(y, m, d)
		if !ok {
			return time.Time{}, fmt.Errorf("%s has an invalid Persian date", field)
		}
		return time.Date(gy, time.Month(gm), gd, 0, 0, 0, 0, time.UTC), nil
	}
	v, err := time.Parse("2006-01-02", fmt.Sprintf("%04d-%02d-%02d", y, m, d))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s has an invalid date", field)
	}
	return v, nil
}

func jalaliToGregorian(jy, jm, jd int) (int, int, int, bool) {
	if jm < 1 || jm > 12 || jd < 1 || jd > 31 || (jm > 6 && jd > 30) {
		return 0, 0, 0, false
	}
	jy += 1595
	days := -355668 + (365 * jy) + (jy/33)*8 + ((jy%33 + 3) / 4) + jd
	if jm < 7 {
		days += (jm - 1) * 31
	} else {
		days += ((jm - 7) * 30) + 186
	}
	gy := 400 * (days / 146097)
	days %= 146097
	if days > 36524 {
		days--
		gy += 100 * (days / 36524)
		days %= 36524
		if days >= 365 {
			days++
		}
	}
	gy += 4 * (days / 1461)
	days %= 1461
	if days > 365 {
		gy += (days - 1) / 365
		days = (days - 1) % 365
	}
	gd := days + 1
	monthDays := []int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	leap := gy%4 == 0 && (gy%100 != 0 || gy%400 == 0)
	if leap {
		monthDays[2] = 29
	}
	gm := 1
	for gm <= 12 && gd > monthDays[gm] {
		gd -= monthDays[gm]
		gm++
	}
	if gm > 12 {
		return 0, 0, 0, false
	}
	return gy, gm, gd, true
}

func (s *Service) CreateBankAccount(ctx context.Context, cmd BankAccountCommand) (BankAccount, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.BankName = strings.TrimSpace(cmd.BankName)
	if cmd.TenantID == uuid.Nil || cmd.StoreID == uuid.Nil || cmd.Name == "" || cmd.BankName == "" {
		return BankAccount{}, errors.New("authenticated store, name and bank_name are required")
	}
	if cmd.OpeningBalance < 0 {
		return BankAccount{}, errors.New("opening_balance cannot be negative")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return BankAccount{}, errors.New("idempotency key is required")
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return BankAccount{}, err
	}
	defer tx.Rollback(ctx)

	var existingID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM store_bank_accounts WHERE tenant_id=$1 AND idempotency_key=$2`, cmd.TenantID, cmd.IdempotencyKey).Scan(&existingID)
	if err == nil {
		out, err := bankAccountTx(ctx, tx, cmd.TenantID, cmd.StoreID, existingID)
		if err != nil {
			return BankAccount{}, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return BankAccount{}, err
	}

	id := uuid.New()
	accountID := uuid.New()
	code := "BANK_" + strings.ReplaceAll(id.String(), "-", "")
	if _, err = tx.Exec(ctx, `INSERT INTO accounts(id,tenant_id,code,name,type) VALUES($1,$2,$3,$4,'asset')`, accountID, cmd.TenantID, code, "Bank - "+cmd.Name); err != nil {
		return BankAccount{}, err
	}
	if cmd.IsDefault {
		if _, err = tx.Exec(ctx, `UPDATE store_bank_accounts SET is_default=false,updated_at=now() WHERE tenant_id=$1 AND store_id=$2 AND is_default`, cmd.TenantID, cmd.StoreID); err != nil {
			return BankAccount{}, err
		}
	}
	var created time.Time
	if err = tx.QueryRow(ctx, `
		INSERT INTO store_bank_accounts(id,tenant_id,store_id,account_id,name,bank_name,account_number,card_number,iban,opening_balance,is_default,idempotency_key)
		VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10,$11,$12)
		RETURNING created_at`, id, cmd.TenantID, cmd.StoreID, accountID, cmd.Name, cmd.BankName,
		strings.TrimSpace(cmd.AccountNumber), strings.TrimSpace(cmd.CardNumber), strings.TrimSpace(cmd.IBAN), cmd.OpeningBalance, cmd.IsDefault, cmd.IdempotencyKey).Scan(&created); err != nil {
		return BankAccount{}, err
	}
	if cmd.OpeningBalance > 0 {
		var openingID uuid.UUID
		if err = tx.QueryRow(ctx, `
			INSERT INTO accounts(tenant_id,code,name,type) VALUES($1,'OPENING_BALANCE','Opening Balance','equity')
			ON CONFLICT(tenant_id,code) DO UPDATE SET name=EXCLUDED.name RETURNING id`, cmd.TenantID).Scan(&openingID); err != nil {
			return BankAccount{}, err
		}
		journalID := uuid.New()
		if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'bank_opening',$3)`, journalID, cmd.TenantID, id); err != nil {
			return BankAccount{}, err
		}
		if err = entry(ctx, tx, cmd.TenantID, journalID, accountID, cmd.OpeningBalance, 0); err != nil {
			return BankAccount{}, err
		}
		if err = entry(ctx, tx, cmd.TenantID, journalID, openingID, 0, cmd.OpeningBalance); err != nil {
			return BankAccount{}, err
		}
	}
	payload, _ := json.Marshal(map[string]any{"bank_account_id": id, "store_id": cmd.StoreID, "opening_balance": cmd.OpeningBalance})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'bank_account',$2,'bank_account.created',$3)`, cmd.TenantID, id, payload); err != nil {
		return BankAccount{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return BankAccount{}, err
	}
	return BankAccount{ID: id, Name: cmd.Name, BankName: cmd.BankName, AccountNumber: strings.TrimSpace(cmd.AccountNumber), CardNumber: strings.TrimSpace(cmd.CardNumber), IBAN: strings.TrimSpace(cmd.IBAN), OpeningBalance: cmd.OpeningBalance, Balance: cmd.OpeningBalance, Active: true, IsDefault: cmd.IsDefault, CreatedAt: created.Format(time.RFC3339)}, nil
}

func bankAccountTx(ctx context.Context, tx pgx.Tx, tenantID, storeID, id uuid.UUID) (BankAccount, error) {
	var out BankAccount
	var created time.Time
	err := tx.QueryRow(ctx, `
		SELECT b.id,b.name,b.bank_name,COALESCE(b.account_number,''),COALESCE(b.card_number,''),COALESCE(b.iban,''),b.opening_balance,b.active,b.is_default,b.created_at,
		       COALESCE((SELECT SUM(je.debit-je.credit)::bigint FROM journal_entries je WHERE je.tenant_id=b.tenant_id AND je.account_id=b.account_id),0)::bigint
		FROM store_bank_accounts b WHERE b.id=$1 AND b.tenant_id=$2 AND b.store_id=$3`, id, tenantID, storeID).
		Scan(&out.ID, &out.Name, &out.BankName, &out.AccountNumber, &out.CardNumber, &out.IBAN, &out.OpeningBalance, &out.Active, &out.IsDefault, &created, &out.Balance)
	if err != nil {
		return BankAccount{}, err
	}
	out.CreatedAt = created.Format(time.RFC3339)
	return out, nil
}

func (s *Service) ListBankAccounts(ctx context.Context, tenantID, storeID uuid.UUID) ([]BankAccount, error) {
	rows, err := s.db.Query(ctx, `
		SELECT b.id,b.name,b.bank_name,COALESCE(b.account_number,''),COALESCE(b.card_number,''),COALESCE(b.iban,''),b.opening_balance,b.active,b.is_default,b.created_at,
		       COALESCE((SELECT SUM(je.debit-je.credit)::bigint FROM journal_entries je WHERE je.tenant_id=b.tenant_id AND je.account_id=b.account_id),0)::bigint
		FROM store_bank_accounts b
		WHERE b.tenant_id=$1 AND b.store_id=$2
		ORDER BY b.active DESC,b.is_default DESC,b.name`, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BankAccount, 0)
	for rows.Next() {
		var x BankAccount
		var created time.Time
		if err := rows.Scan(&x.ID, &x.Name, &x.BankName, &x.AccountNumber, &x.CardNumber, &x.IBAN, &x.OpeningBalance, &x.Active, &x.IsDefault, &created, &x.Balance); err != nil {
			return nil, err
		}
		x.CreatedAt = created.Format(time.RFC3339)
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) BankLedger(ctx context.Context, tenantID, storeID, id uuid.UUID) (BankLedger, error) {
	if id == uuid.Nil {
		return BankLedger{}, errors.New("bank account id is required")
	}
	var account BankAccount
	var accountID uuid.UUID
	var created time.Time
	err := s.db.QueryRow(ctx, `
		SELECT b.id,b.account_id,b.name,b.bank_name,COALESCE(b.account_number,''),COALESCE(b.card_number,''),COALESCE(b.iban,''),b.opening_balance,b.active,b.is_default,b.created_at,
		       COALESCE((SELECT SUM(je.debit-je.credit)::bigint FROM journal_entries je WHERE je.tenant_id=b.tenant_id AND je.account_id=b.account_id),0)::bigint
		FROM store_bank_accounts b WHERE b.id=$1 AND b.tenant_id=$2 AND b.store_id=$3`, id, tenantID, storeID).
		Scan(&account.ID, &accountID, &account.Name, &account.BankName, &account.AccountNumber, &account.CardNumber, &account.IBAN, &account.OpeningBalance, &account.Active, &account.IsDefault, &created, &account.Balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return BankLedger{}, errors.New("bank account does not belong to authenticated store")
	}
	if err != nil {
		return BankLedger{}, err
	}
	account.CreatedAt = created.Format(time.RFC3339)
	rows, err := s.db.Query(ctx, `
		SELECT je.id,je.journal_id,j.reference_type,j.reference_id,je.debit,je.credit,j.posted_at
		FROM journal_entries je
		JOIN journals j ON j.id=je.journal_id AND j.tenant_id=je.tenant_id
		WHERE je.tenant_id=$1 AND je.account_id=$2
		ORDER BY j.posted_at,je.id
		LIMIT 1000`, tenantID, accountID)
	if err != nil {
		return BankLedger{}, err
	}
	defer rows.Close()
	items := make([]BankLedgerLine, 0)
	balance := int64(0)
	for rows.Next() {
		var x BankLedgerLine
		var posted time.Time
		if err := rows.Scan(&x.ID, &x.JournalID, &x.ReferenceType, &x.ReferenceID, &x.Debit, &x.Credit, &posted); err != nil {
			return BankLedger{}, err
		}
		x.Change = x.Debit - x.Credit
		balance += x.Change
		x.Balance = balance
		x.PostedAt = posted.Format(time.RFC3339)
		items = append(items, x)
	}
	return BankLedger{Account: account, Items: items}, rows.Err()
}

func (s *Service) CreateCheck(ctx context.Context, cmd CheckCommand) (Check, error) {
	if cmd.Direction != "receivable" && cmd.Direction != "payable" {
		return Check{}, errors.New("direction must be receivable or payable")
	}
	cmd.CheckNumber = normalizeDigits(cmd.CheckNumber)
	cmd.SayadID = normalizeDigits(cmd.SayadID)
	if cmd.CheckNumber == "" {
		return Check{}, errors.New("check_number is required")
	}
	if cmd.SayadID != "" && len(cmd.SayadID) != 16 {
		return Check{}, errors.New("sayad_id must contain 16 digits when provided")
	}
	if cmd.Amount <= 0 {
		return Check{}, errors.New("amount must be greater than zero")
	}
	issue, err := parseCheckDate(cmd.IssueDate, "issue_date")
	if err != nil {
		return Check{}, err
	}
	due, err := parseCheckDate(cmd.DueDate, "due_date")
	if err != nil {
		return Check{}, err
	}
	if due.Before(issue) {
		return Check{}, errors.New("due_date cannot be before issue_date")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return Check{}, errors.New("idempotency key is required")
	}
	if cmd.Direction == "receivable" && (cmd.CustomerID == nil || *cmd.CustomerID == uuid.Nil) {
		return Check{}, errors.New("customer_id is required for receivable checks")
	}
	if cmd.Direction == "payable" && (cmd.SupplierID == nil || *cmd.SupplierID == uuid.Nil) {
		return Check{}, errors.New("supplier_id is required for payable checks")
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Check{}, err
	}
	defer tx.Rollback(ctx)
	var existingID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM checks WHERE tenant_id=$1 AND idempotency_key=$2`, cmd.TenantID, cmd.IdempotencyKey).Scan(&existingID)
	if err == nil {
		out, err := checkTx(ctx, tx, cmd.TenantID, cmd.StoreID, existingID)
		if err != nil {
			return Check{}, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Check{}, err
	}

	settlementCmd := SettlementCommand{TenantID: cmd.TenantID, StoreID: cmd.StoreID, PartyType: "customer", CustomerID: cmd.CustomerID}
	if cmd.Direction == "payable" {
		settlementCmd.PartyType = "supplier"
		settlementCmd.SupplierID = cmd.SupplierID
	}
	if err := ensureParty(ctx, tx, settlementCmd); err != nil {
		return Check{}, err
	}
	balance, err := partyBalanceTx(ctx, tx, cmd.TenantID, cmd.StoreID, settlementCmd.PartyType, cmd.CustomerID, cmd.SupplierID)
	if err != nil {
		return Check{}, err
	}
	if balance <= 0 {
		return Check{}, errors.New("party has no outstanding balance")
	}
	if cmd.Amount > balance {
		return Check{}, fmt.Errorf("amount exceeds outstanding balance (%d)", balance)
	}

	accounts, err := ensureCheckAccounts(ctx, tx, cmd.TenantID)
	if err != nil {
		return Check{}, err
	}
	id := uuid.New()
	status := "held"
	if cmd.Direction == "payable" {
		status = "issued"
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO checks(id,tenant_id,store_id,direction,customer_id,supplier_id,check_number,sayad_id,bank_name,branch_name,amount,issue_date,due_date,status,note,idempotency_key)
		VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),$11,$12,$13,$14,NULLIF($15,''),$16)`,
		id, cmd.TenantID, cmd.StoreID, cmd.Direction, cmd.CustomerID, cmd.SupplierID, cmd.CheckNumber, cmd.SayadID, strings.TrimSpace(cmd.BankName), strings.TrimSpace(cmd.BranchName), cmd.Amount, issue, due, status, strings.TrimSpace(cmd.Note), cmd.IdempotencyKey); err != nil {
		return Check{}, err
	}
	journalID := uuid.New()
	refType := "check_receivable"
	if cmd.Direction == "payable" {
		refType = "check_payable"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,$3,$4)`, journalID, cmd.TenantID, refType, id); err != nil {
		return Check{}, err
	}
	if cmd.Direction == "receivable" {
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["CHEQUES_RECEIVABLE"], cmd.Amount, 0); err != nil {
			return Check{}, err
		}
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["AR"], 0, cmd.Amount); err != nil {
			return Check{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,customer_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'customer',$3,'check_receipt',$4,0,$5)`, cmd.TenantID, cmd.StoreID, cmd.CustomerID, id, cmd.Amount); err != nil {
			return Check{}, err
		}
	} else {
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["AP"], cmd.Amount, 0); err != nil {
			return Check{}, err
		}
		if err = entry(ctx, tx, cmd.TenantID, journalID, accounts["CHEQUES_PAYABLE"], 0, cmd.Amount); err != nil {
			return Check{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,supplier_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'supplier',$3,'check_payment',$4,$5,0)`, cmd.TenantID, cmd.StoreID, cmd.SupplierID, id, cmd.Amount); err != nil {
			return Check{}, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO check_events(tenant_id,store_id,check_id,action,to_status,actor_user_id,note) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''))`, cmd.TenantID, cmd.StoreID, id, "create", status, cmd.ActorUserID, strings.TrimSpace(cmd.Note)); err != nil {
		return Check{}, err
	}
	payload, _ := json.Marshal(map[string]any{"check_id": id, "direction": cmd.Direction, "amount": cmd.Amount, "due_date": due.Format("2006-01-02")})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'check',$2,'check.created',$3)`, cmd.TenantID, id, payload); err != nil {
		return Check{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Check{}, err
	}
	return s.GetCheck(ctx, cmd.TenantID, cmd.StoreID, id)
}

func ensureCheckAccounts(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (map[string]uuid.UUID, error) {
	out, err := ensureAccounts(ctx, tx, tenantID)
	if err != nil {
		return nil, err
	}
	defs := []struct{ code, name, typ string }{
		{"CHEQUES_RECEIVABLE", "Cheques Receivable", "asset"},
		{"CHEQUES_PAYABLE", "Cheques Payable", "liability"},
	}
	for _, d := range defs {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO accounts(tenant_id,code,name,type) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,code) DO UPDATE SET name=EXCLUDED.name RETURNING id`, tenantID, d.code, d.name, d.typ).Scan(&id); err != nil {
			return nil, err
		}
		out[d.code] = id
	}
	return out, nil
}

func checkTx(ctx context.Context, tx pgx.Tx, tenantID, storeID, id uuid.UUID) (Check, error) {
	var x Check
	var issue, due, created, updated time.Time
	err := tx.QueryRow(ctx, `
		SELECT c.id,c.direction,c.customer_id,COALESCE(cu.name,''),c.supplier_id,COALESCE(sp.name,''),c.check_number,COALESCE(c.sayad_id,''),COALESCE(c.bank_name,''),COALESCE(c.branch_name,''),c.amount,c.issue_date,c.due_date,c.status,c.bank_account_id,COALESCE(ba.name,''),c.endorsed_supplier_id,COALESCE(es.name,''),COALESCE(c.note,''),c.created_at,c.updated_at
		FROM checks c
		LEFT JOIN customers cu ON cu.id=c.customer_id AND cu.tenant_id=c.tenant_id
		LEFT JOIN suppliers sp ON sp.id=c.supplier_id AND sp.tenant_id=c.tenant_id
		LEFT JOIN store_bank_accounts ba ON ba.id=c.bank_account_id AND ba.tenant_id=c.tenant_id
		LEFT JOIN suppliers es ON es.id=c.endorsed_supplier_id AND es.tenant_id=c.tenant_id
		WHERE c.id=$1 AND c.tenant_id=$2 AND c.store_id=$3`, id, tenantID, storeID).
		Scan(&x.ID, &x.Direction, &x.CustomerID, &x.CustomerName, &x.SupplierID, &x.SupplierName, &x.CheckNumber, &x.SayadID, &x.BankName, &x.BranchName, &x.Amount, &issue, &due, &x.Status, &x.BankAccountID, &x.BankAccountName, &x.EndorsedSupplierID, &x.EndorsedSupplier, &x.Note, &created, &updated)
	if err != nil {
		return Check{}, err
	}
	x.IssueDate = issue.Format("2006-01-02")
	x.DueDate = due.Format("2006-01-02")
	x.CreatedAt = created.Format(time.RFC3339)
	x.UpdatedAt = updated.Format(time.RFC3339)
	return x, nil
}

func (s *Service) GetCheck(ctx context.Context, tenantID, storeID, id uuid.UUID) (Check, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return Check{}, err
	}
	defer tx.Rollback(ctx)
	out, err := checkTx(ctx, tx, tenantID, storeID, id)
	if err != nil {
		return Check{}, err
	}
	return out, tx.Commit(ctx)
}

func (s *Service) ListChecks(ctx context.Context, tenantID, storeID uuid.UUID, direction, status, q string, limit, offset int) ([]Check, int, error) {
	direction = strings.TrimSpace(direction)
	status = strings.TrimSpace(status)
	q = "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	if direction != "" && direction != "receivable" && direction != "payable" {
		return nil, 0, errors.New("direction must be receivable or payable")
	}
	var total int
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM checks c
		LEFT JOIN customers cu ON cu.id=c.customer_id AND cu.tenant_id=c.tenant_id
		LEFT JOIN suppliers sp ON sp.id=c.supplier_id AND sp.tenant_id=c.tenant_id
		WHERE c.tenant_id=$1 AND c.store_id=$2
		  AND ($3='' OR c.direction=$3) AND ($4='' OR c.status=$4)
		  AND ($5='%%' OR lower(c.check_number) LIKE $5 OR lower(COALESCE(c.sayad_id,'')) LIKE $5 OR lower(COALESCE(cu.name,'')) LIKE $5 OR lower(COALESCE(sp.name,'')) LIKE $5)`, tenantID, storeID, direction, status, q).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT c.id,c.direction,c.customer_id,COALESCE(cu.name,''),c.supplier_id,COALESCE(sp.name,''),c.check_number,COALESCE(c.sayad_id,''),COALESCE(c.bank_name,''),COALESCE(c.branch_name,''),c.amount,c.issue_date,c.due_date,c.status,c.bank_account_id,COALESCE(ba.name,''),c.endorsed_supplier_id,COALESCE(es.name,''),COALESCE(c.note,''),c.created_at,c.updated_at
		FROM checks c
		LEFT JOIN customers cu ON cu.id=c.customer_id AND cu.tenant_id=c.tenant_id
		LEFT JOIN suppliers sp ON sp.id=c.supplier_id AND sp.tenant_id=c.tenant_id
		LEFT JOIN store_bank_accounts ba ON ba.id=c.bank_account_id AND ba.tenant_id=c.tenant_id
		LEFT JOIN suppliers es ON es.id=c.endorsed_supplier_id AND es.tenant_id=c.tenant_id
		WHERE c.tenant_id=$1 AND c.store_id=$2
		  AND ($3='' OR c.direction=$3) AND ($4='' OR c.status=$4)
		  AND ($5='%%' OR lower(c.check_number) LIKE $5 OR lower(COALESCE(c.sayad_id,'')) LIKE $5 OR lower(COALESCE(cu.name,'')) LIKE $5 OR lower(COALESCE(sp.name,'')) LIKE $5)
		ORDER BY CASE WHEN c.status IN ('held','deposited','issued') THEN 0 ELSE 1 END,c.due_date,c.created_at DESC
		LIMIT $6 OFFSET $7`, tenantID, storeID, direction, status, q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Check, 0)
	for rows.Next() {
		var x Check
		var issue, due, created, updated time.Time
		if err := rows.Scan(&x.ID, &x.Direction, &x.CustomerID, &x.CustomerName, &x.SupplierID, &x.SupplierName, &x.CheckNumber, &x.SayadID, &x.BankName, &x.BranchName, &x.Amount, &issue, &due, &x.Status, &x.BankAccountID, &x.BankAccountName, &x.EndorsedSupplierID, &x.EndorsedSupplier, &x.Note, &created, &updated); err != nil {
			return nil, 0, err
		}
		x.IssueDate = issue.Format("2006-01-02")
		x.DueDate = due.Format("2006-01-02")
		x.CreatedAt = created.Format(time.RFC3339)
		x.UpdatedAt = updated.Format(time.RFC3339)
		out = append(out, x)
	}
	return out, total, rows.Err()
}

func (s *Service) CheckSummary(ctx context.Context, tenantID, storeID uuid.UUID, now time.Time) (CheckSummary, error) {
	var out CheckSummary
	today := now.Format("2006-01-02")
	next7 := now.AddDate(0, 0, 7).Format("2006-01-02")
	err := s.db.QueryRow(ctx, `
		SELECT
		  COALESCE(SUM(amount) FILTER (WHERE direction='receivable' AND status IN ('held','deposited','returned')),0)::bigint,
		  COALESCE(SUM(amount) FILTER (WHERE direction='payable' AND status='issued'),0)::bigint,
		  COUNT(*) FILTER (WHERE due_date=$3::date AND status IN ('held','deposited','issued','returned')),
		  COUNT(*) FILTER (WHERE due_date>$3::date AND due_date<=$4::date AND status IN ('held','deposited','issued','returned')),
		  COUNT(*) FILTER (WHERE due_date<$3::date AND status IN ('held','deposited','issued','returned')),
		  COUNT(*) FILTER (WHERE direction='receivable' AND status='bounced')
		FROM checks WHERE tenant_id=$1 AND store_id=$2`, tenantID, storeID, today, next7).
		Scan(&out.ReceivableOpenAmount, &out.PayableOpenAmount, &out.DueTodayCount, &out.DueNext7Count, &out.OverdueCount, &out.BouncedCount)
	return out, err
}

func (s *Service) TransitionCheck(ctx context.Context, cmd CheckTransitionCommand) (Check, error) {
	if cmd.CheckID == uuid.Nil || strings.TrimSpace(cmd.Action) == "" {
		return Check{}, errors.New("check id and action are required")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return Check{}, errors.New("idempotency key is required")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Check{}, err
	}
	defer tx.Rollback(ctx)

	var duplicateID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT check_id FROM check_events WHERE tenant_id=$1 AND idempotency_key=$2 LIMIT 1`, cmd.TenantID, cmd.IdempotencyKey).Scan(&duplicateID)
	if err == nil {
		out, err := checkTx(ctx, tx, cmd.TenantID, cmd.StoreID, duplicateID)
		if err != nil {
			return Check{}, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Check{}, err
	}

	var direction, status string
	var amount int64
	var customerID, supplierID *uuid.UUID
	var existingBankID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT direction,status,amount,customer_id,supplier_id,bank_account_id FROM checks WHERE id=$1 AND tenant_id=$2 AND store_id=$3 FOR UPDATE`, cmd.CheckID, cmd.TenantID, cmd.StoreID).
		Scan(&direction, &status, &amount, &customerID, &supplierID, &existingBankID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Check{}, errors.New("check does not belong to authenticated store")
	}
	if err != nil {
		return Check{}, err
	}
	if cmd.ActorRole == "cashier" && direction == "payable" {
		return Check{}, errors.New("cashier cannot change payable checks")
	}

	accounts, err := ensureCheckAccounts(ctx, tx, cmd.TenantID)
	if err != nil {
		return Check{}, err
	}
	action := strings.TrimSpace(cmd.Action)
	newStatus := ""
	var bankID *uuid.UUID
	var bankGL uuid.UUID
	if cmd.BankAccountID != nil {
		bankID = cmd.BankAccountID
	} else {
		bankID = existingBankID
	}
	loadBank := func() error {
		if bankID == nil || *bankID == uuid.Nil {
			return errors.New("bank_account_id is required")
		}
		return tx.QueryRow(ctx, `SELECT account_id FROM store_bank_accounts WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND active FOR SHARE`, *bankID, cmd.TenantID, cmd.StoreID).Scan(&bankGL)
	}
	journal := func(refType string, debitAccount uuid.UUID, creditAccount uuid.UUID) error {
		jid := uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,$3,$4)`, jid, cmd.TenantID, refType, cmd.CheckID); err != nil {
			return err
		}
		if err := entry(ctx, tx, cmd.TenantID, jid, debitAccount, amount, 0); err != nil {
			return err
		}
		return entry(ctx, tx, cmd.TenantID, jid, creditAccount, 0, amount)
	}

	if direction == "receivable" {
		switch action {
		case "deposit":
			if status != "held" && status != "returned" {
				return Check{}, errors.New("only held/returned receivable checks can be deposited")
			}
			if err := loadBank(); err != nil {
				return Check{}, err
			}
			newStatus = "deposited"
		case "clear":
			if status != "held" && status != "deposited" && status != "returned" {
				return Check{}, errors.New("receivable check cannot be cleared from current status")
			}
			if err := loadBank(); err != nil {
				return Check{}, err
			}
			if err := journal("check_clear", bankGL, accounts["CHEQUES_RECEIVABLE"]); err != nil {
				return Check{}, err
			}
			newStatus = "cleared"
		case "bounce":
			if status != "held" && status != "deposited" && status != "returned" {
				return Check{}, errors.New("receivable check cannot bounce from current status")
			}
			if customerID == nil {
				return Check{}, errors.New("customer is missing")
			}
			if err := journal("check_bounce", accounts["AR"], accounts["CHEQUES_RECEIVABLE"]); err != nil {
				return Check{}, err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,customer_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'customer',$3,'check_bounce',$4,$5,0)`, cmd.TenantID, cmd.StoreID, customerID, cmd.CheckID, amount); err != nil {
				return Check{}, err
			}
			newStatus = "bounced"
		case "endorse":
			if status != "held" && status != "returned" {
				return Check{}, errors.New("only held/returned receivable checks can be endorsed")
			}
			if cmd.SupplierID == nil || *cmd.SupplierID == uuid.Nil {
				return Check{}, errors.New("supplier_id is required for endorsement")
			}
			partyCmd := SettlementCommand{TenantID: cmd.TenantID, StoreID: cmd.StoreID, PartyType: "supplier", SupplierID: cmd.SupplierID}
			if err := ensureParty(ctx, tx, partyCmd); err != nil {
				return Check{}, err
			}
			balance, err := partyBalanceTx(ctx, tx, cmd.TenantID, cmd.StoreID, "supplier", nil, cmd.SupplierID)
			if err != nil {
				return Check{}, err
			}
			if balance < amount {
				return Check{}, fmt.Errorf("check amount exceeds supplier outstanding balance (%d)", balance)
			}
			if err := journal("check_endorse", accounts["AP"], accounts["CHEQUES_RECEIVABLE"]); err != nil {
				return Check{}, err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,supplier_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'supplier',$3,'check_payment',$4,$5,0)`, cmd.TenantID, cmd.StoreID, cmd.SupplierID, cmd.CheckID, amount); err != nil {
				return Check{}, err
			}
			newStatus = "endorsed"
		case "return_endorsement":
			if status != "endorsed" {
				return Check{}, errors.New("only endorsed checks can be returned by supplier")
			}
			var endorsedSupplier uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT endorsed_supplier_id FROM checks WHERE id=$1`, cmd.CheckID).Scan(&endorsedSupplier); err != nil {
				return Check{}, err
			}
			if err := journal("check_endorse_return", accounts["CHEQUES_RECEIVABLE"], accounts["AP"]); err != nil {
				return Check{}, err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,supplier_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'supplier',$3,'check_return',$4,0,$5)`, cmd.TenantID, cmd.StoreID, endorsedSupplier, cmd.CheckID, amount); err != nil {
				return Check{}, err
			}
			newStatus = "returned"
		case "cancel":
			if status != "held" {
				return Check{}, errors.New("only held receivable checks can be cancelled")
			}
			if customerID == nil {
				return Check{}, errors.New("customer is missing")
			}
			if err := journal("check_cancel", accounts["AR"], accounts["CHEQUES_RECEIVABLE"]); err != nil {
				return Check{}, err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,customer_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'customer',$3,'check_return',$4,$5,0)`, cmd.TenantID, cmd.StoreID, customerID, cmd.CheckID, amount); err != nil {
				return Check{}, err
			}
			newStatus = "cancelled"
		default:
			return Check{}, errors.New("unsupported receivable check action")
		}
	} else {
		switch action {
		case "clear":
			if status != "issued" {
				return Check{}, errors.New("only issued payable checks can be cleared")
			}
			if err := loadBank(); err != nil {
				return Check{}, err
			}
			if err := journal("check_payable_clear", accounts["CHEQUES_PAYABLE"], bankGL); err != nil {
				return Check{}, err
			}
			newStatus = "cleared"
		case "return", "cancel":
			if status != "issued" {
				return Check{}, errors.New("only issued payable checks can be returned/cancelled")
			}
			if supplierID == nil {
				return Check{}, errors.New("supplier is missing")
			}
			if err := journal("check_payable_return", accounts["CHEQUES_PAYABLE"], accounts["AP"]); err != nil {
				return Check{}, err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO party_ledger_entries(tenant_id,store_id,party_type,supplier_id,entry_type,reference_id,debit,credit) VALUES($1,$2,'supplier',$3,'check_return',$4,0,$5)`, cmd.TenantID, cmd.StoreID, supplierID, cmd.CheckID, amount); err != nil {
				return Check{}, err
			}
			if action == "cancel" {
				newStatus = "cancelled"
			} else {
				newStatus = "returned"
			}
		default:
			return Check{}, errors.New("unsupported payable check action")
		}
	}

	if newStatus == "" {
		return Check{}, errors.New("transition produced no status")
	}
	if action == "endorse" {
		_, err = tx.Exec(ctx, `UPDATE checks SET status=$2,endorsed_supplier_id=$3,updated_at=now() WHERE id=$1`, cmd.CheckID, newStatus, cmd.SupplierID)
	} else if action == "return_endorsement" {
		_, err = tx.Exec(ctx, `UPDATE checks SET status=$2,endorsed_supplier_id=NULL,updated_at=now() WHERE id=$1`, cmd.CheckID, newStatus)
	} else if bankID != nil && (action == "deposit" || action == "clear") {
		_, err = tx.Exec(ctx, `UPDATE checks SET status=$2,bank_account_id=$3,updated_at=now() WHERE id=$1`, cmd.CheckID, newStatus, bankID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE checks SET status=$2,updated_at=now() WHERE id=$1`, cmd.CheckID, newStatus)
	}
	if err != nil {
		return Check{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO check_events(tenant_id,store_id,check_id,action,idempotency_key,from_status,to_status,bank_account_id,supplier_id,actor_user_id,note) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''))`, cmd.TenantID, cmd.StoreID, cmd.CheckID, action, cmd.IdempotencyKey, status, newStatus, bankID, cmd.SupplierID, cmd.ActorUserID, strings.TrimSpace(cmd.Note)); err != nil {
		return Check{}, err
	}
	payload, _ := json.Marshal(map[string]any{"idempotency_key": cmd.IdempotencyKey, "check_id": cmd.CheckID, "action": action, "from_status": status, "to_status": newStatus})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'check_transition',$2,'check.transitioned',$3)`, cmd.TenantID, cmd.CheckID, payload); err != nil {
		return Check{}, err
	}
	out, err := checkTx(ctx, tx, cmd.TenantID, cmd.StoreID, cmd.CheckID)
	if err != nil {
		return Check{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Check{}, err
	}
	return out, nil
}
