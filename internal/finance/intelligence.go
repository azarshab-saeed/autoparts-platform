package finance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MaturityAverageCommand struct {
	TenantID      uuid.UUID   `json:"-"`
	StoreID       uuid.UUID   `json:"-"`
	CheckIDs      []uuid.UUID `json:"check_ids"`
	ReferenceDate string      `json:"reference_date,omitempty"`
}

type MaturityAverageItem struct {
	CheckID      uuid.UUID `json:"check_id"`
	CheckNumber  string    `json:"check_number"`
	Amount       int64     `json:"amount"`
	DueDate      string    `json:"due_date"`
	DaysFromBase int       `json:"days_from_reference"`
	WeightBPS    int       `json:"weight_bps"`
}

type MaturityAverageResult struct {
	Direction     string                `json:"direction"`
	Count         int                   `json:"count"`
	TotalAmount   int64                 `json:"total_amount"`
	ReferenceDate string                `json:"reference_date"`
	WeightedDays  int                   `json:"weighted_days"`
	MaturityDate  string                `json:"maturity_date"`
	Items         []MaturityAverageItem `json:"items"`
}

type maturityInput struct {
	ID        uuid.UUID
	Number    string
	Direction string
	Amount    int64
	Due       time.Time
	Status    string
}

func calculateMaturityAverage(reference time.Time, items []maturityInput) (MaturityAverageResult, error) {
	if len(items) == 0 {
		return MaturityAverageResult{}, errors.New("at least one check is required")
	}
	reference = time.Date(reference.Year(), reference.Month(), reference.Day(), 0, 0, 0, 0, time.UTC)
	direction := items[0].Direction
	var total int64
	var weighted float64
	for _, item := range items {
		if item.Amount <= 0 {
			return MaturityAverageResult{}, errors.New("check amount must be positive")
		}
		if item.Direction != direction {
			return MaturityAverageResult{}, errors.New("all checks must have the same direction")
		}
		days := int(item.Due.Sub(reference).Hours() / 24)
		total += item.Amount
		weighted += float64(item.Amount) * float64(days)
	}
	weightedDays := int(math.Round(weighted / float64(total)))
	out := MaturityAverageResult{Direction: direction, Count: len(items), TotalAmount: total, ReferenceDate: reference.Format("2006-01-02"), WeightedDays: weightedDays, MaturityDate: reference.AddDate(0, 0, weightedDays).Format("2006-01-02"), Items: make([]MaturityAverageItem, 0, len(items))}
	for _, item := range items {
		days := int(item.Due.Sub(reference).Hours() / 24)
		out.Items = append(out.Items, MaturityAverageItem{CheckID: item.ID, CheckNumber: item.Number, Amount: item.Amount, DueDate: item.Due.Format("2006-01-02"), DaysFromBase: days, WeightBPS: int(math.Round(float64(item.Amount) * 10000 / float64(total)))})
	}
	return out, nil
}

func (s *Service) MaturityAverage(ctx context.Context, cmd MaturityAverageCommand, now time.Time) (MaturityAverageResult, error) {
	if len(cmd.CheckIDs) == 0 || len(cmd.CheckIDs) > 100 {
		return MaturityAverageResult{}, errors.New("check_ids must contain between 1 and 100 checks")
	}
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(cmd.CheckIDs))
	for _, id := range cmd.CheckIDs {
		if id != uuid.Nil && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return MaturityAverageResult{}, errors.New("valid check_ids are required")
	}
	reference := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if strings.TrimSpace(cmd.ReferenceDate) != "" {
		v, err := parseCheckDate(cmd.ReferenceDate, "reference_date")
		if err != nil {
			return MaturityAverageResult{}, err
		}
		reference = v
	}
	rows, err := s.db.Query(ctx, `SELECT id,check_number,direction,amount,due_date,status FROM checks WHERE tenant_id=$1 AND store_id=$2 AND id=ANY($3::uuid[]) ORDER BY due_date,id`, cmd.TenantID, cmd.StoreID, ids)
	if err != nil {
		return MaturityAverageResult{}, err
	}
	defer rows.Close()
	items := make([]maturityInput, 0, len(ids))
	for rows.Next() {
		var x maturityInput
		if err := rows.Scan(&x.ID, &x.Number, &x.Direction, &x.Amount, &x.Due, &x.Status); err != nil {
			return MaturityAverageResult{}, err
		}
		open := (x.Direction == "receivable" && (x.Status == "held" || x.Status == "deposited" || x.Status == "returned")) || (x.Direction == "payable" && x.Status == "issued")
		if !open {
			return MaturityAverageResult{}, fmt.Errorf("check %s is not open and cannot be averaged", x.Number)
		}
		items = append(items, x)
	}
	if err := rows.Err(); err != nil {
		return MaturityAverageResult{}, err
	}
	if len(items) != len(ids) {
		return MaturityAverageResult{}, errors.New("one or more checks do not belong to authenticated store")
	}
	return calculateMaturityAverage(reference, items)
}

