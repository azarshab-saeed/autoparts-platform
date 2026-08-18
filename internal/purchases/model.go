package purchases

import "github.com/google/uuid"

type CreateItem struct {
	ProductID     uuid.UUID  `json:"product_id"`
	ProductUnitID *uuid.UUID `json:"product_unit_id,omitempty"`
	Qty           float64    `json:"qty"`       // commercial quantity
	UnitCost      int64      `json:"unit_cost"` // cost per selected commercial unit
}

type PaymentPart struct {
	Method string `json:"method"`
	Amount int64  `json:"amount"`
}

type CreateCommand struct {
	TenantID       uuid.UUID     `json:"-"`
	StoreID        uuid.UUID     `json:"-"`
	WarehouseID    uuid.UUID     `json:"warehouse_id"`
	SupplierID     uuid.UUID     `json:"supplier_id"`
	PaymentMethod  string        `json:"payment_method,omitempty"`
	Payments       []PaymentPart `json:"payments,omitempty"`
	IdempotencyKey string        `json:"-"`
	Items          []CreateItem  `json:"items"`
}

type Purchase struct {
	ID          uuid.UUID `json:"id"`
	TotalAmount int64     `json:"total_amount"`
	PaidAmount  int64     `json:"paid_amount"`
	DueAmount   int64     `json:"due_amount"`
	Status      string    `json:"status"`
}

type PurchaseLine struct {
	ID               uuid.UUID  `json:"id"`
	ProductID        uuid.UUID  `json:"product_id"`
	Title            string     `json:"title"`
	ProductUnitID    *uuid.UUID `json:"product_unit_id,omitempty"`
	UnitCode         string     `json:"unit_code"`
	UnitName         string     `json:"unit_name"`
	ConversionFactor float64    `json:"conversion_factor"`
	Qty              float64    `json:"qty"`
	BaseQty          float64    `json:"base_qty"`
	ReturnedQty      float64    `json:"returned_qty"`
	ReturnableQty    float64    `json:"returnable_qty"`
	UnitCost         int64      `json:"unit_cost"`
	LineTotal        int64      `json:"line_total"`
}

type PurchaseDetail struct {
	ID           uuid.UUID      `json:"id"`
	SupplierID   uuid.UUID      `json:"supplier_id"`
	SupplierName string         `json:"supplier_name"`
	WarehouseID  uuid.UUID      `json:"warehouse_id"`
	TotalAmount  int64          `json:"total_amount"`
	PaidAmount   int64          `json:"paid_amount"`
	DueAmount    int64          `json:"due_amount"`
	Status       string         `json:"status"`
	CreatedAt    string         `json:"created_at"`
	Items        []PurchaseLine `json:"items"`
}
