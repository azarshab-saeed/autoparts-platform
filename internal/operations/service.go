package operations

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) ListSales(ctx context.Context, tenantID, storeID uuid.UUID, from, to time.Time, customerID *uuid.UUID, q, paymentState string, limit, offset int) ([]SaleListItem, int, error) {
	if to.Before(from) {
		return nil, 0, errors.New("to must not be before from")
	}
	paymentFilter := ""
	switch strings.TrimSpace(paymentState) {
	case "", "all":
	case "paid":
		paymentFilter = " AND s.due_amount=0"
	case "due":
		paymentFilter = " AND s.due_amount>0"
	default:
		return nil, 0, errors.New("payment_state must be all, paid, or due")
	}
	customerRaw := ""
	if customerID != nil && *customerID != uuid.Nil {
		customerRaw = customerID.String()
	}
	like := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	query := `
		SELECT s.id,s.customer_id,COALESCE(c.name,''),s.gross_amount,s.discount_amount,s.total_amount,s.paid_amount,s.due_amount,s.status,s.created_at,
		       (SELECT COUNT(*)::int FROM sale_items si WHERE si.tenant_id=s.tenant_id AND si.sale_id=s.id),
		       COALESCE((SELECT SUM(si.qty)::float8 FROM sale_items si WHERE si.tenant_id=s.tenant_id AND si.sale_id=s.id),0),
		       (SELECT COUNT(*)::int FROM sale_items si WHERE si.tenant_id=s.tenant_id AND si.sale_id=s.id AND si.below_margin_guard),
		       (EXISTS(SELECT 1 FROM network_reservations nr WHERE nr.tenant_id=s.tenant_id AND nr.store_id=s.store_id AND nr.sale_id=s.id) OR EXISTS(SELECT 1 FROM network_procurements np WHERE np.seller_tenant_id=s.tenant_id AND np.seller_store_id=s.store_id AND np.seller_sale_id=s.id)),
		       COUNT(*) OVER()::int
		FROM sales s
		LEFT JOIN customers c ON c.id=s.customer_id AND c.tenant_id=s.tenant_id AND c.store_id=s.store_id
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.created_at >= $3 AND s.created_at < $4 + interval '1 day'
		  AND ($5='' OR s.customer_id=NULLIF($5,'')::uuid)
		  AND ($6='%%' OR lower(COALESCE(c.name,'')) LIKE $6 OR lower(s.id::text) LIKE $6)` + paymentFilter + `
		ORDER BY s.created_at DESC,s.id DESC LIMIT $7 OFFSET $8`
	rows, err := s.db.Query(ctx, query, tenantID, storeID, from, to, customerRaw, like, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []SaleListItem{}
	total := 0
	for rows.Next() {
		var x SaleListItem
		var created time.Time
		var count int
		if err := rows.Scan(&x.ID, &x.CustomerID, &x.CustomerName, &x.GrossAmount, &x.DiscountAmount, &x.TotalAmount, &x.PaidAmount, &x.DueAmount, &x.Status, &created, &x.LineCount, &x.TotalQty, &x.BelowMarginCount, &x.NetworkSource, &count); err != nil {
			return nil, 0, err
		}
		x.CreatedAt = created.Format(time.RFC3339)
		total = count
		items = append(items, x)
	}
	return items, total, rows.Err()
}

func (s *Service) ListPurchases(ctx context.Context, tenantID, storeID uuid.UUID, from, to time.Time, supplierID *uuid.UUID, q, paymentState string, limit, offset int) ([]PurchaseListItem, int, error) {
	if to.Before(from) {
		return nil, 0, errors.New("to must not be before from")
	}
	paymentFilter := ""
	switch strings.TrimSpace(paymentState) {
	case "", "all":
	case "paid":
		paymentFilter = " AND pch.due_amount=0"
	case "due":
		paymentFilter = " AND pch.due_amount>0"
	default:
		return nil, 0, errors.New("payment_state must be all, paid, or due")
	}
	supplierRaw := ""
	if supplierID != nil && *supplierID != uuid.Nil {
		supplierRaw = supplierID.String()
	}
	like := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	query := `
		SELECT pch.id,pch.supplier_id,sp.name,pch.total_amount,pch.paid_amount,pch.due_amount,pch.status,pch.created_at,
		       (SELECT COUNT(*)::int FROM purchase_items pi WHERE pi.tenant_id=pch.tenant_id AND pi.purchase_id=pch.id),
		       COALESCE((SELECT SUM(pi.qty)::float8 FROM purchase_items pi WHERE pi.tenant_id=pch.tenant_id AND pi.purchase_id=pch.id),0),
		       COUNT(*) OVER()::int
		FROM purchases pch
		JOIN suppliers sp ON sp.id=pch.supplier_id AND sp.tenant_id=pch.tenant_id AND sp.store_id=pch.store_id
		WHERE pch.tenant_id=$1 AND pch.store_id=$2 AND pch.created_at >= $3 AND pch.created_at < $4 + interval '1 day'
		  AND ($5='' OR pch.supplier_id=NULLIF($5,'')::uuid)
		  AND ($6='%%' OR lower(sp.name) LIKE $6 OR lower(pch.id::text) LIKE $6)` + paymentFilter + `
		ORDER BY pch.created_at DESC,pch.id DESC LIMIT $7 OFFSET $8`
	rows, err := s.db.Query(ctx, query, tenantID, storeID, from, to, supplierRaw, like, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []PurchaseListItem{}
	total := 0
	for rows.Next() {
		var x PurchaseListItem
		var created time.Time
		var count int
		if err := rows.Scan(&x.ID, &x.SupplierID, &x.SupplierName, &x.TotalAmount, &x.PaidAmount, &x.DueAmount, &x.Status, &created, &x.LineCount, &x.TotalQty, &count); err != nil {
			return nil, 0, err
		}
		x.CreatedAt = created.Format(time.RFC3339)
		total = count
		items = append(items, x)
	}
	return items, total, rows.Err()
}

func (s *Service) Dashboard(ctx context.Context, tenantID, storeID uuid.UUID, now time.Time) (DashboardSummary, error) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := day.AddDate(0, 0, 1)
	out := DashboardSummary{RecentSales: []SaleListItem{}, SalesLastSevenDays: []DailyAmount{}}
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(total_amount),0)::bigint FROM sales WHERE tenant_id=$1 AND store_id=$2 AND status='posted' AND created_at >= $3 AND created_at < $4`, tenantID, storeID, day, tomorrow).Scan(&out.SalesToday); err != nil {
		return out, err
	}
	var cogsToday, cogsReversed int64
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(si.qty*si.unit_cost),0)::bigint FROM sale_items si JOIN sales s ON s.id=si.sale_id AND s.tenant_id=si.tenant_id WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.status='posted' AND s.created_at >= $3 AND s.created_at < $4`, tenantID, storeID, day, tomorrow).Scan(&cogsToday); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(total_amount),0)::bigint,COALESCE(SUM(total_cost),0)::bigint FROM sales_returns WHERE tenant_id=$1 AND store_id=$2 AND created_at >= $3 AND created_at < $4`, tenantID, storeID, day, tomorrow).Scan(&out.SalesReturnsToday, &cogsReversed); err != nil {
		return out, err
	}
	out.NetSalesToday = out.SalesToday - out.SalesReturnsToday
	out.GrossProfitToday = out.NetSalesToday - (cogsToday - cogsReversed)
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(total_amount),0)::bigint FROM purchases WHERE tenant_id=$1 AND store_id=$2 AND status='posted' AND created_at >= $3 AND created_at < $4`, tenantID, storeID, day, tomorrow).Scan(&out.PurchasesToday); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(balance),0)::bigint FROM (SELECT SUM(debit-credit) AS balance FROM party_ledger_entries WHERE tenant_id=$1 AND store_id=$2 AND customer_id IS NOT NULL GROUP BY customer_id HAVING SUM(debit-credit)>0) x`, tenantID, storeID).Scan(&out.Receivables); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(balance),0)::bigint FROM (SELECT SUM(credit-debit) AS balance FROM party_ledger_entries WHERE tenant_id=$1 AND store_id=$2 AND supplier_id IS NOT NULL GROUP BY supplier_id HAVING SUM(credit-debit)>0) x`, tenantID, storeID).Scan(&out.Payables); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(ROUND(ib.on_hand*ib.avg_unit_cost)),0)::bigint FROM inventory_balances ib JOIN warehouses w ON w.id=ib.warehouse_id AND w.tenant_id=ib.tenant_id WHERE ib.tenant_id=$1 AND w.store_id=$2`, tenantID, storeID).Scan(&out.InventoryValue); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*)::int FROM network_reservations WHERE tenant_id=$1 AND store_id=$2 AND status IN ('pending','accepted','ready')`, tenantID, storeID).Scan(&out.OpenReservations); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*)::int FROM inventory_balances ib JOIN warehouses w ON w.id=ib.warehouse_id AND w.tenant_id=ib.tenant_id LEFT JOIN inventory_reorder_points rp ON rp.tenant_id=ib.tenant_id AND rp.warehouse_id=ib.warehouse_id AND rp.product_id=ib.product_id WHERE ib.tenant_id=$1 AND w.store_id=$2 AND COALESCE(rp.min_qty,0)>0 AND (ib.on_hand-ib.reserved)<=rp.min_qty`, tenantID, storeID).Scan(&out.LowStockCount); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT network_enabled FROM stores WHERE tenant_id=$1 AND id=$2`, tenantID, storeID).Scan(&out.NetworkEnabled); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*)::int FROM store_product_offers WHERE tenant_id=$1 AND store_id=$2 AND visible`, tenantID, storeID).Scan(&out.PublishedOffers); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*)::int FROM network_procurements WHERE buyer_tenant_id=$1 AND buyer_store_id=$2 AND status IN ('requested','accepted','ready')`, tenantID, storeID).Scan(&out.OpenBuyingProcurements); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*)::int FROM network_procurements WHERE seller_tenant_id=$1 AND seller_store_id=$2 AND status IN ('requested','accepted','ready')`, tenantID, storeID).Scan(&out.OpenSellingProcurements); err != nil {
		return out, err
	}
	networkStart := day.AddDate(0, 0, -29)
	if err := s.db.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*)::int FROM network_reservations WHERE tenant_id=$1 AND store_id=$2 AND created_at >= $3) +
		  (SELECT COUNT(*)::int FROM network_procurements WHERE seller_tenant_id=$1 AND seller_store_id=$2 AND created_at >= $3)`, tenantID, storeID, networkStart).Scan(&out.NetworkRequests30d); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(s.total_amount),0)::bigint,COUNT(*)::int
		FROM sales s
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.status='posted' AND s.created_at >= $3
		  AND (
		    EXISTS(SELECT 1 FROM network_reservations nr WHERE nr.tenant_id=s.tenant_id AND nr.store_id=s.store_id AND nr.sale_id=s.id)
		    OR EXISTS(SELECT 1 FROM network_procurements np WHERE np.seller_tenant_id=s.tenant_id AND np.seller_store_id=s.store_id AND np.seller_sale_id=s.id)
		  )`, tenantID, storeID, networkStart).Scan(&out.NetworkSales30d, &out.NetworkSalesCount30d); err != nil {
		return out, err
	}
	rows, err := s.db.Query(ctx, `SELECT s.id,s.customer_id,COALESCE(c.name,''),s.gross_amount,s.discount_amount,s.total_amount,s.paid_amount,s.due_amount,s.status,s.created_at,(SELECT COUNT(*)::int FROM sale_items si WHERE si.tenant_id=s.tenant_id AND si.sale_id=s.id),COALESCE((SELECT SUM(si.qty)::float8 FROM sale_items si WHERE si.tenant_id=s.tenant_id AND si.sale_id=s.id),0),(SELECT COUNT(*)::int FROM sale_items si WHERE si.tenant_id=s.tenant_id AND si.sale_id=s.id AND si.below_margin_guard),(EXISTS(SELECT 1 FROM network_reservations nr WHERE nr.sale_id=s.id AND nr.tenant_id=s.tenant_id) OR EXISTS(SELECT 1 FROM network_procurements np WHERE np.seller_sale_id=s.id AND np.seller_tenant_id=s.tenant_id)) FROM sales s LEFT JOIN customers c ON c.id=s.customer_id AND c.tenant_id=s.tenant_id WHERE s.tenant_id=$1 AND s.store_id=$2 ORDER BY s.created_at DESC LIMIT 5`, tenantID, storeID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var x SaleListItem
		var created time.Time
		if err := rows.Scan(&x.ID, &x.CustomerID, &x.CustomerName, &x.GrossAmount, &x.DiscountAmount, &x.TotalAmount, &x.PaidAmount, &x.DueAmount, &x.Status, &created, &x.LineCount, &x.TotalQty, &x.BelowMarginCount, &x.NetworkSource); err != nil {
			rows.Close()
			return out, err
		}
		x.CreatedAt = created.Format(time.RFC3339)
		out.RecentSales = append(out.RecentSales, x)
	}
	rows.Close()
	start := day.AddDate(0, 0, -6)
	dailyRows, err := s.db.Query(ctx, `SELECT created_at::date,COALESCE(SUM(total_amount),0)::bigint FROM sales WHERE tenant_id=$1 AND store_id=$2 AND status='posted' AND created_at >= $3 AND created_at < $4 GROUP BY created_at::date ORDER BY created_at::date`, tenantID, storeID, start, tomorrow)
	if err != nil {
		return out, err
	}
	daily := map[string]int64{}
	for dailyRows.Next() {
		var d time.Time
		var amount int64
		if err := dailyRows.Scan(&d, &amount); err != nil {
			dailyRows.Close()
			return out, err
		}
		daily[d.Format("2006-01-02")] = amount
	}
	dailyRows.Close()
	for i := 0; i < 7; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		out.SalesLastSevenDays = append(out.SalesLastSevenDays, DailyAmount{Date: d, Amount: daily[d]})
	}
	return out, nil
}

func (s *Service) InventoryInsights(ctx context.Context, tenantID, storeID, warehouseID uuid.UUID, q, sortBy string, limit, offset int) (InventoryInsightReport, error) {
	var warehouseOK bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, warehouseID, tenantID, storeID).Scan(&warehouseOK); err != nil {
		return InventoryInsightReport{}, err
	}
	if !warehouseOK {
		return InventoryInsightReport{}, errors.New("warehouse does not belong to authenticated store")
	}
	order := "lower(p.title),p.id"
	switch strings.TrimSpace(sortBy) {
	case "", "title":
	case "value":
		order = "inventory_value DESC,lower(p.title)"
	case "low_stock":
		order = "low_stock DESC,available ASC,lower(p.title)"
	case "sold_qty":
		order = "sold_qty_30d DESC,lower(p.title)"
	case "dead_stock":
		order = "dead_stock DESC,inventory_value DESC,lower(p.title)"
	default:
		return InventoryInsightReport{}, errors.New("sort must be title, value, low_stock, sold_qty, or dead_stock")
	}
	like := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	query := `
		SELECT p.id,p.title,COALESCE(p.sku,''),COALESCE(p.brand,''),ib.on_hand::float8,ib.reserved::float8,(ib.on_hand-ib.reserved)::float8,ib.avg_unit_cost,
		       ROUND(ib.on_hand*ib.avg_unit_cost)::bigint AS inventory_value,COALESCE(rp.min_qty,0)::float8,COALESCE(rp.target_qty,0)::float8,
		       (COALESCE(rp.min_qty,0)>0 AND (ib.on_hand-ib.reserved)<=rp.min_qty) AS low_stock,
		       COALESCE(sold.sold_qty_30d,0)::float8 AS sold_qty_30d,sold.last_sale_at,
		       CASE WHEN sold.last_sale_at IS NULL THEN NULL ELSE FLOOR(EXTRACT(EPOCH FROM (now()-sold.last_sale_at))/86400)::int END AS days_since_sale,
		       (ib.on_hand>0 AND (sold.last_sale_at IS NULL OR sold.last_sale_at < now()-interval '90 days')) AS dead_stock,
		       COUNT(*) OVER()::int
		FROM inventory_balances ib
		JOIN products p ON p.id=ib.product_id AND p.tenant_id=ib.tenant_id
		LEFT JOIN inventory_reorder_points rp ON rp.tenant_id=ib.tenant_id AND rp.warehouse_id=ib.warehouse_id AND rp.product_id=ib.product_id
		LEFT JOIN LATERAL (
		  SELECT COALESCE(SUM(si.qty) FILTER (WHERE s.created_at>=now()-interval '30 days'),0) AS sold_qty_30d,MAX(s.created_at) AS last_sale_at
		  FROM sale_items si JOIN sales s ON s.id=si.sale_id AND s.tenant_id=si.tenant_id
		  WHERE si.tenant_id=ib.tenant_id AND si.product_id=ib.product_id AND s.store_id=$3 AND s.status='posted'
		) sold ON true
		WHERE ib.tenant_id=$1 AND ib.warehouse_id=$2 AND p.active AND p.deleted_at IS NULL
		  AND ($4='%%' OR lower(p.title) LIKE $4 OR lower(COALESCE(p.sku,'')) LIKE $4 OR lower(COALESCE(p.brand,'')) LIKE $4 OR lower(COALESCE(p.oem_code,'')) LIKE $4)
		ORDER BY ` + order + ` LIMIT $5 OFFSET $6`
	rows, err := s.db.Query(ctx, query, tenantID, warehouseID, storeID, like, limit, offset)
	if err != nil {
		return InventoryInsightReport{}, err
	}
	defer rows.Close()
	out := InventoryInsightReport{Items: []InventoryInsightItem{}}
	for rows.Next() {
		var x InventoryInsightItem
		var lastSale *time.Time
		var total int
		if err := rows.Scan(&x.ProductID, &x.Title, &x.SKU, &x.Brand, &x.OnHand, &x.Reserved, &x.Available, &x.AvgUnitCost, &x.InventoryValue, &x.MinQty, &x.TargetQty, &x.LowStock, &x.SoldQty30d, &lastSale, &x.DaysSinceSale, &x.DeadStock, &total); err != nil {
			return InventoryInsightReport{}, err
		}
		if lastSale != nil {
			x.LastSaleAt = lastSale.Format(time.RFC3339)
		}
		out.Total = total
		out.Items = append(out.Items, x)
	}
	if err := rows.Err(); err != nil {
		return InventoryInsightReport{}, err
	}
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*)::int,COALESCE(SUM(ib.on_hand),0)::float8,COALESCE(SUM(ib.reserved),0)::float8,COALESCE(SUM(ROUND(ib.on_hand*ib.avg_unit_cost)),0)::bigint,
		       (COUNT(*) FILTER (WHERE COALESCE(rp.min_qty,0)>0 AND (ib.on_hand-ib.reserved)<=rp.min_qty))::int,
		       (COUNT(*) FILTER (WHERE ib.on_hand>0 AND (lastsale.last_sale_at IS NULL OR lastsale.last_sale_at<now()-interval '90 days')))::int
		FROM inventory_balances ib
		JOIN products p ON p.id=ib.product_id AND p.tenant_id=ib.tenant_id
		LEFT JOIN inventory_reorder_points rp ON rp.tenant_id=ib.tenant_id AND rp.warehouse_id=ib.warehouse_id AND rp.product_id=ib.product_id
		LEFT JOIN LATERAL (SELECT MAX(s.created_at) AS last_sale_at FROM sale_items si JOIN sales s ON s.id=si.sale_id AND s.tenant_id=si.tenant_id WHERE si.tenant_id=ib.tenant_id AND si.product_id=ib.product_id AND s.store_id=$3 AND s.status='posted') lastsale ON true
		WHERE ib.tenant_id=$1 AND ib.warehouse_id=$2 AND p.active AND p.deleted_at IS NULL`, tenantID, warehouseID, storeID).
		Scan(&out.Summary.SKUCount, &out.Summary.OnHand, &out.Summary.Reserved, &out.Summary.InventoryValue, &out.Summary.LowStockCount, &out.Summary.DeadStockCount); err != nil {
		return InventoryInsightReport{}, err
	}
	return out, nil
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func cashFlow(ctx context.Context, q queryRower, tenantID, storeID uuid.UUID, date time.Time) (CashReport, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.AddDate(0, 0, 1)
	out := CashReport{BusinessDate: start.Format("2006-01-02")}
	var saleCardIn, receiptCardIn, purchaseReturnCardIn int64
	var purchaseCardOut, supplierCardOut, expenseCardOut, saleReturnCardOut int64
	err := q.QueryRow(ctx, `
		SELECT
		 COALESCE((SELECT SUM(p.amount) FROM payments p JOIN sales s ON s.id=p.sale_id AND s.tenant_id=p.tenant_id WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.method='cash' AND p.created_at>=$3 AND p.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(st.amount) FROM settlements st WHERE st.tenant_id=$1 AND st.store_id=$2 AND st.party_type='customer' AND st.method='cash' AND st.created_at>=$3 AND st.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(pr.total_amount) FROM purchase_returns pr WHERE pr.tenant_id=$1 AND pr.store_id=$2 AND pr.refund_method='cash' AND pr.created_at>=$3 AND pr.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(p.amount) FROM payments p JOIN purchases pch ON pch.id=p.purchase_id AND pch.tenant_id=p.tenant_id WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.method='cash' AND p.created_at>=$3 AND p.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(st.amount) FROM settlements st WHERE st.tenant_id=$1 AND st.store_id=$2 AND st.party_type='supplier' AND st.method='cash' AND st.created_at>=$3 AND st.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(e.amount) FROM expenses e WHERE e.tenant_id=$1 AND e.store_id=$2 AND e.method='cash' AND e.occurred_on=$3::date),0)::bigint,
		 COALESCE((SELECT SUM(sr.total_amount) FROM sales_returns sr WHERE sr.tenant_id=$1 AND sr.store_id=$2 AND sr.refund_method='cash' AND sr.created_at>=$3 AND sr.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(p.amount) FROM payments p JOIN sales s ON s.id=p.sale_id AND s.tenant_id=p.tenant_id WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.method='card' AND p.created_at>=$3 AND p.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(st.amount) FROM settlements st WHERE st.tenant_id=$1 AND st.store_id=$2 AND st.party_type='customer' AND st.method='card' AND st.created_at>=$3 AND st.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(pr.total_amount) FROM purchase_returns pr WHERE pr.tenant_id=$1 AND pr.store_id=$2 AND pr.refund_method='card' AND pr.created_at>=$3 AND pr.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(p.amount) FROM payments p JOIN purchases pch ON pch.id=p.purchase_id AND pch.tenant_id=p.tenant_id WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.method='card' AND p.created_at>=$3 AND p.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(st.amount) FROM settlements st WHERE st.tenant_id=$1 AND st.store_id=$2 AND st.party_type='supplier' AND st.method='card' AND st.created_at>=$3 AND st.created_at<$4),0)::bigint,
		 COALESCE((SELECT SUM(e.amount) FROM expenses e WHERE e.tenant_id=$1 AND e.store_id=$2 AND e.method='card' AND e.occurred_on=$3::date),0)::bigint,
		 COALESCE((SELECT SUM(sr.total_amount) FROM sales_returns sr WHERE sr.tenant_id=$1 AND sr.store_id=$2 AND sr.refund_method='card' AND sr.created_at>=$3 AND sr.created_at<$4),0)::bigint`,
		tenantID, storeID, start, end).Scan(
		&out.SaleCashIn, &out.CustomerReceiptCashIn, &out.PurchaseReturnCashIn,
		&out.PurchaseCashOut, &out.SupplierPaymentCashOut, &out.ExpenseCashOut, &out.SaleReturnCashOut,
		&saleCardIn, &receiptCardIn, &purchaseReturnCardIn,
		&purchaseCardOut, &supplierCardOut, &expenseCardOut, &saleReturnCardOut,
	)
	if err != nil {
		return out, err
	}
	out.CashIn = out.SaleCashIn + out.CustomerReceiptCashIn + out.PurchaseReturnCashIn
	out.CashOut = out.PurchaseCashOut + out.SupplierPaymentCashOut + out.ExpenseCashOut + out.SaleReturnCashOut
	out.NetCashMovement = out.CashIn - out.CashOut
	out.CardIn = saleCardIn + receiptCardIn + purchaseReturnCardIn
	out.CardOut = purchaseCardOut + supplierCardOut + expenseCardOut + saleReturnCardOut
	out.NetCardMovement = out.CardIn - out.CardOut
	return out, nil
}