type MaturityBucket struct {
	Key              string `json:"key"`
	Label            string `json:"label"`
	ReceivableCount  int64  `json:"receivable_count"`
	ReceivableAmount int64  `json:"receivable_amount"`
	PayableCount     int64  `json:"payable_count"`
	PayableAmount    int64  `json:"payable_amount"`
}

type CashCalendarDay struct {
	Date             string `json:"date"`
	ReceivableAmount int64  `json:"receivable_amount"`
	PayableAmount    int64  `json:"payable_amount"`
	Net              int64  `json:"net"`
	ProjectedBalance int64  `json:"projected_balance"`
}

type CustomerCheckRisk struct {
	CustomerID     uuid.UUID `json:"customer_id"`
	CustomerName   string    `json:"customer_name"`
	TotalCount     int64     `json:"total_count"`
	TotalAmount    int64     `json:"total_amount"`
	OpenAmount     int64     `json:"open_amount"`
	OverdueCount   int64     `json:"overdue_count"`
	OverdueAmount  int64     `json:"overdue_amount"`
	BouncedCount   int64     `json:"bounced_count"`
	BouncedAmount  int64     `json:"bounced_amount"`
	BounceRateBPS  int       `json:"bounce_rate_bps"`
	OverdueRateBPS int       `json:"overdue_rate_bps"`
	MaxOverdueDays int       `json:"max_overdue_days"`
	RiskLevel      string    `json:"risk_level"`
}

type FinanceIntelligenceDashboard struct {
	GeneratedAt             string              `json:"generated_at"`
	WindowDays              int                 `json:"window_days"`
	BankBalance             int64               `json:"bank_balance"`
	ReceivableOpenAmount    int64               `json:"receivable_open_amount"`
	PayableOpenAmount       int64               `json:"payable_open_amount"`
	OverdueReceivableAmount int64               `json:"overdue_receivable_amount"`
	OverduePayableAmount    int64               `json:"overdue_payable_amount"`
	Next30Net               int64               `json:"next_30_net"`
	ProjectedBankBalance30  int64               `json:"projected_bank_balance_30"`
	UnreconciledBankLines   int64               `json:"unreconciled_bank_lines"`
	UnreconciledBankAmount  int64               `json:"unreconciled_bank_amount"`
	Buckets                 []MaturityBucket    `json:"maturity_buckets"`
	Calendar                []CashCalendarDay   `json:"cash_calendar"`
	CustomerRisks           []CustomerCheckRisk `json:"customer_risks"`
}

func maturityBucketFor(due, today time.Time) string {
	days := int(due.Sub(today).Hours() / 24)
	switch {
	case days < 0:
		return "overdue"
	case days == 0:
		return "today"
	case days <= 7:
		return "1_7"
	case days <= 30:
		return "8_30"
	case days <= 60:
		return "31_60"
	default:
		return "61_plus"
	}
}

func classifyCustomerRisk(totalAmount, overdueAmount int64, totalCount, bouncedCount int64, maxOverdueDays int) (int, int, string) {
	bounceBPS, overdueBPS := 0, 0
	if totalCount > 0 {
		bounceBPS = int(math.Round(float64(bouncedCount) * 10000 / float64(totalCount)))
	}
	if totalAmount > 0 {
		overdueBPS = int(math.Round(float64(overdueAmount) * 10000 / float64(totalAmount)))
	}
	level := "low"
	if bouncedCount >= 2 || bounceBPS >= 2000 || overdueBPS >= 3000 || maxOverdueDays >= 30 {
		level = "high"
	} else if bouncedCount > 0 || overdueAmount > 0 || maxOverdueDays > 0 {
		level = "medium"
	}
	return bounceBPS, overdueBPS, level
}

