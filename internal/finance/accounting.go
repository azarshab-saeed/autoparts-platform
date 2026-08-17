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

type ExpenseCategory struct {
	ID   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Name string    `json:"name"`
}

type ExpenseCommand struct {
	TenantID       uuid.UUID `json:"-"`
	StoreID        uuid.UUID `json:"-"`
	CategoryID     uuid.UUID `json:"category_id"`
	Method         string    `json:"method"`
	Amount         int64     `json:"amount"`
	Note           string    `json:"note,omitempty"`
	OccurredOn     string    `json:"occurred_on,omitempty"`
	IdempotencyKey string    `json:"-"`
}

type Expense struct {
	ID           uuid.UUID `json:"id"`
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryCode string    `json:"category_code"`
	CategoryName string    `json:"category_name"`
	Method       string    `json:"method"`
	Amount       int64     `json:"amount"`
	Note         string    `json:"note,omitempty"`
	OccurredOn   string    `json:"occurred_on"`
	CreatedAt    string    `json:"created_at"`
	Status       string    `json:"status"`
}

type ExpenseCategoryTotal struct {
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryCode string    `json:"category_code"`
	CategoryName string    `json:"category_name"`
	Amount       int64     `json:"amount"`
}

type ProfitLoss struct {
	From              string                 `json:"from"`
	To                string                 `json:"to"`
	GrossSales        int64                  `json:"gross_sales"`
	SalesReturns      int64                  `json:"sales_returns"`
	NetSales          int64                  `json:"net_sales"`
	COGS              int64                  `json:"cogs"`
	COGSReversed      int64                  `json:"cogs_reversed"`
	NetCOGS           int64                  `json:"net_cogs"`
	GrossProfit       int64                  `json:"gross_profit"`
	OperatingExpenses int64                  `json:"operating_expenses"`
	NetProfit         int64                  `json:"net_profit"`
	ExpenseBreakdown  []ExpenseCategoryTotal `json:"expense_breakdown"`
}

type PartyStatementLine struct {
	ID          uuid.UUID `json:"id"`
	EntryType   string    `json:"entry_type"`
	ReferenceID uuid.UUID `json:"reference_id"`
	Debit       int64     `json:"debit"`
	Credit      int64     `json:"credit"`
	Change      int64     `json:"change"`
	Balance     int64     `json:"balance"`
	CreatedAt   string    `json:"created_at"`
}

type PartyStatement struct {
	PartyType      string               `json:"party_type"`
	PartyID        uuid.UUID            `json:"party_id"`
	PartyName      string               `json:"party_name"`
	ClosingBalance int64                `json:"closing_balance"`
	Items          []PartyStatementLine `json:"items"`
}

var defaultExpenseCategories = []struct {
	code string
	name string
}{
	{"RENT", "اجاره"},
	{"PAYROLL", "حقوق و دستمزد"},
	{"UTILITIES", "آب، برق، گاز و اینترنت"},
	{"TRANSPORT", "حمل و رفت‌وآمد"},
	{"SUPPLIES", "ملزومات فروشگاه"},
	{"MARKETING", "تبلیغات و بازاریابی"},
	{"OTHER", "سایر هزینه‌ها"},
}

