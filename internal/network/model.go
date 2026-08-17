package network

import (
	"time"

	"github.com/google/uuid"
)

type SearchResult struct {
	OfferID          uuid.UUID `json:"offer_id"`
	ProductID        uuid.UUID `json:"product_id"`
	Title            string    `json:"title"`
	SKU              string    `json:"sku,omitempty"`
	Brand            string    `json:"brand,omitempty"`
	OEMCode          string    `json:"oem_code,omitempty"`
	StoreID          uuid.UUID `json:"store_id"`
	StoreName        string    `json:"store_name"`
	City             string    `json:"city,omitempty"`
	Address          string    `json:"address,omitempty"`
	Phone            string    `json:"phone,omitempty"`
	SellingPrice     int64     `json:"selling_price"`
	Available        float64   `json:"available"`
	AllowReservation bool      `json:"allow_reservation"`
	AllowProcurement bool      `json:"allow_procurement"`
	LastUpdatedAt    time.Time `json:"last_updated_at"`
	Freshness        string    `json:"freshness"`
	DistanceKM       *float64  `json:"distance_km,omitempty"`
}

type StoreOffer struct {
	ProductID        uuid.UUID `json:"product_id"`
	Title            string    `json:"title"`
	SKU              string    `json:"sku,omitempty"`
	Brand            string    `json:"brand,omitempty"`
	OnHand           float64   `json:"on_hand"`
	Reserved         float64   `json:"reserved"`
	Available        float64   `json:"available"`
	SellingPrice     int64     `json:"selling_price"`
	Visible          bool      `json:"visible"`
	AllowReservation bool      `json:"allow_reservation"`
	AllowProcurement bool      `json:"allow_procurement"`
	LastVerifiedAt   time.Time `json:"last_verified_at"`
}

type UpdateOffer struct {
	WarehouseID      uuid.UUID `json:"warehouse_id"`
	SellingPrice     int64     `json:"selling_price"`
	Visible          bool      `json:"visible"`
	AllowReservation bool      `json:"allow_reservation"`
	AllowProcurement bool      `json:"allow_procurement"`
}

type StoreProfile struct {
	StoreID        uuid.UUID `json:"store_id"`
	StoreName      string    `json:"store_name"`
	NetworkEnabled bool      `json:"network_enabled"`
	Address        string    `json:"address,omitempty"`
	Phone          string    `json:"phone,omitempty"`
	City           string    `json:"city,omitempty"`
	Latitude       *float64  `json:"latitude,omitempty"`
	Longitude      *float64  `json:"longitude,omitempty"`
}

type UpdateStoreProfile struct {
	NetworkEnabled bool     `json:"network_enabled"`
	Address        string   `json:"address"`
	Phone          string   `json:"phone"`
	City           string   `json:"city"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
}
