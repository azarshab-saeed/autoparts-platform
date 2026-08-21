package management

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type KPIs struct {
	SalesAmount          int64 `json:"sales_amount"`
	SalesCount           int64 `json:"sales_count"`
	GrossProfit          int64 `json:"gross_profit"`
	DiscountAmount       int64 `json:"discount_amount"`
	CustomerDueAmount    int64 `json:"customer_due_amount"`
	ReturnAmount         int64 `json:"return_amount"`
	ReturnCount          int64 `json:"return_count"`
	BelowMarginLines     int64 `json:"below_margin_lines"`
	OverrideLines        int64 `json:"override_lines"`
	UnattributedSales    int64 `json:"unattributed_sales"`
	ActiveMechanics      int64 `json:"active_mechanics"`
	MechanicReceivable   int64 `json:"mechanic_receivable"`
	PendingTradeAmount   int64 `json:"pending_trade_amount"`
	NetworkWorkshopSales int64 `json:"network_workshop_parts_amount"`
}

type UserPerformance struct {
	UserID           uuid.UUID `json:"user_id"`
	Role             string    `json:"role"`
	SalesCount       int64     `json:"sales_count"`
	SalesAmount      int64     `json:"sales_amount"`
	GrossProfit      int64     `json:"gross_profit"`
	DiscountAmount   int64     `json:"discount_amount"`
	OverrideLines    int64     `json:"override_lines"`
	BelowMarginLines int64     `json:"below_margin_lines"`
	ReturnCount      int64     `json:"return_count"`
	ReturnAmount     int64     `json:"return_amount"`
}

type InventorySignal struct {
	ProductID    uuid.UUID  `json:"product_id"`
	Title        string     `json:"title"`
	OnHand       float64    `json:"on_hand"`
	Available    float64    `json:"available"`
	Sold30       float64    `json:"sold_30_days"`
	LastSaleAt   *time.Time `json:"last_sale_at,omitempty"`
	DaysIdle     int        `json:"days_idle"`
	Signal       string     `json:"signal"` // reorder | slow | dead
	SuggestedQty float64    `json:"suggested_qty"`
}

type CustomerSignal struct {
	CustomerID uuid.UUID  `json:"customer_id"`
	Name       string     `json:"name"`
	Balance    int64      `json:"balance"`
	LastSaleAt *time.Time `json:"last_sale_at,omitempty"`
	DaysIdle   int        `json:"days_idle"`
	Signal     string     `json:"signal"` // receivable | inactive | declining
}

type MechanicSignal struct {
	MechanicUserID uuid.UUID `json:"mechanic_user_id"`
	MechanicName   string    `json:"mechanic_name"`
	Balance        int64     `json:"balance"`
	PendingAmount  int64     `json:"pending_amount"`
	Purchases30    int64     `json:"network_workshop_parts_amount_30_days"`
}

type Action struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Href     string `json:"href"`
}

type Overview struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Days        int               `json:"days"`
	KPIs        KPIs              `json:"kpis"`
	Users       []UserPerformance `json:"users"`
	Inventory   []InventorySignal `json:"inventory_signals"`
	Customers   []CustomerSignal  `json:"customer_signals"`
	Mechanics   []MechanicSignal  `json:"mechanic_signals"`
	Actions     []Action          `json:"actions"`
}

