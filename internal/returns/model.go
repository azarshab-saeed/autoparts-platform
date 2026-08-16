package returns

import "github.com/google/uuid"

type ReturnItem struct {
	SourceItemID uuid.UUID `json:"source_item_id"`
	Qty          float64   `json:"qty"`
}

type SaleReturnCommand struct {
	TenantID       uuid.UUID    `json:"-"`
	StoreID        uuid.UUID    `json:"-"`
	SaleID         uuid.UUID    `json:"sale_id"`
	RefundMethod   string       `json:"refund_method"`
	Items          []ReturnItem `json:"items"`
	IdempotencyKey string       `json:"-"`
}

type PurchaseReturnCommand struct {
	TenantID       uuid.UUID    `json:"-"`
	StoreID        uuid.UUID    `json:"-"`
	PurchaseID     uuid.UUID    `json:"purchase_id"`
	RefundMethod   string       `json:"refund_method"`
	Items          []ReturnItem `json:"items"`
	IdempotencyKey string       `json:"-"`
}

type Result struct {
	ID          uuid.UUID `json:"id"`
	TotalAmount int64     `json:"total_amount"`
	Status      string    `json:"status"`
}