func (s *Service) CashReport(ctx context.Context, tenantID, storeID uuid.UUID, date time.Time) (CashReport, error) {
	out, err := cashFlow(ctx, s.db, tenantID, storeID, date)
	if err != nil {
		return out, err
	}
	var c DailyClosing
	var businessDate time.Time
	var created time.Time
	err = s.db.QueryRow(ctx, `SELECT id,business_date,opening_cash,cash_in,cash_out,expected_cash,actual_cash,variance,closed_by_user_id,COALESCE(note,''),created_at FROM daily_closings WHERE tenant_id=$1 AND store_id=$2 AND business_date=$3::date`, tenantID, storeID, date).Scan(&c.ID, &businessDate, &c.OpeningCash, &c.CashIn, &c.CashOut, &c.ExpectedCash, &c.ActualCash, &c.Variance, &c.ClosedByUserID, &c.Note, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	c.BusinessDate = businessDate.Format("2006-01-02")
	c.CreatedAt = created.Format(time.RFC3339)
	out.Closing = &c
	out.ChangedAfterClose = c.CashIn != out.CashIn || c.CashOut != out.CashOut
	return out, nil
}

func (s *Service) CloseDay(ctx context.Context, cmd CloseDayCommand) (DailyClosing, error) {
	if cmd.TenantID == uuid.Nil || cmd.StoreID == uuid.Nil || cmd.ActorUserID == uuid.Nil {
		return DailyClosing{}, errors.New("authenticated store and user are required")
	}
	if cmd.OpeningCash < 0 || cmd.ActualCash < 0 {
		return DailyClosing{}, errors.New("opening_cash and actual_cash cannot be negative")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return DailyClosing{}, errors.New("idempotency key is required")
	}
	date, err := time.Parse("2006-01-02", strings.TrimSpace(cmd.BusinessDate))
	if err != nil {
		return DailyClosing{}, errors.New("business_date must use YYYY-MM-DD")
	}
	today := time.Now()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	if date.After(today) {
		return DailyClosing{}, errors.New("future business dates cannot be closed")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return DailyClosing{}, err
	}
	defer tx.Rollback(ctx)
	var existing DailyClosing
	var existingDate time.Time
	var existingCreated time.Time
	err = tx.QueryRow(ctx, `SELECT id,business_date,opening_cash,cash_in,cash_out,expected_cash,actual_cash,variance,closed_by_user_id,COALESCE(note,''),created_at FROM daily_closings WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, cmd.TenantID, cmd.StoreID, cmd.IdempotencyKey).Scan(&existing.ID, &existingDate, &existing.OpeningCash, &existing.CashIn, &existing.CashOut, &existing.ExpectedCash, &existing.ActualCash, &existing.Variance, &existing.ClosedByUserID, &existing.Note, &existingCreated)
	if err == nil {
		existing.BusinessDate = existingDate.Format("2006-01-02")
		existing.CreatedAt = existingCreated.Format(time.RFC3339)
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return DailyClosing{}, err
	}
	var already bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM daily_closings WHERE tenant_id=$1 AND store_id=$2 AND business_date=$3::date)`, cmd.TenantID, cmd.StoreID, date).Scan(&already); err != nil {
		return DailyClosing{}, err
	}
	if already {
		return DailyClosing{}, errors.New("business day is already closed")
	}
	flow, err := cashFlow(ctx, tx, cmd.TenantID, cmd.StoreID, date)
	if err != nil {
		return DailyClosing{}, err
	}
	expected, variance := calculateClosing(cmd.OpeningCash, flow.CashIn, flow.CashOut, cmd.ActualCash)
	id := uuid.New()
	var created time.Time
	if err = tx.QueryRow(ctx, `INSERT INTO daily_closings(id,tenant_id,store_id,business_date,opening_cash,cash_in,cash_out,expected_cash,actual_cash,variance,closed_by_user_id,note,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,''),$13) RETURNING created_at`, id, cmd.TenantID, cmd.StoreID, date, cmd.OpeningCash, flow.CashIn, flow.CashOut, expected, cmd.ActualCash, variance, cmd.ActorUserID, strings.TrimSpace(cmd.Note), cmd.IdempotencyKey).Scan(&created); err != nil {
		return DailyClosing{}, err
	}
	payload, _ := json.Marshal(map[string]any{"closing_id": id, "store_id": cmd.StoreID, "business_date": date.Format("2006-01-02"), "variance": variance})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(tenant_id,aggregate_type,aggregate_id,event_type,payload) VALUES($1,'daily_closing',$2,'store.day.closed',$3)`, cmd.TenantID, id, payload); err != nil {
		return DailyClosing{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return DailyClosing{}, err
	}
	return DailyClosing{ID: id, BusinessDate: date.Format("2006-01-02"), OpeningCash: cmd.OpeningCash, CashIn: flow.CashIn, CashOut: flow.CashOut, ExpectedCash: expected, ActualCash: cmd.ActualCash, Variance: variance, ClosedByUserID: cmd.ActorUserID, Note: strings.TrimSpace(cmd.Note), CreatedAt: created.Format(time.RFC3339)}, nil
}

func calculateClosing(openingCash, cashIn, cashOut, actualCash int64) (int64, int64) {
	expected := openingCash + cashIn - cashOut
	return expected, actualCash - expected
}
