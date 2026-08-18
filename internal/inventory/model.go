package inventory

import "github.com/google/uuid"

type Stock struct {
	ProductID    uuid.UUID `json:"product_id"`
	Title        string    `json:"title"`
	SKU          *string   `json:"sku,omitempty"`
	BaseUnitCode string    `json:"base_unit_code"`
	BaseUnitName string    `json:"base_unit_name"`
	OnHand       float64   `json:"on_hand"`
	Reserved     float64   `json:"reserved"`
	Available    float64   `json:"available"`
	AvgUnitCost  int64     `json:"avg_unit_cost"`
	MinQty       float64   `json:"min_qty"`
	TargetQty    float64   `json:"target_qty"`
	LowStock     bool      `json:"low_stock"`
}

type AdjustmentCommand struct {
	TenantID       uuid.UUID `json:"-"`
	StoreID        uuid.UUID `json:"-"`
	WarehouseID    uuid.UUID `json:"warehouse_id"`
	ProductID      uuid.UUID `json:"product_id"`
	QtyDelta       float64   `json:"qty_delta"`
	Reason         string    `json:"reason"`
	IdempotencyKey string    `json:"-"`
}