func (s *Service) Overview(ctx context.Context, tenantID, storeID uuid.UUID, days int, now time.Time) (Overview, error) {
	if days < 7 {
		days = 7
	}
	if days > 180 {
		days = 180
	}
	out := Overview{GeneratedAt: now.UTC(), Days: days, Users: []UserPerformance{}, Inventory: []InventorySignal{}, Customers: []CustomerSignal{}, Mechanics: []MechanicSignal{}, Actions: []Action{}}
	from := now.AddDate(0, 0, -days)
	if err := s.kpis(ctx, tenantID, storeID, from, &out.KPIs); err != nil {
		return out, err
	}
	users, err := s.users(ctx, tenantID, storeID, from)
	if err != nil {
		return out, err
	}
	out.Users = users
	inventory, err := s.inventory(ctx, tenantID, storeID, now)
	if err != nil {
		return out, err
	}
	out.Inventory = inventory
	customers, err := s.customers(ctx, tenantID, storeID, now)
	if err != nil {
		return out, err
	}
	out.Customers = customers
	mechanics, err := s.mechanics(ctx, tenantID, storeID)
	if err != nil {
		return out, err
	}
	out.Mechanics = mechanics
	out.Actions = buildActions(out)
	return out, nil
}

func (s *Service) kpis(ctx context.Context, tenantID, storeID uuid.UUID, from time.Time, k *KPIs) error {
	if err := s.db.QueryRow(ctx, `WITH sf AS (
		SELECT s.id,s.total_amount,s.discount_amount,s.due_amount,s.actor_user_id,
			COALESCE(SUM(si.line_total - round(si.qty::numeric*si.unit_cost)),0)::bigint gp,
			COUNT(*) FILTER (WHERE si.below_margin_guard) below_count,
			COUNT(*) FILTER (WHERE si.price_override) override_count
		FROM sales s JOIN sale_items si ON si.sale_id=s.id AND si.tenant_id=s.tenant_id
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.status='posted' AND s.created_at >= $3
		GROUP BY s.id
	)
	SELECT COALESCE(SUM(total_amount),0)::bigint,COUNT(*)::bigint,COALESCE(SUM(gp),0)::bigint,
		COALESCE(SUM(discount_amount),0)::bigint,COALESCE(SUM(due_amount),0)::bigint,
		COALESCE(SUM(below_count),0)::bigint,COALESCE(SUM(override_count),0)::bigint,
		COUNT(*) FILTER (WHERE actor_user_id IS NULL)::bigint FROM sf`, tenantID, storeID, from).
		Scan(&k.SalesAmount, &k.SalesCount, &k.GrossProfit, &k.DiscountAmount, &k.CustomerDueAmount, &k.BelowMarginLines, &k.OverrideLines, &k.UnattributedSales); err != nil {
		return err
	}
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*)::bigint,COALESCE(SUM(total_amount),0)::bigint FROM sales_returns WHERE tenant_id=$1 AND store_id=$2 AND created_at >= $3`, tenantID, storeID, from).Scan(&k.ReturnCount, &k.ReturnAmount); err != nil {
		return err
	}
	if err := s.db.QueryRow(ctx, `WITH balances AS (
		SELECT a.mechanic_user_id,a.id,COALESCE(SUM(l.debit-l.credit),0)::bigint balance
		FROM mechanic_store_accounts a LEFT JOIN mechanic_store_ledger_entries l ON l.account_id=a.id
		WHERE a.tenant_id=$1 AND a.store_id=$2 GROUP BY a.id
	), pending AS (
		SELECT COALESCE(SUM(r.amount),0)::bigint amount FROM mechanic_store_trade_requests r
		JOIN mechanic_store_accounts a ON a.id=r.account_id WHERE a.tenant_id=$1 AND a.store_id=$2 AND r.status='pending'
	)
	SELECT COUNT(*) FILTER (WHERE balance<>0)::bigint,COALESCE(SUM(GREATEST(balance,0)),0)::bigint,(SELECT amount FROM pending) FROM balances`, tenantID, storeID).
		Scan(&k.ActiveMechanics, &k.MechanicReceivable, &k.PendingTradeAmount); err != nil {
		return err
	}
	return s.db.QueryRow(ctx, `SELECT COALESCE(SUM(i.line_total),0)::bigint FROM workshop_job_items i
		WHERE i.source_store_id=$1 AND i.created_at >= $2`, storeID, from).Scan(&k.NetworkWorkshopSales)
}

func (s *Service) users(ctx context.Context, tenantID, storeID uuid.UUID, from time.Time) ([]UserPerformance, error) {
	rows, err := s.db.Query(ctx, `WITH sf AS (
		SELECT s.id,s.actor_user_id,COALESCE(s.actor_role,'') role,s.total_amount,s.discount_amount,
			COALESCE(SUM(si.line_total-round(si.qty::numeric*si.unit_cost)),0)::bigint gp,
			COUNT(*) FILTER (WHERE si.price_override) overrides,
			COUNT(*) FILTER (WHERE si.below_margin_guard) below
		FROM sales s JOIN sale_items si ON si.sale_id=s.id AND si.tenant_id=s.tenant_id
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.status='posted' AND s.created_at >= $3 AND s.actor_user_id IS NOT NULL
		GROUP BY s.id
	), rf AS (
		SELECT actor_user_id,COUNT(*)::bigint returns,COALESCE(SUM(total_amount),0)::bigint return_amount
		FROM sales_returns WHERE tenant_id=$1 AND store_id=$2 AND created_at >= $3 AND actor_user_id IS NOT NULL GROUP BY actor_user_id
	)
	SELECT sf.actor_user_id,MAX(sf.role),COUNT(*)::bigint,COALESCE(SUM(sf.total_amount),0)::bigint,
		COALESCE(SUM(sf.gp),0)::bigint,COALESCE(SUM(sf.discount_amount),0)::bigint,
		COALESCE(SUM(sf.overrides),0)::bigint,COALESCE(SUM(sf.below),0)::bigint,
		COALESCE(MAX(rf.returns),0)::bigint,COALESCE(MAX(rf.return_amount),0)::bigint
	FROM (SELECT id,actor_user_id,role,total_amount,discount_amount,gp,overrides,below FROM sf) sf
	LEFT JOIN rf ON rf.actor_user_id=sf.actor_user_id
	GROUP BY sf.actor_user_id ORDER BY SUM(sf.total_amount) DESC LIMIT 30`, tenantID, storeID, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserPerformance{}
	for rows.Next() {
		var x UserPerformance
		if err := rows.Scan(&x.UserID, &x.Role, &x.SalesCount, &x.SalesAmount, &x.GrossProfit, &x.DiscountAmount, &x.OverrideLines, &x.BelowMarginLines, &x.ReturnCount, &x.ReturnAmount); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) inventory(ctx context.Context, tenantID, storeID uuid.UUID, now time.Time) ([]InventorySignal, error) {
	rows, err := s.db.Query(ctx, `WITH sold AS (
		SELECT im.product_id,COALESCE(SUM(-im.qty_delta) FILTER (WHERE im.created_at>=now()-interval '30 days'),0)::float8 sold30,
			MAX(im.created_at) last_sale
		FROM inventory_movements im JOIN warehouses w ON w.id=im.warehouse_id AND w.tenant_id=im.tenant_id
		WHERE im.tenant_id=$1 AND w.store_id=$2 AND im.movement_type='sale' GROUP BY im.product_id
	), stock AS (
		SELECT ib.product_id,SUM(ib.on_hand)::float8 on_hand,SUM(ib.on_hand-ib.reserved)::float8 available
		FROM inventory_balances ib JOIN warehouses w ON w.id=ib.warehouse_id AND w.tenant_id=ib.tenant_id
		WHERE ib.tenant_id=$1 AND w.store_id=$2 GROUP BY ib.product_id
	)
	SELECT p.id,p.title,st.on_hand,st.available,COALESCE(sd.sold30,0),sd.last_sale
	FROM stock st JOIN products p ON p.id=st.product_id AND p.tenant_id=$1
	LEFT JOIN sold sd ON sd.product_id=st.product_id
	WHERE st.on_hand>0 AND p.active AND p.deleted_at IS NULL
	ORDER BY CASE WHEN COALESCE(sd.sold30,0)>0 AND st.available < sd.sold30*0.35 THEN 0
		WHEN sd.last_sale IS NULL OR sd.last_sale < now()-interval '90 days' THEN 1
		WHEN sd.last_sale < now()-interval '45 days' THEN 2 ELSE 3 END,
		COALESCE(sd.last_sale,'1970-01-01'::timestamptz) ASC LIMIT 20`, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InventorySignal{}
	for rows.Next() {
		var x InventorySignal
		if err := rows.Scan(&x.ProductID, &x.Title, &x.OnHand, &x.Available, &x.Sold30, &x.LastSaleAt); err != nil {
			return nil, err
		}
		if x.LastSaleAt == nil {
			x.DaysIdle = 999
		} else {
			x.DaysIdle = int(now.Sub(*x.LastSaleAt).Hours() / 24)
		}
		switch {
		case x.Sold30 > 0 && x.Available < x.Sold30*0.35:
			x.Signal = "reorder"
			target := x.Sold30 * 1.2
			if target > x.Available {
				x.SuggestedQty = target - x.Available
			}
		case x.DaysIdle >= 90:
			x.Signal = "dead"
		case x.DaysIdle >= 45:
			x.Signal = "slow"
		default:
			continue
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) customers(ctx context.Context, tenantID, storeID uuid.UUID, now time.Time) ([]CustomerSignal, error) {
	rows, err := s.db.Query(ctx, `WITH bal AS (
		SELECT customer_id,COALESCE(SUM(debit-credit),0)::bigint balance FROM party_ledger_entries
		WHERE tenant_id=$1 AND store_id=$2 AND party_type='customer' GROUP BY customer_id
	), last_sale AS (
		SELECT customer_id,MAX(created_at) last_at FROM sales WHERE tenant_id=$1 AND store_id=$2 AND customer_id IS NOT NULL AND status='posted' GROUP BY customer_id
	)
	SELECT c.id,c.name,COALESCE(b.balance,0)::bigint,ls.last_at
	FROM customers c LEFT JOIN bal b ON b.customer_id=c.id LEFT JOIN last_sale ls ON ls.customer_id=c.id
	WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.deleted_at IS NULL
		AND (COALESCE(b.balance,0)>0 OR (ls.last_at IS NOT NULL AND ls.last_at<now()-interval '60 days'))
	ORDER BY CASE WHEN COALESCE(b.balance,0)>0 THEN 0 ELSE 1 END,COALESCE(b.balance,0) DESC,ls.last_at ASC LIMIT 20`, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CustomerSignal{}
	for rows.Next() {
		var x CustomerSignal
		if err := rows.Scan(&x.CustomerID, &x.Name, &x.Balance, &x.LastSaleAt); err != nil {
			return nil, err
		}
		if x.LastSaleAt != nil {
			x.DaysIdle = int(now.Sub(*x.LastSaleAt).Hours() / 24)
		}
		if x.Balance > 0 {
			x.Signal = "receivable"
		} else {
			x.Signal = "inactive"
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) mechanics(ctx context.Context, tenantID, storeID uuid.UUID) ([]MechanicSignal, error) {
	rows, err := s.db.Query(ctx, `SELECT a.mechanic_user_id,a.mechanic_name,
		COALESCE((SELECT SUM(l.debit-l.credit) FROM mechanic_store_ledger_entries l WHERE l.account_id=a.id),0)::bigint,
		COALESCE((SELECT SUM(r.amount) FROM mechanic_store_trade_requests r WHERE r.account_id=a.id AND r.status='pending'),0)::bigint,
		COALESCE((SELECT SUM(i.line_total) FROM workshop_job_items i WHERE i.source_store_id=a.store_id AND i.created_at>=now()-interval '30 days'
			AND EXISTS(SELECT 1 FROM workshop_jobs j WHERE j.id=i.job_id AND j.mechanic_user_id=a.mechanic_user_id)),0)::bigint
	FROM mechanic_store_accounts a WHERE a.tenant_id=$1 AND a.store_id=$2
	ORDER BY 3 DESC,5 DESC LIMIT 30`, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MechanicSignal{}
	for rows.Next() {
		var x MechanicSignal
		if err := rows.Scan(&x.MechanicUserID, &x.MechanicName, &x.Balance, &x.PendingAmount, &x.Purchases30); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func buildActions(o Overview) []Action {
	out := []Action{}
	if o.KPIs.MechanicReceivable > 0 {
		out = append(out, Action{Kind: "mechanic_receivable", Severity: "high", Title: "مانده حساب مکانیک‌ها را پیگیری کن", Detail: fmt.Sprintf("%d تومان طلب تأییدشده از مکانیک‌های شبکه ثبت شده است.", o.KPIs.MechanicReceivable), Href: "/store/mechanics"})
	}
	if o.KPIs.PendingTradeAmount > 0 {
		out = append(out, Action{Kind: "trade_confirmation", Severity: "medium", Title: "تسویه‌های منتظر تأیید", Detail: fmt.Sprintf("%d تومان درخواست تجاری هنوز دوطرفه تأیید نشده است.", o.KPIs.PendingTradeAmount), Href: "/store/mechanics"})
	}
	if o.KPIs.BelowMarginLines > 0 {
		out = append(out, Action{Kind: "margin", Severity: "high", Title: "فروش زیر حد سود را بررسی کن", Detail: fmt.Sprintf("%d ردیف فروش زیر گارد مارجین در این بازه ثبت شده است.", o.KPIs.BelowMarginLines), Href: "/store/pricing"})
	}
	reorders, dead := 0, 0
	for _, x := range o.Inventory {
		if x.Signal == "reorder" {
			reorders++
		}
		if x.Signal == "dead" {
			dead++
		}
	}
	if reorders > 0 {
		out = append(out, Action{Kind: "reorder", Severity: "medium", Title: "کالاهای رو به اتمام را سفارش بده", Detail: fmt.Sprintf("%d کالا نسبت به سرعت فروش ۳۰ روزه نیاز به تأمین دارند.", reorders), Href: "/store/procurement"})
	}
	if dead > 0 {
		out = append(out, Action{Kind: "dead_stock", Severity: "medium", Title: "برای موجودی راکد تصمیم بگیر", Detail: fmt.Sprintf("%d کالا بیش از ۹۰ روز بدون فروش مانده‌اند.", dead), Href: "/store/inventory"})
	}
	inactive, receivable := 0, 0
	for _, x := range o.Customers {
		if x.Signal == "inactive" {
			inactive++
		} else if x.Signal == "receivable" {
			receivable++
		}
	}
	if receivable > 0 {
		out = append(out, Action{Kind: "customer_receivable", Severity: "medium", Title: "مطالبات مشتریان را پیگیری کن", Detail: fmt.Sprintf("%d مشتری مانده بدهکار دارند.", receivable), Href: "/store/accounts"})
	}
	if inactive > 0 {
		out = append(out, Action{Kind: "customer_inactive", Severity: "low", Title: "با مشتریان غیرفعال تماس بگیر", Detail: fmt.Sprintf("%d مشتری فعال قدیمی بیش از ۶۰ روز خرید نکرده‌اند.", inactive), Href: "/store/accounts"})
	}
	if o.KPIs.UnattributedSales > 0 {
		out = append(out, Action{Kind: "attribution", Severity: "low", Title: "بخشی از فروش قدیمی بدون کاربر ثبت شده", Detail: "فروش‌های قبل از Phase 15.17 به‌صورت تاریخی به کاربر نسبت داده نمی‌شوند؛ فروش‌های جدید کامل خواهند بود.", Href: "/store/intelligence"})
	}
	if len(out) == 0 {
		out = append(out, Action{Kind: "healthy", Severity: "low", Title: "اقدام فوری شناسایی نشد", Detail: "در داده‌های فعلی هشدار عملیاتی مهمی دیده نشد.", Href: "/store"})
	}
	return out
}
