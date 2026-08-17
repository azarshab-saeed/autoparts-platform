package procurement

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusRequested Status = "requested"
	StatusAccepted  Status = "accepted"
	StatusReady     Status = "ready"
	StatusReceived  Status = "received"
	StatusRejected  Status = "rejected"
	StatusCancelled Status = "cancelled"
	StatusExpired   Status = "expired"
)

type Order struct {
	ID                 uuid.UUID  `json:"id"`
	BuyerStoreID       uuid.UUID  `json:"buyer_store_id"`
	BuyerStoreName     string     `json:"buyer_store_name"`
	BuyerWarehouseID   uuid.UUID  `json:"buyer_warehouse_id"`
	BuyerProductID     uuid.UUID  `json:"buyer_product_id"`
	BuyerProductTitle  string     `json:"buyer_product_title"`
	SellerStoreID      uuid.UUID  `json:"seller_store_id"`
	SellerStoreName    string     `json:"seller_store_name"`
	SellerWarehouseID  uuid.UUID  `json:"seller_warehouse_id"`
	SellerProductID    uuid.UUID  `json:"seller_product_id"`
	SellerProductTitle string     `json:"seller_product_title"`
	OfferID            uuid.UUID  `json:"offer_id"`
	Qty                float64    `json:"qty"`
	UnitPrice          int64      `json:"unit_price"`
	TotalAmount        int64      `json:"total_amount"`
	Status             Status     `json:"status"`
	ExpiresAt          time.Time  `json:"expires_at"`
	SellerSaleID       *uuid.UUID `json:"seller_sale_id,omitempty"`
	BuyerPurchaseID    *uuid.UUID `json:"buyer_purchase_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type CreateCommand struct {
	BuyerTenantID    uuid.UUID
	BuyerStoreID     uuid.UUID
	BuyerWarehouseID uuid.UUID
	BuyerProductID   uuid.UUID
	ActorUserID      uuid.UUID
	OfferID          uuid.UUID
	Qty              float64
	IdempotencyKey   string
}

type ReceiveCommand struct {
	BuyerTenantID  uuid.UUID
	BuyerStoreID   uuid.UUID
	ActorUserID    uuid.UUID
	ProcurementID  uuid.UUID
	IdempotencyKey string
}

type ReceiveResult struct {
	ProcurementID   uuid.UUID `json:"procurement_id"`
	SellerSaleID    uuid.UUID `json:"seller_sale_id"`
	BuyerPurchaseID uuid.UUID `json:"buyer_purchase_id"`
	TotalAmount     int64     `json:"total_amount"`
	Status          Status    `json:"status"`
}
