package sales

import (
	"time"

	"github.com/google/uuid"
)

type CreateSaleItem struct {
	ProductID      uuid.UUID  `json:"product_id"`
	ProductUnitID  *uuid.UUID `json:"product_unit_id,omitempty"`
	Qty            float64    `json:"qty"`        // commercial quantity in the selected unit
	UnitPrice      int64      `json:"unit_price"` // price per selected commercial unit
	OverrideReason string     `json:"override_reason,omitempty"`
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
	InvoiceMode          string           `json:"invoice_mode,omitempty"`
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
	ID                   uuid.UUID `json:"id"`
	GrossAmount          int64     `json:"gross_amount"`
	DiscountAmount       int64     `json:"discount_amount"`
	NetAmount            int64     `json:"net_amount"`
	TaxableAmount        int64     `json:"taxable_amount"`
	ExemptAmount         int64     `json:"exempt_amount"`
	TaxAmount            int64     `json:"tax_amount"`
	TotalAmount          int64     `json:"total_amount"`
	PaidAmount           int64     `json:"paid_amount"`
	DueAmount            int64     `json:"due_amount"`
	InvoiceMode          string    `json:"invoice_mode"`
	InvoiceNumberDisplay string    `json:"invoice_number_display,omitempty"`
	Status               string    `json:"status"`
}

type SaleLine struct {
	ID                  uuid.UUID  `json:"id"`
	ProductID           uuid.UUID  `json:"product_id"`
	Title               string     `json:"title"`
	ProductUnitID       *uuid.UUID `json:"product_unit_id,omitempty"`
	UnitCode            string     `json:"unit_code"`
	UnitName            string     `json:"unit_name"`
	ConversionFactor    float64    `json:"conversion_factor"`
	Qty                 float64    `json:"qty"` // commercial quantity
	BaseQty             float64    `json:"base_qty"`
	ReturnedQty         float64    `json:"returned_qty"` // commercial quantity
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
	TaxCategory         string     `json:"tax_category"`
	TaxCode             *string    `json:"tax_code,omitempty"`
	TaxRateName         *string    `json:"tax_rate_name,omitempty"`
	TaxRateBPS          int        `json:"tax_rate_bps"`
	TaxBaseAmount       int64      `json:"tax_base_amount"`
	TaxAmount           int64      `json:"tax_amount"`
	TotalWithTax        int64      `json:"total_with_tax"`
	TaxExemptionReason  *string    `json:"tax_exemption_reason,omitempty"`
}

type SaleDetail struct {
	ID                   uuid.UUID  `json:"id"`
	CustomerID           *uuid.UUID `json:"customer_id,omitempty"`
	CustomerName         string     `json:"customer_name,omitempty"`
	WarehouseID          uuid.UUID  `json:"warehouse_id"`
	GrossAmount          int64      `json:"gross_amount"`
	DiscountAmount       int64      `json:"discount_amount"`
	NetAmount            int64      `json:"net_amount"`
	TaxableAmount        int64      `json:"taxable_amount"`
	ExemptAmount         int64      `json:"exempt_amount"`
	TaxAmount            int64      `json:"tax_amount"`
	TotalAmount          int64      `json:"total_amount"`
	PaidAmount           int64      `json:"paid_amount"`
	DueAmount            int64      `json:"due_amount"`
	InvoiceMode          string     `json:"invoice_mode"`
	InvoiceState         string     `json:"invoice_state"`
	InvoiceNumberDisplay string     `json:"invoice_number_display,omitempty"`
	Status               string     `json:"status"`
	CreatedAt            string     `json:"created_at"`
	Items                []SaleLine `json:"items"`
}
