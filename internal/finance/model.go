package finance

import "github.com/google/uuid"

type SettlementCommand struct {
	TenantID       uuid.UUID  `json:"-"`
	StoreID        uuid.UUID  `json:"-"`
	PartyType      string     `json:"-"`
	CustomerID     *uuid.UUID `json:"customer_id,omitempty"`
	SupplierID     *uuid.UUID `json:"supplier_id,omitempty"`
	Method         string     `json:"method"`
	Amount         int64      `json:"amount"`
	Note           string     `json:"note,omitempty"`
	IdempotencyKey string     `json:"-"`
}

type Settlement struct {
	ID        uuid.UUID `json:"id"`
	PartyType string    `json:"party_type"`
	Method    string    `json:"method"`
	Amount    int64     `json:"amount"`
	Balance   int64     `json:"balance"`
	Status    string    `json:"status"`
}

type PartyBalance struct {
	ID      uuid.UUID `json:"id"`
	Code    string    `json:"code,omitempty"`
	Name    string    `json:"name"`
	Phone   string    `json:"phone,omitempty"`
	Balance int64     `json:"balance"`
}