func (s *Service) FinanceIntelligence(ctx context.Context, tenantID, storeID uuid.UUID, now time.Time, days int) (FinanceIntelligenceDashboard, error) {
	if days < 7 {
		days = 7
	}
	if days > 180 {
		days = 180
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end := today.AddDate(0, 0, days)
	out := FinanceIntelligenceDashboard{GeneratedAt: now.UTC().Format(time.RFC3339), WindowDays: days}
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(je.debit-je.credit),0)::bigint FROM store_bank_accounts b LEFT JOIN journal_entries je ON je.tenant_id=b.tenant_id AND je.account_id=b.account_id WHERE b.tenant_id=$1 AND b.store_id=$2 AND b.active`, tenantID, storeID).Scan(&out.BankBalance); err != nil {
		return out, err
	}
	defs := []struct{ key, label string }{{"overdue", "معوق"}, {"today", "امروز"}, {"1_7", "۱ تا ۷ روز"}, {"8_30", "۸ تا ۳۰ روز"}, {"31_60", "۳۱ تا ۶۰ روز"}, {"61_plus", "بیش از ۶۰ روز"}}
	bucketMap := map[string]*MaturityBucket{}
	for _, d := range defs {
		out.Buckets = append(out.Buckets, MaturityBucket{Key: d.key, Label: d.label})
		bucketMap[d.key] = &out.Buckets[len(out.Buckets)-1]
	}
	type dayAmounts struct{ in, out int64 }
	daily := map[string]*dayAmounts{}
	rows, err := s.db.Query(ctx, `SELECT direction,amount,due_date FROM checks WHERE tenant_id=$1 AND store_id=$2 AND ((direction='receivable' AND status IN ('held','deposited','returned')) OR (direction='payable' AND status='issued'))`, tenantID, storeID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var direction string
		var amount int64
		var due time.Time
		if err := rows.Scan(&direction, &amount, &due); err != nil {
			rows.Close()
			return out, err
		}
		b := bucketMap[maturityBucketFor(due, today)]
		if direction == "receivable" {
			out.ReceivableOpenAmount += amount
			b.ReceivableCount++
			b.ReceivableAmount += amount
			if due.Before(today) {
				out.OverdueReceivableAmount += amount
			}
		} else {
			out.PayableOpenAmount += amount
			b.PayableCount++
			b.PayableAmount += amount
			if due.Before(today) {
				out.OverduePayableAmount += amount
			}
		}
		if !due.Before(today) && !due.After(end) {
			key := due.Format("2006-01-02")
			x := daily[key]
			if x == nil {
				x = &dayAmounts{}
				daily[key] = x
			}
			if direction == "receivable" {
				x.in += amount
			} else {
				x.out += amount
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	balance := out.BankBalance
	for d := today; !d.After(end); d = d.AddDate(0, 0, 1) {
		x := daily[d.Format("2006-01-02")]
		if x == nil {
			x = &dayAmounts{}
		}
		net := x.in - x.out
		balance += net
		if x.in != 0 || x.out != 0 {
			out.Calendar = append(out.Calendar, CashCalendarDay{Date: d.Format("2006-01-02"), ReceivableAmount: x.in, PayableAmount: x.out, Net: net, ProjectedBalance: balance})
		}
		if !d.After(today.AddDate(0, 0, 30)) {
			out.Next30Net += net
		}
	}
	out.ProjectedBankBalance30 = out.BankBalance + out.Next30Net
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE COALESCE(m.matched,0)<ABS(l.amount)),COALESCE(SUM(ABS(l.amount)-COALESCE(m.matched,0)) FILTER (WHERE COALESCE(m.matched,0)<ABS(l.amount)),0)::bigint FROM bank_statement_lines l LEFT JOIN (SELECT statement_line_id,SUM(matched_amount)::bigint matched FROM bank_reconciliation_matches WHERE tenant_id=$1 GROUP BY statement_line_id) m ON m.statement_line_id=l.id WHERE l.tenant_id=$1 AND l.store_id=$2`, tenantID, storeID).Scan(&out.UnreconciledBankLines, &out.UnreconciledBankAmount); err != nil {
		return out, err
	}
	riskRows, err := s.db.Query(ctx, `SELECT c.id,c.name,COUNT(ch.id)::bigint,COALESCE(SUM(ch.amount),0)::bigint,COALESCE(SUM(ch.amount) FILTER (WHERE ch.status IN ('held','deposited','returned')),0)::bigint,COUNT(ch.id) FILTER (WHERE ch.status IN ('held','deposited','returned') AND ch.due_date<$3)::bigint,COALESCE(SUM(ch.amount) FILTER (WHERE ch.status IN ('held','deposited','returned') AND ch.due_date<$3),0)::bigint,COUNT(ch.id) FILTER (WHERE ch.status='bounced')::bigint,COALESCE(SUM(ch.amount) FILTER (WHERE ch.status='bounced'),0)::bigint,COALESCE(MAX(($3-ch.due_date)) FILTER (WHERE ch.status IN ('held','deposited','returned') AND ch.due_date<$3),0)::int FROM customers c JOIN checks ch ON ch.tenant_id=c.tenant_id AND ch.store_id=c.store_id AND ch.customer_id=c.id AND ch.direction='receivable' WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.deleted_at IS NULL GROUP BY c.id,c.name LIMIT 30`, tenantID, storeID, today)
	if err != nil {
		return out, err
	}
	for riskRows.Next() {
		var x CustomerCheckRisk
		if err := riskRows.Scan(&x.CustomerID, &x.CustomerName, &x.TotalCount, &x.TotalAmount, &x.OpenAmount, &x.OverdueCount, &x.OverdueAmount, &x.BouncedCount, &x.BouncedAmount, &x.MaxOverdueDays); err != nil {
			riskRows.Close()
			return out, err
		}
		x.BounceRateBPS, x.OverdueRateBPS, x.RiskLevel = classifyCustomerRisk(x.TotalAmount, x.OverdueAmount, x.TotalCount, x.BouncedCount, x.MaxOverdueDays)
		out.CustomerRisks = append(out.CustomerRisks, x)
	}
	if err := riskRows.Err(); err != nil {
		riskRows.Close()
		return out, err
	}
	riskRows.Close()
	sort.SliceStable(out.CustomerRisks, func(i, j int) bool {
		rank := map[string]int{"high": 3, "medium": 2, "low": 1}
		if rank[out.CustomerRisks[i].RiskLevel] != rank[out.CustomerRisks[j].RiskLevel] {
			return rank[out.CustomerRisks[i].RiskLevel] > rank[out.CustomerRisks[j].RiskLevel]
		}
		return out.CustomerRisks[i].BouncedAmount+out.CustomerRisks[i].OverdueAmount > out.CustomerRisks[j].BouncedAmount+out.CustomerRisks[j].OverdueAmount
	})
	return out, nil
}