func (s *Service) ensureExpenseCategories(ctx context.Context, tenantID uuid.UUID) error {
	for _, x := range defaultExpenseCategories {
		if _, err := s.db.Exec(ctx, `
			INSERT INTO expense_categories(tenant_id,code,name)
			VALUES($1,$2,$3)
			ON CONFLICT(tenant_id,code) DO UPDATE SET name=EXCLUDED.name`, tenantID, x.code, x.name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListExpenseCategories(ctx context.Context, tenantID uuid.UUID) ([]ExpenseCategory, error) {
	if err := s.ensureExpenseCategories(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT id,code,name FROM expense_categories WHERE tenant_id=$1 AND active ORDER BY created_at,code`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExpenseCategory{}
	for rows.Next() {
		var x ExpenseCategory
		if err := rows.Scan(&x.ID, &x.Code, &x.Name); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func parseBusinessDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local), nil
	}
	v, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, errors.New("date must use YYYY-MM-DD")
	}
	return v, nil
}

func (s *Service) CreateExpense(ctx context.Context, cmd ExpenseCommand) (Expense, error) {
	if cmd.TenantID == uuid.Nil || cmd.StoreID == uuid.Nil || cmd.CategoryID == uuid.Nil {
		return Expense{}, errors.New("authenticated store and category_id are required")
	}
	if cmd.Method != "cash" && cmd.Method != "card" {
		return Expense{}, errors.New("method must be cash or card")
	}
	if cmd.Amount <= 0 {
		return Expense{}, errors.New("amount must be greater than zero")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return Expense{}, errors.New("idempotency key is required")
	}
	occurred, err := parseBusinessDate(cmd.OccurredOn)
	if err != nil {
		return Expense{}, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Expense{}, err
	}
	defer tx.Rollback(ctx)

	var existing Expense
	var occurredExisting time.Time
	var createdExisting time.Time
	err = tx.QueryRow(ctx, `
		SELECT e.id,e.category_id,c.code,c.name,e.method,e.amount,COALESCE(e.note,''),e.occurred_on,e.created_at
		FROM expenses e
		JOIN expense_categories c ON c.id=e.category_id AND c.tenant_id=e.tenant_id
		WHERE e.tenant_id=$1 AND e.idempotency_key=$2`, cmd.TenantID, cmd.IdempotencyKey).
		Scan(&existing.ID, &existing.CategoryID, &existing.CategoryCode, &existing.CategoryName, &existing.Method, &existing.Amount, &existing.Note, &occurredExisting, &createdExisting)
	if err == nil {
		existing.OccurredOn = occurredExisting.Format("2006-01-02")
		existing.CreatedAt = createdExisting.Format(time.RFC3339)
		existing.Status = "posted"
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Expense{}, err
	}

	var categoryCode, categoryName string
	err = tx.QueryRow(ctx, `SELECT code,name FROM expense_categories WHERE id=$1 AND tenant_id=$2 AND active FOR UPDATE`, cmd.CategoryID, cmd.TenantID).Scan(&categoryCode, &categoryName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Expense{}, errors.New("expense category does not belong to authenticated tenant")
	}
	if err != nil {
		return Expense{}, err
	}

	id := uuid.New()
	var created time.Time
	if err = tx.QueryRow(ctx, `
		INSERT INTO expenses(id,tenant_id,store_id,category_id,method,amount,note,occurred_on,idempotency_key)
		VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9)
		RETURNING created_at`, id, cmd.TenantID, cmd.StoreID, cmd.CategoryID, cmd.Method, cmd.Amount, strings.TrimSpace(cmd.Note), occurred, cmd.IdempotencyKey).Scan(&created); err != nil {
		return Expense{}, err
	}

	accounts, err := ensureAccounts(ctx, tx, cmd.TenantID)
	if err != nil {
		return Expense{}, err
	}
	var expenseAccount uuid.UUID
	if err = tx.QueryRow(ctx, `
		INSERT INTO accounts(tenant_id,code,name,type)
		VALUES($1,'OPERATING_EXPENSE','Operating Expenses','expense')
		ON CONFLICT(tenant_id,code) DO UPDATE SET name=EXCLUDED.name
		RETURNING id`, cmd.TenantID).Scan(&expenseAccount); err != nil {
		return Expense{}, err
	}
	journalID := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO journals(id,tenant_id,reference_type,reference_id) VALUES($1,$2,'expense',$3)`, journalID, cmd.TenantID, id); err != nil {
		return Expense{}, err
	}
	moneyCode := "CASH"
	if cmd.Method == "card" {
		moneyCode = "BANK_CARD"
	}
	if err = entry(ctx, tx, cmd.TenantID, journalID, expenseAccount, cmd.Amount, 0); err != nil {
		return Expense{}, err
	}
	if err = entry(ctx, tx, cmd.TenantID, journalID, accounts[moneyCode], 0, cmd.Amount); err != nil {
		return Expense{}, err
	}
	payload, _ := json.Marshal(map[string]any{"expense_id": id, "store_id": cmd.StoreID, "category_code": categoryCode, "amount": cmd.Amount, "method": cmd.Method})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'expense',$2,'expense.created',$3)`, cmd.TenantID, id, payload); err != nil {
		return Expense{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Expense{}, err
	}
	return Expense{ID: id, CategoryID: cmd.CategoryID, CategoryCode: categoryCode, CategoryName: categoryName, Method: cmd.Method, Amount: cmd.Amount, Note: strings.TrimSpace(cmd.Note), OccurredOn: occurred.Format("2006-01-02"), CreatedAt: created.Format(time.RFC3339), Status: "posted"}, nil
}

func (s *Service) ListExpenses(ctx context.Context, tenantID, storeID uuid.UUID, from, to time.Time, categoryID *uuid.UUID, limit, offset int) ([]Expense, error) {
	if err := s.ensureExpenseCategories(ctx, tenantID); err != nil {
		return nil, err
	}
	if to.Before(from) {
		return nil, errors.New("to must not be before from")
	}
	rows, err := s.db.Query(ctx, `
		SELECT e.id,e.category_id,c.code,c.name,e.method,e.amount,COALESCE(e.note,''),e.occurred_on,e.created_at
		FROM expenses e
		JOIN expense_categories c ON c.id=e.category_id AND c.tenant_id=e.tenant_id
		WHERE e.tenant_id=$1 AND e.store_id=$2
		  AND e.occurred_on >= $3::date AND e.occurred_on <= $4::date
		  AND ($5::uuid IS NULL OR e.category_id=$5)
		ORDER BY e.occurred_on DESC,e.created_at DESC
		LIMIT $6 OFFSET $7`, tenantID, storeID, from, to, categoryID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Expense{}
	for rows.Next() {
		var x Expense
		var occurred, created time.Time
		if err := rows.Scan(&x.ID, &x.CategoryID, &x.CategoryCode, &x.CategoryName, &x.Method, &x.Amount, &x.Note, &occurred, &created); err != nil {
			return nil, err
		}
		x.OccurredOn = occurred.Format("2006-01-02")
		x.CreatedAt = created.Format(time.RFC3339)
		x.Status = "posted"
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) ProfitLoss(ctx context.Context, tenantID, storeID uuid.UUID, from, to time.Time) (ProfitLoss, error) {
	if to.Before(from) {
		return ProfitLoss{}, errors.New("to must not be before from")
	}
	toExclusive := to.AddDate(0, 0, 1)
	out := ProfitLoss{From: from.Format("2006-01-02"), To: to.Format("2006-01-02"), ExpenseBreakdown: []ExpenseCategoryTotal{}}
	err := s.db.QueryRow(ctx, `
		SELECT
		  (SELECT COALESCE(SUM(total_amount),0)::bigint FROM sales WHERE tenant_id=$1 AND store_id=$2 AND status='posted' AND created_at >= $3 AND created_at < $4),
		  (SELECT COALESCE(SUM(total_amount),0)::bigint FROM sales_returns WHERE tenant_id=$1 AND store_id=$2 AND created_at >= $3 AND created_at < $4),
		  (SELECT COALESCE(SUM(ROUND(si.qty * si.unit_cost)),0)::bigint FROM sale_items si JOIN sales s ON s.id=si.sale_id AND s.tenant_id=si.tenant_id WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.status='posted' AND s.created_at >= $3 AND s.created_at < $4),
		  (SELECT COALESCE(SUM(ROUND(sri.qty * sri.unit_cost)),0)::bigint FROM sales_return_items sri JOIN sales_returns sr ON sr.id=sri.sales_return_id AND sr.tenant_id=sri.tenant_id WHERE sr.tenant_id=$1 AND sr.store_id=$2 AND sr.created_at >= $3 AND sr.created_at < $4),
		  (SELECT COALESCE(SUM(amount),0)::bigint FROM expenses WHERE tenant_id=$1 AND store_id=$2 AND occurred_on >= $3::date AND occurred_on <= $5::date)
	`, tenantID, storeID, from, toExclusive, to).Scan(&out.GrossSales, &out.SalesReturns, &out.COGS, &out.COGSReversed, &out.OperatingExpenses)
	if err != nil {
		return ProfitLoss{}, err
	}
	out.NetSales = out.GrossSales - out.SalesReturns
	out.NetCOGS = out.COGS - out.COGSReversed
	out.GrossProfit = out.NetSales - out.NetCOGS
	out.NetProfit = out.GrossProfit - out.OperatingExpenses

	rows, err := s.db.Query(ctx, `
		SELECT c.id,c.code,c.name,COALESCE(SUM(e.amount),0)::bigint
		FROM expenses e
		JOIN expense_categories c ON c.id=e.category_id AND c.tenant_id=e.tenant_id
		WHERE e.tenant_id=$1 AND e.store_id=$2 AND e.occurred_on >= $3::date AND e.occurred_on <= $4::date
		GROUP BY c.id,c.code,c.name
		ORDER BY SUM(e.amount) DESC,c.name`, tenantID, storeID, from, to)
	if err != nil {
		return ProfitLoss{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var x ExpenseCategoryTotal
		if err := rows.Scan(&x.CategoryID, &x.CategoryCode, &x.CategoryName, &x.Amount); err != nil {
			return ProfitLoss{}, err
		}
		out.ExpenseBreakdown = append(out.ExpenseBreakdown, x)
	}
	return out, rows.Err()
}

func (s *Service) PartyStatement(ctx context.Context, tenantID, storeID uuid.UUID, partyType string, partyID uuid.UUID) (PartyStatement, error) {
	if partyType != "customer" && partyType != "supplier" {
		return PartyStatement{}, errors.New("party_type must be customer or supplier")
	}
	if partyID == uuid.Nil {
		return PartyStatement{}, errors.New("party id is required")
	}
	out := PartyStatement{PartyType: partyType, PartyID: partyID, Items: []PartyStatementLine{}}
	var err error
	if partyType == "customer" {
		err = s.db.QueryRow(ctx, `SELECT name FROM customers WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND deleted_at IS NULL`, partyID, tenantID, storeID).Scan(&out.PartyName)
	} else {
		err = s.db.QueryRow(ctx, `SELECT name FROM suppliers WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND deleted_at IS NULL`, partyID, tenantID, storeID).Scan(&out.PartyName)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return PartyStatement{}, errors.New("party does not belong to authenticated store")
	}
	if err != nil {
		return PartyStatement{}, err
	}

	column := "customer_id"
	if partyType == "supplier" {
		column = "supplier_id"
	}
	query := fmt.Sprintf(`
		SELECT id,entry_type,reference_id,debit,credit,created_at
		FROM party_ledger_entries
		WHERE tenant_id=$1 AND store_id=$2 AND %s=$3
		ORDER BY created_at,id
		LIMIT 500`, column)
	rows, err := s.db.Query(ctx, query, tenantID, storeID, partyID)
	if err != nil {
		return PartyStatement{}, err
	}
	defer rows.Close()
	balance := int64(0)
	for rows.Next() {
		var x PartyStatementLine
		var created time.Time
		if err := rows.Scan(&x.ID, &x.EntryType, &x.ReferenceID, &x.Debit, &x.Credit, &created); err != nil {
			return PartyStatement{}, err
		}
		if partyType == "customer" {
			x.Change = x.Debit - x.Credit
		} else {
			x.Change = x.Credit - x.Debit
		}
		balance += x.Change
		x.Balance = balance
		x.CreatedAt = created.Format(time.RFC3339)
		out.Items = append(out.Items, x)
	}
	if err := rows.Err(); err != nil {
		return PartyStatement{}, err
	}
	out.ClosingBalance = balance
	return out, nil
}
