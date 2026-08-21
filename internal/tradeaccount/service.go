package tradeaccount

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type Account struct {
	ID             uuid.UUID `json:"id"`
	MechanicUserID uuid.UUID `json:"mechanic_user_id"`
	MechanicName   string    `json:"mechanic_name"`
	MechanicEmail  string    `json:"mechanic_email,omitempty"`
	TenantID       uuid.UUID `json:"tenant_id"`
	StoreID        uuid.UUID `json:"store_id"`
	StoreName      string    `json:"store_name"`
	Balance        int64     `json:"balance"` // positive = mechanic owes store
	PendingAmount  int64     `json:"pending_amount"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Request struct {
	ID                uuid.UUID  `json:"id"`
	AccountID         uuid.UUID  `json:"account_id"`
	RequestType       string     `json:"request_type"`
	Amount            int64      `json:"amount"`
	Method            string     `json:"method,omitempty"`
	ReservationID     *uuid.UUID `json:"reservation_id,omitempty"`
	ReferenceType     string     `json:"reference_type,omitempty"`
	ReferenceID       *uuid.UUID `json:"reference_id,omitempty"`
	Note              string     `json:"note,omitempty"`
	Status            string     `json:"status"`
	InitiatedByRole   string     `json:"initiated_by_role"`
	InitiatedByUserID uuid.UUID  `json:"initiated_by_user_id"`
	ConfirmedByRole   string     `json:"confirmed_by_role,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
}

type LedgerEntry struct {
	ID            uuid.UUID  `json:"id"`
	RequestID     *uuid.UUID `json:"request_id,omitempty"`
	EntryType     string     `json:"entry_type"`
	ReferenceType string     `json:"reference_type,omitempty"`
	ReferenceID   *uuid.UUID `json:"reference_id,omitempty"`
	Debit         int64      `json:"debit"`
	Credit        int64      `json:"credit"`
	Note          string     `json:"note,omitempty"`
	Balance       int64      `json:"balance"`
	CreatedAt     time.Time  `json:"created_at"`
}

type CreateRequest struct {
	RequestType   string     `json:"request_type"`
	Amount        int64      `json:"amount"`
	Method        string     `json:"method,omitempty"`
	ReferenceType string     `json:"reference_type,omitempty"`
	ReferenceID   *uuid.UUID `json:"reference_id,omitempty"`
	Note          string     `json:"note,omitempty"`
}

func (s *Service) ListMechanicAccounts(ctx context.Context, mechanicID uuid.UUID) ([]Account, error) {
	return s.listAccounts(ctx, `a.mechanic_user_id=$1`, mechanicID)
}

func (s *Service) ListStoreAccounts(ctx context.Context, tenantID, storeID uuid.UUID) ([]Account, error) {
	return s.listAccounts(ctx, `a.tenant_id=$1 AND a.store_id=$2`, tenantID, storeID)
}

