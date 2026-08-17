package sales

import (
	"time"

	"github.com/google/uuid"
)

type CreateSaleItem struct {
	ProductID uuid.UUID `json:"product_id"`
	Qty       float64   `json:"qty"`
	UnitPrice int64     `json:"unit_price"`
}

type PaymentPart struct {
	Method string `json:"method"`
	Amount int64  `json:"amount"`
}

type CreateSaleCommand struct {
	TenantID             uuid.UUID        `json:"-"`
	StoreID              uuid.UUID        `json:"-"`
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
	ID          uuid.UUID `json:"id"`
	TotalAmount int64     `json:"total_amount"`
	PaidAmount  int64     `json:"paid_amount"`
	DueAmount   int64     `json:"due_amount"`
	Status      string    `json:"status"`
}

type SaleLine struct {
	ID            uuid.UUID `json:"id"`
	ProductID     uuid.UUID `json:"product_id"`
	Title         string    `json:"title"`
	Qty           float64   `json:"qty"`
	ReturnedQty   float64   `json:"returned_qty"`
	ReturnableQty float64   `json:"returnable_qty"`
	UnitPrice     int64     `json:"unit_price"`
	UnitCost      int64     `json:"unit_cost"`
	LineTotal     int64     `json:"line_total"`
}

type SaleDetail struct {
	ID           uuid.UUID  `json:"id"`
	CustomerID   *uuid.UUID `json:"customer_id,omitempty"`
	CustomerName string     `json:"customer_name,omitempty"`
	WarehouseID  uuid.UUID  `json:"warehouse_id"`
	TotalAmount  int64      `json:"total_amount"`
	PaidAmount   int64      `json:"paid_amount"`
	DueAmount    int64      `json:"due_amount"`
	Status       string     `json:"status"`
	CreatedAt    string     `json:"created_at"`
	Items        []SaleLine `json:"items"`
}