type BankStatementInput struct {
	Date        string `json:"date"`
	Amount      int64  `json:"amount"`
	Description string `json:"description,omitempty"`
	Reference   string `json:"reference,omitempty"`
	ExternalID  string `json:"external_id,omitempty"`
}

type BankStatementImportCommand struct {
	TenantID      uuid.UUID            `json:"-"`
	StoreID       uuid.UUID            `json:"-"`
	BankAccountID uuid.UUID            `json:"-"`
	ActorUserID   uuid.UUID            `json:"-"`
	Lines         []BankStatementInput `json:"lines"`
}

type BankStatementImportResult struct {
	Imported   int `json:"imported"`
	Duplicates int `json:"duplicates"`
}

type BankStatementLine struct {
	ID                 uuid.UUID `json:"id"`
	BankAccountID      uuid.UUID `json:"bank_account_id"`
	Date               string    `json:"date"`
	Amount             int64     `json:"amount"`
	Description        string    `json:"description,omitempty"`
	Reference          string    `json:"reference,omitempty"`
	ExternalID         string    `json:"external_id,omitempty"`
	MatchedAmount      int64     `json:"matched_amount"`
	RemainingAmount    int64     `json:"remaining_amount"`
	Status             string    `json:"status"`
	DuplicateSuspected bool      `json:"duplicate_suspected"`
	CreatedAt          string    `json:"created_at"`
}

