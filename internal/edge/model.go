package edge

import (
	"time"

	"github.com/google/uuid"
)

type Pairing struct {
	Code      string    `json:"pair_code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Device struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"-"`
	StoreID     uuid.UUID  `json:"store_id"`
	WarehouseID uuid.UUID  `json:"warehouse_id"`
	Name        string     `json:"name"`
	Active      bool       `json:"active"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type PairResult struct {
	DeviceID     uuid.UUID `json:"device_id"`
	DeviceSecret string    `json:"device_secret"`
	StoreID      uuid.UUID `json:"store_id"`
	WarehouseID  uuid.UUID `json:"warehouse_id"`
	StoreName    string    `json:"store_name"`
}

type SnapshotPriceBreak struct {
	MinQty    float64 `json:"min_qty"`
	UnitPrice int64   `json:"unit_price"`
}

type SnapshotProductUnit struct {
	ProductUnitID uuid.UUID            `json:"product_unit_id"`
	Code          string               `json:"code"`
	Name          string               `json:"name"`
	FactorToBase  float64              `json:"factor_to_base"`
	Barcode       string               `json:"barcode,omitempty"`
	IsBase        bool                 `json:"is_base"`
	PriceBreaks   []SnapshotPriceBreak `json:"price_breaks,omitempty"`
}

type SnapshotPricingPolicy struct {
	CashierMayOverride bool `json:"cashier_may_override"`
}

type SnapshotProduct struct {
	ProductID              uuid.UUID             `json:"product_id"`
	Title                  string                `json:"title"`
	SKU                    string                `json:"sku,omitempty"`
	Brand                  string                `json:"brand,omitempty"`
	OEMCode                string                `json:"oem_code,omitempty"`
	Barcode                string                `json:"barcode,omitempty"`
	OnHand                 float64               `json:"on_hand"`
	Reserved               float64               `json:"reserved"`
	Available              float64               `json:"available"`
	SellingPrice           int64                 `json:"selling_price"`
	AllowFractionalBaseQty bool                  `json:"allow_fractional_base_qty"`
	PriceBreaks            []SnapshotPriceBreak  `json:"price_breaks,omitempty"` // base-unit compatibility
	Units                  []SnapshotProductUnit `json:"units,omitempty"`
	UpdatedAt              time.Time             `json:"updated_at"`
}

type Snapshot struct {
	GeneratedAt   time.Time              `json:"generated_at"`
	StoreID       uuid.UUID              `json:"store_id"`
	StoreName     string                 `json:"store_name"`
	WarehouseID   uuid.UUID              `json:"warehouse_id"`
	PricingPolicy *SnapshotPricingPolicy `json:"pricing_policy,omitempty"`
	Products      []SnapshotProduct      `json:"products"`
}