func (s *Service) listAccounts(ctx context.Context, where string, args ...any) ([]Account, error) {
	rows, err := s.db.Query(ctx, accountSelect+` WHERE `+where+` ORDER BY abs(COALESCE(l.balance,0)) DESC,a.updated_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.MechanicUserID, &a.MechanicName, &a.MechanicEmail, &a.TenantID, &a.StoreID, &a.StoreName, &a.Balance, &a.PendingAmount, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

const accountSelect = `SELECT a.id,a.mechanic_user_id,a.mechanic_name,COALESCE(a.mechanic_email,''),a.tenant_id,a.store_id,s.name,
	COALESCE(l.balance,0)::bigint,
	COALESCE(p.pending,0)::bigint,a.updated_at
	FROM mechanic_store_accounts a JOIN stores s ON s.id=a.store_id AND s.tenant_id=a.tenant_id
	LEFT JOIN LATERAL (SELECT COALESCE(SUM(debit-credit),0) balance FROM mechanic_store_ledger_entries WHERE account_id=a.id) l ON true
	LEFT JOIN LATERAL (SELECT COALESCE(SUM(amount),0) pending FROM mechanic_store_trade_requests WHERE account_id=a.id AND status='pending') p ON true`

func (s *Service) AccountForMechanic(ctx context.Context, mechanicID, accountID uuid.UUID) (Account, error) {
	return s.account(ctx, `a.id=$1 AND a.mechanic_user_id=$2`, accountID, mechanicID)
}

func (s *Service) AccountForStore(ctx context.Context, tenantID, storeID, accountID uuid.UUID) (Account, error) {
	return s.account(ctx, `a.id=$1 AND a.tenant_id=$2 AND a.store_id=$3`, accountID, tenantID, storeID)
}

func (s *Service) account(ctx context.Context, where string, args ...any) (Account, error) {
	var a Account
	err := s.db.QueryRow(ctx, accountSelect+` WHERE `+where, args...).Scan(&a.ID, &a.MechanicUserID, &a.MechanicName, &a.MechanicEmail, &a.TenantID, &a.StoreID, &a.StoreName, &a.Balance, &a.PendingAmount, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, errors.New("trade account not found")
	}
	return a, err
}

func (s *Service) LedgerForMechanic(ctx context.Context, mechanicID, accountID uuid.UUID) ([]LedgerEntry, error) {
	if _, err := s.AccountForMechanic(ctx, mechanicID, accountID); err != nil {
		return nil, err
	}
	return s.ledger(ctx, accountID)
}

func (s *Service) LedgerForStore(ctx context.Context, tenantID, storeID, accountID uuid.UUID) ([]LedgerEntry, error) {
	if _, err := s.AccountForStore(ctx, tenantID, storeID, accountID); err != nil {
		return nil, err
	}
	return s.ledger(ctx, accountID)
}

func (s *Service) ledger(ctx context.Context, accountID uuid.UUID) ([]LedgerEntry, error) {
	rows, err := s.db.Query(ctx, `SELECT id,request_id,entry_type,COALESCE(reference_type,''),reference_id,debit,credit,COALESCE(note,''),created_at
		FROM mechanic_store_ledger_entries WHERE account_id=$1 ORDER BY created_at,id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LedgerEntry{}
	var balance int64
	for rows.Next() {
		var x LedgerEntry
		if err := rows.Scan(&x.ID, &x.RequestID, &x.EntryType, &x.ReferenceType, &x.ReferenceID, &x.Debit, &x.Credit, &x.Note, &x.CreatedAt); err != nil {
			return nil, err
		}
		balance += x.Debit - x.Credit
		x.Balance = balance
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// UI wants newest first but each row keeps the chronological running balance.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (s *Service) ListMechanicRequests(ctx context.Context, mechanicID uuid.UUID, status string) ([]Request, error) {
	return s.listRequests(ctx, `a.mechanic_user_id=$1`, status, mechanicID)
}

func (s *Service) ListStoreRequests(ctx context.Context, tenantID, storeID uuid.UUID, status string) ([]Request, error) {
	return s.listRequests(ctx, `a.tenant_id=$1 AND a.store_id=$2`, status, tenantID, storeID)
}

func (s *Service) listRequests(ctx context.Context, where, status string, args ...any) ([]Request, error) {
	if status == "" {
		status = "pending"
	}
	if status != "all" && status != "pending" && status != "confirmed" && status != "rejected" {
		return nil, errors.New("invalid request status")
	}
	query := requestSelect + ` WHERE ` + where
	if status != "all" {
		args = append(args, status)
		query += ` AND r.status=$` + itoa(len(args))
	}
	query += ` ORDER BY CASE r.status WHEN 'pending' THEN 0 ELSE 1 END,r.created_at DESC LIMIT 200`
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		x, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

const requestSelect = `SELECT r.id,r.account_id,r.request_type,r.amount,COALESCE(r.method,''),r.reservation_id,COALESCE(r.reference_type,''),r.reference_id,COALESCE(r.note,''),r.status,r.initiated_by_role,r.initiated_by_user_id,COALESCE(r.confirmed_by_role,''),r.created_at,r.resolved_at
	FROM mechanic_store_trade_requests r JOIN mechanic_store_accounts a ON a.id=r.account_id`

func scanRequest(row scanner) (Request, error) {
	var x Request
	err := row.Scan(&x.ID, &x.AccountID, &x.RequestType, &x.Amount, &x.Method, &x.ReservationID, &x.ReferenceType, &x.ReferenceID, &x.Note, &x.Status, &x.InitiatedByRole, &x.InitiatedByUserID, &x.ConfirmedByRole, &x.CreatedAt, &x.ResolvedAt)
	return x, err
}

type scanner interface{ Scan(...any) error }

func (s *Service) CreateReservationCharge(ctx context.Context, mechanicID uuid.UUID, mechanicName, mechanicEmail string, reservationID uuid.UUID, note string) (Request, error) {
	if mechanicID == uuid.Nil || reservationID == uuid.Nil {
		return Request{}, errors.New("mechanic and reservation are required")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Request{}, err
	}
	defer tx.Rollback(ctx)
	var buyer, tenantID, storeID uuid.UUID
	var status string
	var total, due int64
	var saleID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT r.buyer_user_id,r.tenant_id,r.store_id,r.status,r.total_amount,r.sale_id,COALESCE(s.due_amount,r.total_amount)
		FROM network_reservations r LEFT JOIN sales s ON s.id=r.sale_id WHERE r.id=$1 FOR UPDATE OF r`, reservationID).
		Scan(&buyer, &tenantID, &storeID, &status, &total, &saleID, &due)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, errors.New("network reservation not found")
	}
	if err != nil {
		return Request{}, err
	}
	if buyer != mechanicID {
		return Request{}, errors.New("network reservation does not belong to authenticated mechanic")
	}
	if status != "fulfilled" {
		return Request{}, errors.New("only fulfilled network purchases can create debt")
	}
	if due <= 0 {
		return Request{}, errors.New("this network purchase has no unpaid balance")
	}
	accountID, err := ensureAccount(ctx, tx, mechanicID, mechanicName, mechanicEmail, tenantID, storeID)
	if err != nil {
		return Request{}, err
	}
	refType := "network_reservation"
	refID := reservationID
	if saleID != nil {
		refType, refID = "sale", *saleID
	}
	id := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO mechanic_store_trade_requests(id,account_id,request_type,amount,reservation_id,reference_type,reference_id,note,initiated_by_role,initiated_by_user_id)
		VALUES($1,$2,'charge',$3,$4,$5,$6,NULLIF($7,''),'mechanic',$8)`, id, accountID, due, reservationID, refType, refID, strings.TrimSpace(note), mechanicID)
	if err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	return s.requestByID(ctx, id)
}

func (s *Service) CreateMechanicPayment(ctx context.Context, mechanicID, accountID uuid.UUID, in CreateRequest) (Request, error) {
	if in.Amount <= 0 {
		return Request{}, errors.New("amount must be positive")
	}
	if !validMethod(in.Method) {
		return Request{}, errors.New("invalid settlement method")
	}
	if _, err := s.AccountForMechanic(ctx, mechanicID, accountID); err != nil {
		return Request{}, err
	}
	id := uuid.New()
	_, err := s.db.Exec(ctx, `INSERT INTO mechanic_store_trade_requests(id,account_id,request_type,amount,method,reference_type,reference_id,note,initiated_by_role,initiated_by_user_id)
		VALUES($1,$2,'payment',$3,$4,NULLIF($5,''),$6,NULLIF($7,''),'mechanic',$8)`, id, accountID, in.Amount, in.Method, strings.TrimSpace(in.ReferenceType), in.ReferenceID, strings.TrimSpace(in.Note), mechanicID)
	if err != nil {
		return Request{}, err
	}
	return s.requestByID(ctx, id)
}

func (s *Service) CreateStoreRequest(ctx context.Context, tenantID, storeID, actorID, accountID uuid.UUID, in CreateRequest) (Request, error) {
	if in.Amount <= 0 {
		return Request{}, errors.New("amount must be positive")
	}
	if !validType(in.RequestType) || in.RequestType == "payment" && !validMethod(in.Method) {
		return Request{}, errors.New("invalid request type or method")
	}
	if _, err := s.AccountForStore(ctx, tenantID, storeID, accountID); err != nil {
		return Request{}, err
	}
	id := uuid.New()
	_, err := s.db.Exec(ctx, `INSERT INTO mechanic_store_trade_requests(id,account_id,request_type,amount,method,reference_type,reference_id,note,initiated_by_role,initiated_by_user_id)
		VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,NULLIF($8,''),'store',$9)`, id, accountID, in.RequestType, in.Amount, strings.TrimSpace(in.Method), strings.TrimSpace(in.ReferenceType), in.ReferenceID, strings.TrimSpace(in.Note), actorID)
	if err != nil {
		return Request{}, err
	}
	return s.requestByID(ctx, id)
}

func (s *Service) ResolveByMechanic(ctx context.Context, mechanicID, requestID uuid.UUID, confirm bool) (Request, error) {
	return s.resolve(ctx, "mechanic", mechanicID, uuid.Nil, uuid.Nil, requestID, confirm)
}

func (s *Service) ResolveByStore(ctx context.Context, tenantID, storeID, actorID, requestID uuid.UUID, confirm bool) (Request, error) {
	return s.resolve(ctx, "store", actorID, tenantID, storeID, requestID, confirm)
}

func (s *Service) resolve(ctx context.Context, resolverRole string, actorID, tenantID, storeID, requestID uuid.UUID, confirm bool) (Request, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Request{}, err
	}
	defer tx.Rollback(ctx)
	var x Request
	var mechanicID, rowTenant, rowStore uuid.UUID
	err = tx.QueryRow(ctx, requestSelect+` WHERE r.id=$1 FOR UPDATE OF r`, requestID).
		Scan(&x.ID, &x.AccountID, &x.RequestType, &x.Amount, &x.Method, &x.ReservationID, &x.ReferenceType, &x.ReferenceID, &x.Note, &x.Status, &x.InitiatedByRole, &x.InitiatedByUserID, &x.ConfirmedByRole, &x.CreatedAt, &x.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, errors.New("trade request not found")
	}
	if err != nil {
		return Request{}, err
	}
	if err = tx.QueryRow(ctx, `SELECT mechanic_user_id,tenant_id,store_id FROM mechanic_store_accounts WHERE id=$1`, x.AccountID).Scan(&mechanicID, &rowTenant, &rowStore); err != nil {
		return Request{}, err
	}
	if x.Status != "pending" {
		return Request{}, errors.New("trade request is already resolved")
	}
	if resolverRole == x.InitiatedByRole {
		return Request{}, errors.New("the opposite party must confirm this request")
	}
	if resolverRole == "mechanic" && mechanicID != actorID {
		return Request{}, errors.New("trade request does not belong to authenticated mechanic")
	}
	if resolverRole == "store" && (rowTenant != tenantID || rowStore != storeID) {
		return Request{}, errors.New("trade request does not belong to authenticated store")
	}
	next := "rejected"
	if confirm {
		next = "confirmed"
	}
	if _, err = tx.Exec(ctx, `UPDATE mechanic_store_trade_requests SET status=$2,confirmed_by_role=$3,confirmed_by_user_id=$4,resolved_at=now() WHERE id=$1`, requestID, next, resolverRole, actorID); err != nil {
		return Request{}, err
	}
	if confirm {
		debit, credit := int64(0), int64(0)
		if x.RequestType == "charge" || x.RequestType == "adjustment_debit" {
			debit = x.Amount
		} else {
			credit = x.Amount
		}
		if _, err = tx.Exec(ctx, `INSERT INTO mechanic_store_ledger_entries(account_id,request_id,entry_type,reference_type,reference_id,debit,credit,note)
			VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,NULLIF($8,''))`, x.AccountID, x.ID, x.RequestType, x.ReferenceType, x.ReferenceID, debit, credit, x.Note); err != nil {
			return Request{}, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE mechanic_store_accounts SET updated_at=now() WHERE id=$1`, x.AccountID); err != nil {
		return Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	return s.requestByID(ctx, requestID)
}

func (s *Service) requestByID(ctx context.Context, id uuid.UUID) (Request, error) {
	return scanRequest(s.db.QueryRow(ctx, requestSelect+` WHERE r.id=$1`, id))
}

func ensureAccount(ctx context.Context, tx pgx.Tx, mechanicID uuid.UUID, name, email string, tenantID, storeID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `INSERT INTO mechanic_store_accounts(mechanic_user_id,mechanic_name,mechanic_email,tenant_id,store_id)
		VALUES($1,$2,NULLIF($3,''),$4,$5)
		ON CONFLICT(mechanic_user_id,store_id) DO UPDATE SET mechanic_name=CASE WHEN EXCLUDED.mechanic_name<>'' THEN EXCLUDED.mechanic_name ELSE mechanic_store_accounts.mechanic_name END,
		mechanic_email=COALESCE(EXCLUDED.mechanic_email,mechanic_store_accounts.mechanic_email),updated_at=now()
		RETURNING id`, mechanicID, strings.TrimSpace(name), strings.TrimSpace(email), tenantID, storeID).Scan(&id)
	return id, err
}

func validType(v string) bool {
	switch v {
	case "charge", "payment", "return", "credit", "adjustment_debit", "adjustment_credit":
		return true
	}
	return false
}

func validMethod(v string) bool {
	switch v {
	case "cash", "card", "transfer", "check", "credit", "other":
		return true
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := [20]byte{}
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