func bankStatementFingerprint(accountID uuid.UUID, date time.Time, amount int64, description, reference, externalID string) string {
	raw := strings.Join([]string{accountID.String(), date.Format("2006-01-02"), fmt.Sprintf("%d", amount), strings.ToLower(strings.TrimSpace(description)), strings.ToLower(strings.TrimSpace(reference)), strings.ToLower(strings.TrimSpace(externalID))}, "|")
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (s *Service) ImportBankStatement(ctx context.Context, cmd BankStatementImportCommand) (BankStatementImportResult, error) {
	if len(cmd.Lines) == 0 || len(cmd.Lines) > 1000 {
		return BankStatementImportResult{}, errors.New("statement import must contain between 1 and 1000 lines")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return BankStatementImportResult{}, err
	}
	defer tx.Rollback(ctx)
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM store_bank_accounts WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND active)`, cmd.BankAccountID, cmd.TenantID, cmd.StoreID).Scan(&ok); err != nil {
		return BankStatementImportResult{}, err
	}
	if !ok {
		return BankStatementImportResult{}, errors.New("bank account does not belong to authenticated store")
	}
	out := BankStatementImportResult{}
	for i, line := range cmd.Lines {
		if line.Amount == 0 {
			return out, fmt.Errorf("statement line %d amount cannot be zero", i+1)
		}
		date, err := parseCheckDate(line.Date, fmt.Sprintf("lines[%d].date", i))
		if err != nil {
			return out, err
		}
		fingerprint := bankStatementFingerprint(cmd.BankAccountID, date, line.Amount, line.Description, line.Reference, line.ExternalID)
		id := uuid.New()
		var inserted uuid.UUID
		err = tx.QueryRow(ctx, `INSERT INTO bank_statement_lines(id,tenant_id,store_id,bank_account_id,txn_date,amount,description,reference,external_id,fingerprint,imported_by) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10,$11) ON CONFLICT(tenant_id,bank_account_id,fingerprint) DO NOTHING RETURNING id`, id, cmd.TenantID, cmd.StoreID, cmd.BankAccountID, date, line.Amount, strings.TrimSpace(line.Description), strings.TrimSpace(line.Reference), strings.TrimSpace(line.ExternalID), fingerprint, cmd.ActorUserID).Scan(&inserted)
		if errors.Is(err, pgx.ErrNoRows) {
			out.Duplicates++
			continue
		}
		if err != nil {
			return out, err
		}
		out.Imported++
		payload, _ := json.Marshal(map[string]any{"amount": line.Amount, "date": date.Format("2006-01-02")})
		if _, err := tx.Exec(ctx, `INSERT INTO bank_reconciliation_events(tenant_id,store_id,bank_account_id,statement_line_id,action,actor_user_id,payload) VALUES($1,$2,$3,$4,'statement_imported',$5,$6)`, cmd.TenantID, cmd.StoreID, cmd.BankAccountID, inserted, cmd.ActorUserID, payload); err != nil {
			return out, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
func sameSign(a, b int64) bool { return (a > 0 && b > 0) || (a < 0 && b < 0) }

func (s *Service) ListBankStatementLines(ctx context.Context, tenantID, storeID, bankAccountID uuid.UUID) ([]BankStatementLine, error) {
	rows, err := s.db.Query(ctx, `SELECT l.id,l.bank_account_id,l.txn_date,l.amount,COALESCE(l.description,''),COALESCE(l.reference,''),COALESCE(l.external_id,''),COALESCE(SUM(m.matched_amount),0)::bigint,(SELECT COUNT(*)>1 FROM bank_statement_lines d WHERE d.tenant_id=l.tenant_id AND d.store_id=l.store_id AND d.bank_account_id=l.bank_account_id AND d.txn_date=l.txn_date AND d.amount=l.amount),l.created_at FROM bank_statement_lines l LEFT JOIN bank_reconciliation_matches m ON m.tenant_id=l.tenant_id AND m.statement_line_id=l.id WHERE l.tenant_id=$1 AND l.store_id=$2 AND l.bank_account_id=$3 GROUP BY l.id ORDER BY l.txn_date DESC,l.created_at DESC LIMIT 1000`, tenantID, storeID, bankAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BankStatementLine{}
	for rows.Next() {
		var x BankStatementLine
		var d, c time.Time
		if err := rows.Scan(&x.ID, &x.BankAccountID, &d, &x.Amount, &x.Description, &x.Reference, &x.ExternalID, &x.MatchedAmount, &x.DuplicateSuspected, &c); err != nil {
			return nil, err
		}
		x.Date, x.CreatedAt = d.Format("2006-01-02"), c.Format(time.RFC3339)
		x.RemainingAmount = abs64(x.Amount) - x.MatchedAmount
		if x.RemainingAmount < 0 {
			x.RemainingAmount = 0
		}
		if x.MatchedAmount == 0 {
			x.Status = "unmatched"
		} else if x.RemainingAmount == 0 {
			x.Status = "matched"
		} else {
			x.Status = "partial"
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

type ReconciliationCandidate struct {
	JournalEntryID  uuid.UUID `json:"journal_entry_id"`
	JournalID       uuid.UUID `json:"journal_id"`
	ReferenceType   string    `json:"reference_type"`
	ReferenceID     uuid.UUID `json:"reference_id"`
	PostedAt        string    `json:"posted_at"`
	Change          int64     `json:"change"`
	MatchedAmount   int64     `json:"matched_amount"`
	RemainingAmount int64     `json:"remaining_amount"`
	ExactAmount     bool      `json:"exact_amount"`
}

func (s *Service) ReconciliationCandidates(ctx context.Context, tenantID, storeID, bankAccountID, statementLineID uuid.UUID) ([]ReconciliationCandidate, error) {
	var amount int64
	var txnDate time.Time
	var bankGL uuid.UUID
	var matched int64
	if err := s.db.QueryRow(ctx, `SELECT l.amount,l.txn_date,b.account_id,COALESCE((SELECT SUM(matched_amount)::bigint FROM bank_reconciliation_matches m WHERE m.tenant_id=l.tenant_id AND m.statement_line_id=l.id),0)::bigint FROM bank_statement_lines l JOIN store_bank_accounts b ON b.id=l.bank_account_id AND b.tenant_id=l.tenant_id AND b.store_id=l.store_id WHERE l.id=$1 AND l.tenant_id=$2 AND l.store_id=$3 AND l.bank_account_id=$4`, statementLineID, tenantID, storeID, bankAccountID).Scan(&amount, &txnDate, &bankGL, &matched); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("statement line does not belong to selected bank account")
		}
		return nil, err
	}
	remaining := abs64(amount) - matched
	if remaining <= 0 {
		return []ReconciliationCandidate{}, nil
	}
	rows, err := s.db.Query(ctx, `SELECT je.id,je.journal_id,j.reference_type,j.reference_id,j.posted_at,(je.debit-je.credit)::bigint,COALESCE(SUM(m.matched_amount),0)::bigint FROM journal_entries je JOIN journals j ON j.id=je.journal_id AND j.tenant_id=je.tenant_id LEFT JOIN bank_reconciliation_matches m ON m.tenant_id=je.tenant_id AND m.journal_entry_id=je.id WHERE je.tenant_id=$1 AND je.account_id=$2 AND j.reference_type<>'bank_opening' AND (($3::bigint>0 AND je.debit>je.credit) OR ($3::bigint<0 AND je.credit>je.debit)) AND j.posted_at::date BETWEEN $4::date-INTERVAL '14 days' AND $4::date+INTERVAL '14 days' GROUP BY je.id,j.id HAVING ABS((je.debit-je.credit)::bigint)>COALESCE(SUM(m.matched_amount),0) ORDER BY (ABS((je.debit-je.credit)::bigint)-COALESCE(SUM(m.matched_amount),0)=$5::bigint) DESC,ABS(j.posted_at::date-$4::date) LIMIT 50`, tenantID, bankGL, amount, txnDate, remaining)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReconciliationCandidate{}
	for rows.Next() {
		var x ReconciliationCandidate
		var posted time.Time
		if err := rows.Scan(&x.JournalEntryID, &x.JournalID, &x.ReferenceType, &x.ReferenceID, &posted, &x.Change, &x.MatchedAmount); err != nil {
			return nil, err
		}
		x.RemainingAmount = abs64(x.Change) - x.MatchedAmount
		x.ExactAmount = x.RemainingAmount == remaining
		x.PostedAt = posted.Format(time.RFC3339)
		out = append(out, x)
	}
	return out, rows.Err()
}

type ReconciliationMatchCommand struct {
	TenantID        uuid.UUID `json:"-"`
	StoreID         uuid.UUID `json:"-"`
	ActorUserID     uuid.UUID `json:"-"`
	StatementLineID uuid.UUID `json:"-"`
	JournalEntryID  uuid.UUID `json:"journal_entry_id"`
	Amount          int64     `json:"amount,omitempty"`
	Note            string    `json:"note,omitempty"`
}

type ReconciliationMatch struct {
	ID              uuid.UUID `json:"id"`
	StatementLineID uuid.UUID `json:"statement_line_id"`
	JournalEntryID  uuid.UUID `json:"journal_entry_id"`
	MatchedAmount   int64     `json:"matched_amount"`
	Note            string    `json:"note,omitempty"`
	ReferenceType   string    `json:"reference_type,omitempty"`
	ReferenceID     uuid.UUID `json:"reference_id,omitempty"`
	PostedAt        string    `json:"posted_at,omitempty"`
	CreatedAt       string    `json:"created_at"`
}

func (s *Service) MatchBankStatementLine(ctx context.Context, cmd ReconciliationMatchCommand) (ReconciliationMatch, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return ReconciliationMatch{}, err
	}
	defer tx.Rollback(ctx)
	var lineAmount, lineMatched int64
	var bankAccountID, bankGL uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT l.amount,l.bank_account_id,b.account_id,COALESCE((SELECT SUM(matched_amount)::bigint FROM bank_reconciliation_matches m WHERE m.tenant_id=l.tenant_id AND m.statement_line_id=l.id),0)::bigint FROM bank_statement_lines l JOIN store_bank_accounts b ON b.id=l.bank_account_id AND b.tenant_id=l.tenant_id AND b.store_id=l.store_id WHERE l.id=$1 AND l.tenant_id=$2 AND l.store_id=$3 FOR UPDATE OF l`, cmd.StatementLineID, cmd.TenantID, cmd.StoreID).Scan(&lineAmount, &bankAccountID, &bankGL, &lineMatched); err != nil {
		return ReconciliationMatch{}, err
	}
	lineRemaining := abs64(lineAmount) - lineMatched
	if lineRemaining <= 0 {
		return ReconciliationMatch{}, errors.New("statement line is already fully matched")
	}
	var change, entryMatched int64
	if err := tx.QueryRow(ctx, `SELECT (je.debit-je.credit)::bigint,COALESCE((SELECT SUM(matched_amount)::bigint FROM bank_reconciliation_matches m WHERE m.tenant_id=je.tenant_id AND m.journal_entry_id=je.id),0)::bigint FROM journal_entries je JOIN journals j ON j.id=je.journal_id AND j.tenant_id=je.tenant_id WHERE je.id=$1 AND je.tenant_id=$2 AND je.account_id=$3 AND j.reference_type<>'bank_opening' FOR SHARE OF je`, cmd.JournalEntryID, cmd.TenantID, bankGL).Scan(&change, &entryMatched); err != nil {
		return ReconciliationMatch{}, err
	}
	if !sameSign(lineAmount, change) {
		return ReconciliationMatch{}, errors.New("statement and journal directions do not match")
	}
	entryRemaining := abs64(change) - entryMatched
	amount := cmd.Amount
	if amount == 0 {
		amount = lineRemaining
		if entryRemaining < amount {
			amount = entryRemaining
		}
	}
	if amount <= 0 || amount > lineRemaining || amount > entryRemaining {
		return ReconciliationMatch{}, errors.New("match amount exceeds remaining amount")
	}
	id := uuid.New()
	var created time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO bank_reconciliation_matches(id,tenant_id,store_id,statement_line_id,journal_entry_id,matched_amount,note,matched_by) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8) RETURNING created_at`, id, cmd.TenantID, cmd.StoreID, cmd.StatementLineID, cmd.JournalEntryID, amount, strings.TrimSpace(cmd.Note), cmd.ActorUserID).Scan(&created); err != nil {
		return ReconciliationMatch{}, err
	}
	payload, _ := json.Marshal(map[string]any{"journal_entry_id": cmd.JournalEntryID, "matched_amount": amount})
	if _, err := tx.Exec(ctx, `INSERT INTO bank_reconciliation_events(tenant_id,store_id,bank_account_id,statement_line_id,match_id,action,actor_user_id,payload) VALUES($1,$2,$3,$4,$5,'matched',$6,$7)`, cmd.TenantID, cmd.StoreID, bankAccountID, cmd.StatementLineID, id, cmd.ActorUserID, payload); err != nil {
		return ReconciliationMatch{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ReconciliationMatch{}, err
	}
	return ReconciliationMatch{ID: id, StatementLineID: cmd.StatementLineID, JournalEntryID: cmd.JournalEntryID, MatchedAmount: amount, CreatedAt: created.Format(time.RFC3339)}, nil
}

func (s *Service) ListReconciliationMatches(ctx context.Context, tenantID, storeID, statementLineID uuid.UUID) ([]ReconciliationMatch, error) {
	rows, err := s.db.Query(ctx, `SELECT m.id,m.statement_line_id,m.journal_entry_id,m.matched_amount,COALESCE(m.note,''),m.created_at,j.reference_type,j.reference_id,j.posted_at
