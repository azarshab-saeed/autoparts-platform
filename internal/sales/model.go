package sales

import (
	"time"

	"github.com/google/uuid"
)

type CreateSaleItem struct {
	ProductID      uuid.UUID `json:"product_id"`
	Qty            float64   `json:"qty"`
	UnitPrice      int64     `json:"unit_price"`
	OverrideReason string    `json:"override_reason,omitempty"`
}

type PaymentPart struct {
	Method string `json:"method"`
	Amount int64  `json:"amount"`
}

type CreateSaleCommand struct {
	TenantID             uuid.UUID        `json:"-"`
	StoreID              uuid.UUID        `json:"-"`
	ActorUserID          uuid.UUID        `json:"-"`
	ActorRole            string           `json:"-"`
	WarehouseID          uuid.UUID        `json:"warehouse_id"`
	CustomerID           *uuid.UUID       `json:"customer_id,omitempty"`
	PaymentMethod        string           `json:"payment_method,omitempty"` // legacy single-method input
	Payments             []PaymentPart    `json:"payments,omitempty"`
	IdempotencyKey       string           `json:"-"`
	Source               string           `json:"-"`
	EdgeDeviceID         *uuid.UUID       `json:"-"`
	EdgeLocalOperationID string           `json:"-"`
	EdgeOccurredAt       *time.Time       `json:"-"`
	Items                []CreateSaleItem `json:"items"`
}

type Sale struct {
	ID             uuid.UUID `json:"id"`
	GrossAmount    int64     `json:"gross_amount"`
	DiscountAmount int64     `json:"discount_amount"`
	TotalAmount    int64     `json:"total_amount"`
	PaidAmount     int64     `json:"paid_amount"`
	DueAmount      int64     `json:"due_amount"`
	Status         string    `json:"status"`
}

type SaleLine struct {
	ID                  uuid.UUID  `json:"id"`
	ProductID           uuid.UUID  `json:"product_id"`
	Title               string     `json:"title"`
	Qty                 float64    `json:"qty"`
	ReturnedQty         float64    `json:"returned_qty"`
	ReturnableQty       float64    `json:"returnable_qty"`
	UnitPrice           int64      `json:"unit_price"`
	UnitCost            int64      `json:"unit_cost"`
	LineTotal           int64      `json:"line_total"`
	GrossLineTotal      int64      `json:"gross_line_total"`
	DiscountAmount      int64      `json:"discount_amount"`
	PriceListID         *uuid.UUID `json:"price_list_id,omitempty"`
	ListUnitPrice       *int64     `json:"list_unit_price,omitempty"`
	PriceSource         *string    `json:"price_source,omitempty"`
	PriceOverride       bool       `json:"price_override"`
	OverrideReason      *string    `json:"override_reason,omitempty"`
	OverrideActorUserID *uuid.UUID `json:"override_actor_user_id,omitempty"`
	MarginBPS           *int       `json:"margin_bps,omitempty"`
	MarginGuardBPS      *int       `json:"margin_guard_bps,omitempty"`
	BelowMarginGuard    bool       `json:"below_margin_guard"`
}

type SaleDetail struct {
	ID             uuid.UUID  `json:"id"`
	CustomerID     *uuid.UUID `json:"customer_id,omitempty"`
	CustomerName   string     `json:"customer_name,omitempty"`
	WarehouseID    uuid.UUID  `json:"warehouse_id"`
	GrossAmount    int64      `json:"gross_amount"`
	DiscountAmount int64      `json:"discount_amount"`
	TotalAmount    int64      `json:"total_amount"`
	PaidAmount     int64      `json:"paid_amount"`
	DueAmount      int64      `json:"due_amount"`
	Status         string     `json:"status"`
	CreatedAt      string     `json:"created_at"`
	Items          []SaleLine `json:"items"`
}
