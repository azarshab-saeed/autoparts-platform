package reservations

import (
	"time"

	"github.com/google/uuid"
)

type Reservation struct {
	ID           uuid.UUID  `json:"id"`
	OfferID      uuid.UUID  `json:"offer_id"`
	ProductID    uuid.UUID  `json:"product_id"`
	ProductTitle string     `json:"product_title"`
	StoreID      uuid.UUID  `json:"store_id"`
	StoreName    string     `json:"store_name"`
	Address      string     `json:"address,omitempty"`
	Phone        string     `json:"phone,omitempty"`
	BuyerUserID  uuid.UUID  `json:"buyer_user_id,omitempty"`
	BuyerName    string     `json:"buyer_name,omitempty"`
	BuyerEmail   string     `json:"buyer_email,omitempty"`
	BuyerRole    string     `json:"buyer_role,omitempty"`
	SaleID       *uuid.UUID `json:"sale_id,omitempty"`
	PaidAmount   int64      `json:"paid_amount"`
	DueAmount    int64      `json:"due_amount"`
	Qty          float64    `json:"qty"`
	UnitPrice    int64      `json:"unit_price"`
	TotalAmount  int64      `json:"total_amount"`
	Status       string     `json:"status"`
	ExpiresAt    time.Time  `json:"expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type CreateCommand struct {
	OfferID        uuid.UUID
	BuyerUserID    uuid.UUID
	BuyerName      string
	BuyerEmail     string
	BuyerRole      string
	Qty            float64
	IdempotencyKey string
}
