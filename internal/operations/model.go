package operations

import "github.com/google/uuid"

type SaleListItem struct {
	ID            uuid.UUID  `json:"id"`
	CustomerID    *uuid.UUID `json:"customer_id,omitempty"`
	CustomerName  string     `json:"customer_name,omitempty"`
	TotalAmount   int64      `json:"total_amount"`
	PaidAmount    int64      `json:"paid_amount"`
	DueAmount     int64      `json:"due_amount"`
	Status        string     `json:"status"`
	CreatedAt     string     `json:"created_at"`
	LineCount     int        `json:"line_count"`
	TotalQty      float64    `json:"total_qty"`
	NetworkSource bool       `json:"network_source"`
}

type PurchaseListItem struct {
	ID           uuid.UUID `json:"id"`
	SupplierID   uuid.UUID `json:"supplier_id"`
	SupplierName string    `json:"supplier_name"`
	TotalAmount  int64     `json:"total_amount"`
	PaidAmount   int64     `json:"paid_amount"`
	DueAmount    int64     `json:"due_amount"`
	Status       string    `json:"status"`
	CreatedAt    string    `json:"created_at"`
	LineCount    int       `json:"line_count"`
	TotalQty     float64   `json:"total_qty"`
}

type DailyAmount struct {
	Date   string `json:"date"`
	Amount int64  `json:"amount"`
}

type DashboardSummary struct {
	SalesToday         int64          `json:"sales_today"`
	SalesReturnsToday  int64          `json:"sales_returns_today"`
	NetSalesToday      int64          `json:"net_sales_today"`
	GrossProfitToday   int64          `json:"gross_profit_today"`
	PurchasesToday     int64          `json:"purchases_today"`
	Receivables        int64          `json:"receivables"`
	Payables           int64          `json:"payables"`
	InventoryValue     int64          `json:"inventory_value"`
	OpenReservations   int            `json:"open_reservations"`
	LowStockCount      int            `json:"low_stock_count"`
	RecentSales        []SaleListItem `json:"recent_sales"`
	SalesLastSevenDays []DailyAmount  `json:"sales_last_seven_days"`
}

type InventoryInsightItem struct {
	ProductID      uuid.UUID `json:"product_id"`
	Title          string    `json:"title"`
	SKU            string    `json:"sku,omitempty"`
	Brand          string    `json:"brand,omitempty"`
	OnHand         float64   `json:"on_hand"`
	Reserved       float64   `json:"reserved"`
	Available      float64   `json:"available"`
	AvgUnitCost    int64     `json:"avg_unit_cost"`
	InventoryValue int64     `json:"inventory_value"`
	MinQty         float64   `json:"min_qty"`
	TargetQty      float64   `json:"target_qty"`
	LowStock       bool      `json:"low_stock"`
	SoldQty30d     float64   `json:"sold_qty_30d"`
	LastSaleAt     string    `json:"last_sale_at,omitempty"`
	DaysSinceSale  *int      `json:"days_since_sale,omitempty"`
	DeadStock      bool      `json:"dead_stock"`
}

type InventoryInsightSummary struct {
	SKUCount       int     `json:"sku_count"`
	OnHand         float64 `json:"on_hand"`
	Reserved       float64 `json:"reserved"`
	InventoryValue int64   `json:"inventory_value"`
	LowStockCount  int     `json:"low_stock_count"`
	DeadStockCount int     `json:"dead_stock_count"`
}

type InventoryInsightReport struct {
	Summary InventoryInsightSummary `json:"summary"`
	Items   []InventoryInsightItem  `json:"items"`
	Total   int                     `json:"total"`
}

type DailyClosing struct {
	ID             uuid.UUID `json:"id"`
	BusinessDate   string    `json:"business_date"`
	OpeningCash    int64     `json:"opening_cash"`
	CashIn         int64     `json:"cash_in"`
	CashOut        int64     `json:"cash_out"`
	ExpectedCash   int64     `json:"expected_cash"`
	ActualCash     int64     `json:"actual_cash"`
	Variance       int64     `json:"variance"`
	ClosedByUserID uuid.UUID `json:"closed_by_user_id"`
	Note           string    `json:"note,omitempty"`
	CreatedAt      string    `json:"created_at"`
}

type CashReport struct {
	BusinessDate           string        `json:"business_date"`
	SaleCashIn             int64         `json:"sale_cash_in"`
	CustomerReceiptCashIn  int64         `json:"customer_receipt_cash_in"`
	PurchaseReturnCashIn   int64         `json:"purchase_return_cash_in"`
	CashIn                 int64         `json:"cash_in"`
	PurchaseCashOut        int64         `json:"purchase_cash_out"`
	SupplierPaymentCashOut int64         `json:"supplier_payment_cash_out"`
	ExpenseCashOut         int64         `json:"expense_cash_out"`
	SaleReturnCashOut      int64         `json:"sale_return_cash_out"`
	CashOut                int64         `json:"cash_out"`
	NetCashMovement        int64         `json:"net_cash_movement"`
	CardIn                 int64         `json:"card_in"`
	CardOut                int64         `json:"card_out"`
	NetCardMovement        int64         `json:"net_card_movement"`
	Closing                *DailyClosing `json:"closing,omitempty"`
	ChangedAfterClose      bool          `json:"changed_after_close"`
}

type CloseDayCommand struct {
	TenantID       uuid.UUID `json:"-"`
	StoreID        uuid.UUID `json:"-"`
	ActorUserID    uuid.UUID `json:"-"`
	BusinessDate   string    `json:"business_date"`
	OpeningCash    int64     `json:"opening_cash"`
	ActualCash     int64     `json:"actual_cash"`
	Note           string    `json:"note,omitempty"`
	IdempotencyKey string    `json:"-"`
}
