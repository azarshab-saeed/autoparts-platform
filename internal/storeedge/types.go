package storeedge

import "time"

type Config struct {
	CloudURL       string   `json:"cloud_url"`
	DeviceID       string   `json:"device_id"`
	DeviceSecret   string   `json:"device_secret"`
	DeviceName     string   `json:"device_name"`
	StoreID        string   `json:"store_id"`
	StoreName      string   `json:"store_name"`
	WarehouseID    string   `json:"warehouse_id"`
	Listen         string   `json:"listen"`
	AllowedOrigins []string `json:"allowed_origins"`
	SyncSeconds    int      `json:"sync_seconds"`
}

type PricingPolicy struct {
	CashierMayOverride bool `json:"cashier_may_override"`
}

type PriceBreak struct {
	MinQty    float64 `json:"min_qty"`
	UnitPrice int64   `json:"unit_price"`
}

type Product struct {
	ProductID    string       `json:"product_id"`
	Title        string       `json:"title"`
	SKU          string       `json:"sku,omitempty"`
	Brand        string       `json:"brand,omitempty"`
	OEMCode      string       `json:"oem_code,omitempty"`
	Barcode      string       `json:"barcode,omitempty"`
	OnHand       float64      `json:"on_hand"`
	Reserved     float64      `json:"reserved"`
	Available    float64      `json:"available"`
	SellingPrice int64        `json:"selling_price"`
	PriceBreaks  []PriceBreak `json:"price_breaks,omitempty"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type Snapshot struct {
	GeneratedAt   time.Time      `json:"generated_at"`
	StoreID       string         `json:"store_id"`
	StoreName     string         `json:"store_name"`
	WarehouseID   string         `json:"warehouse_id"`
	PricingPolicy *PricingPolicy `json:"pricing_policy,omitempty"`
	Products      []Product      `json:"products"`
}

type LocalSaleItem struct {
	ProductID     string  `json:"product_id"`
	Title         string  `json:"title"`
	Qty           float64 `json:"qty"`
	UnitPrice     int64   `json:"unit_price"`
	ManualPrice   bool    `json:"manual_price,omitempty"`
	PreservePrice bool    `json:"preserve_price,omitempty"`
}

type LocalSale struct {
	LocalOperationID string          `json:"local_operation_id"`
	LocalNumber      string          `json:"local_number"`
	CreatedAt        time.Time       `json:"created_at"`
	PaymentMethod    string          `json:"payment_method"`
	CustomerID       string          `json:"customer_id,omitempty"`
	Items            []LocalSaleItem `json:"items"`
	TotalAmount      int64           `json:"total_amount"`
	Status           string          `json:"status"`
	ServerSaleID     string          `json:"server_sale_id,omitempty"`
	LastError        string          `json:"last_error,omitempty"`
	Attempts         int             `json:"attempts"`
	LastAttemptAt    *time.Time      `json:"last_attempt_at,omitempty"`
}

type State struct {
	Snapshot      Snapshot    `json:"snapshot"`
	Sales         []LocalSale `json:"sales"`
	Sequence      int         `json:"sequence"`
	LastSyncAt    *time.Time  `json:"last_sync_at,omitempty"`
	LastSyncError string      `json:"last_sync_error,omitempty"`
}

type PairResponse struct {
	DeviceID     string `json:"device_id"`
	DeviceSecret string `json:"device_secret"`
	StoreID      string `json:"store_id"`
	WarehouseID  string `json:"warehouse_id"`
	StoreName    string `json:"store_name"`
	WebOrigin    string `json:"web_origin,omitempty"`
}

type CloudSaleResponse struct {
	ID          string `json:"id"`
	TotalAmount int64  `json:"total_amount"`
	PaidAmount  int64  `json:"paid_amount"`
	DueAmount   int64  `json:"due_amount"`
	Status      string `json:"status"`
}