FROM bank_reconciliation_matches m
JOIN bank_statement_lines l ON l.id=m.statement_line_id AND l.tenant_id=m.tenant_id AND l.store_id=m.store_id
JOIN journal_entries je ON je.id=m.journal_entry_id AND je.tenant_id=m.tenant_id
JOIN journals j ON j.id=je.journal_id AND j.tenant_id=je.tenant_id
WHERE m.tenant_id=$1 AND m.store_id=$2 AND m.statement_line_id=$3
ORDER BY m.created_at`, tenantID, storeID, statementLineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReconciliationMatch{}
	for rows.Next() {
		var x ReconciliationMatch
		var created, posted time.Time
		if err := rows.Scan(&x.ID, &x.StatementLineID, &x.JournalEntryID, &x.MatchedAmount, &x.Note, &created, &x.ReferenceType, &x.ReferenceID, &posted); err != nil {
			return nil, err
		}
		x.CreatedAt = created.Format(time.RFC3339)
		x.PostedAt = posted.Format(time.RFC3339)
		out = append(out, x)
	}
	return out, rows.Err()
}

type ReconciliationUnmatchCommand struct {
	TenantID        uuid.UUID
	StoreID         uuid.UUID
	ActorUserID     uuid.UUID
	StatementLineID uuid.UUID
	MatchID         uuid.UUID
}

func (s *Service) UnmatchBankStatementLine(ctx context.Context, cmd ReconciliationUnmatchCommand) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var bankAccountID, journalEntryID uuid.UUID
	var matchedAmount int64
	if err := tx.QueryRow(ctx, `SELECT l.bank_account_id,m.journal_entry_id,m.matched_amount
FROM bank_reconciliation_matches m
JOIN bank_statement_lines l ON l.id=m.statement_line_id AND l.tenant_id=m.tenant_id AND l.store_id=m.store_id
WHERE m.id=$1 AND m.statement_line_id=$2 AND m.tenant_id=$3 AND m.store_id=$4
FOR UPDATE OF m,l`, cmd.MatchID, cmd.StatementLineID, cmd.TenantID, cmd.StoreID).Scan(&bankAccountID, &journalEntryID, &matchedAmount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("reconciliation match not found")
		}
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM bank_reconciliation_matches WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, cmd.MatchID, cmd.TenantID, cmd.StoreID); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"journal_entry_id": journalEntryID, "matched_amount": matchedAmount})
	if _, err := tx.Exec(ctx, `INSERT INTO bank_reconciliation_events(tenant_id,store_id,bank_account_id,statement_line_id,match_id,action,actor_user_id,payload) VALUES($1,$2,$3,$4,$5,'unmatched',$6,$7)`, cmd.TenantID, cmd.StoreID, bankAccountID, cmd.StatementLineID, cmd.MatchID, cmd.ActorUserID, payload); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
