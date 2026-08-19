package tax

import (
	"time"

	"github.com/google/uuid"
)

type TenantProfile struct {
	LegalName          string `json:"legal_name"`
	NationalID         string `json:"national_id"`
	EconomicCode       string `json:"economic_code"`
	RegistrationNumber string `json:"registration_number"`
	PostalCode         string `json:"postal_code"`
	Province           string `json:"province"`
	City               string `json:"city"`
	Address            string `json:"address"`
	Phone              string `json:"phone"`
}

type Settings struct {
	TenantProfile
	TaxEnabled         bool   `json:"tax_enabled"`
	TaxOnNormalSales   bool   `json:"tax_on_normal_sales"`
	CalculationMode    string `json:"calculation_mode"`
	DefaultInvoiceMode string `json:"default_invoice_mode"`
	DefaultTaxCode     string `json:"default_tax_code"`
	OfficialSeries     string `json:"official_series"`
	NextOfficialNumber int64  `json:"next_official_number"`
	InvoiceNumberWidth int    `json:"invoice_number_width"`
}

type Rate struct {
	ID              uuid.UUID `json:"id"`
	Code            string    `json:"code"`
	Name            string    `json:"name"`
	Category        string    `json:"category"`
	RateBPS         int       `json:"rate_bps"`
	EffectiveFrom   string    `json:"effective_from"`
	EffectiveTo     string    `json:"effective_to,omitempty"`
	ExemptionReason string    `json:"exemption_reason,omitempty"`
	Active          bool      `json:"active"`
}

type UpsertRate struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	RateBPS         int    `json:"rate_bps"`
	EffectiveFrom   string `json:"effective_from"`
	EffectiveTo     string `json:"effective_to,omitempty"`
	ExemptionReason string `json:"exemption_reason,omitempty"`
	Active          bool   `json:"active"`
}

type CustomerIdentity struct {
	CustomerID         uuid.UUID `json:"customer_id"`
	Name               string    `json:"name"`
	LegalType          string    `json:"legal_type"`
	NationalID         string    `json:"national_id"`
	EconomicCode       string    `json:"economic_code"`
	RegistrationNumber string    `json:"registration_number"`
	PostalCode         string    `json:"postal_code"`
	Address            string    `json:"address"`
}

type ProductTaxRow struct {
	ProductID        uuid.UUID `json:"product_id"`
	Title            string    `json:"title"`
	SKU              string    `json:"sku,omitempty"`
	ExplicitTaxCode  string    `json:"explicit_tax_code,omitempty"`
	EffectiveTaxCode string    `json:"effective_tax_code,omitempty"`
	RateName         string    `json:"rate_name,omitempty"`
	Category         string    `json:"category,omitempty"`
	RateBPS          int       `json:"rate_bps"`
}

type ProductTaxProfile struct {
	ProductID uuid.UUID `json:"product_id"`
	TaxCode   string    `json:"tax_code"`
}

type QuoteLineInput struct {
	ProductID uuid.UUID `json:"product_id"`
	Amount    int64     `json:"amount"`
}

type QuoteLine struct {
	ProductID       uuid.UUID `json:"product_id"`
	Category        string    `json:"category"`
	TaxCode         string    `json:"tax_code,omitempty"`
	TaxRateName     string    `json:"tax_rate_name,omitempty"`
	TaxRateBPS      int       `json:"tax_rate_bps"`
	TaxBaseAmount   int64     `json:"tax_base_amount"`
	TaxAmount       int64     `json:"tax_amount"`
	TotalWithTax    int64     `json:"total_with_tax"`
	ExemptionReason string    `json:"exemption_reason,omitempty"`
}

type Quote struct {
	InvoiceMode     string         `json:"invoice_mode"`
	CalculationMode string         `json:"calculation_mode"`
	Applied         bool           `json:"applied"`
	NetAmount       int64          `json:"net_amount"`
	TaxableAmount   int64          `json:"taxable_amount"`
	ExemptAmount    int64          `json:"exempt_amount"`
	TaxAmount       int64          `json:"tax_amount"`
	TotalAmount     int64          `json:"total_amount"`
	SellerReady     bool           `json:"seller_ready"`
	BuyerReady      bool           `json:"buyer_ready"`
	Warnings        []string       `json:"warnings"`
	Items           []QuoteLine    `json:"items"`
	SellerSnapshot  map[string]any `json:"-"`
	BuyerSnapshot   map[string]any `json:"-"`
}

type InvoiceListItem struct {
	SaleID               uuid.UUID  `json:"sale_id"`
	InvoiceMode          string     `json:"invoice_mode"`
	InvoiceState         string     `json:"invoice_state"`
	InvoiceNumberDisplay string     `json:"invoice_number_display,omitempty"`
	CustomerID           *uuid.UUID `json:"customer_id,omitempty"`
	CustomerName         string     `json:"customer_name,omitempty"`
	NetAmount            int64      `json:"net_amount"`
	TaxAmount            int64      `json:"tax_amount"`
	TotalAmount          int64      `json:"total_amount"`
	CreatedAt            time.Time  `json:"created_at"`
}

type InvoiceAction struct {
	ID                uuid.UUID  `json:"id"`
	SaleID            uuid.UUID  `json:"sale_id"`
	ActionType        string     `json:"action_type"`
	Reason            string     `json:"reason"`
	Status            string     `json:"status"`
	ActorUserID       *uuid.UUID `json:"actor_user_id,omitempty"`
	ReplacementSaleID *uuid.UUID `json:"replacement_sale_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

type PrintLine struct {
	ProductID       uuid.UUID `json:"product_id"`
	Title           string    `json:"title"`
	UnitName        string    `json:"unit_name"`
	Qty             float64   `json:"qty"`
	UnitPrice       int64     `json:"unit_price"`
	NetAmount       int64     `json:"net_amount"`
	TaxCategory     string    `json:"tax_category"`
	TaxCode         string    `json:"tax_code,omitempty"`
	TaxRateName     string    `json:"tax_rate_name,omitempty"`
	TaxRateBPS      int       `json:"tax_rate_bps"`
	TaxAmount       int64     `json:"tax_amount"`
	TotalWithTax    int64     `json:"total_with_tax"`
	ExemptionReason string    `json:"exemption_reason,omitempty"`
}

type PrintData struct {
	SaleID               uuid.UUID      `json:"sale_id"`
	InvoiceMode          string         `json:"invoice_mode"`
	InvoiceKind          string         `json:"invoice_kind"`
	InvoiceState         string         `json:"invoice_state"`
	InvoiceNumberDisplay string         `json:"invoice_number_display,omitempty"`
	IssuedAt             *time.Time     `json:"issued_at,omitempty"`
	Seller               map[string]any `json:"seller"`
	Buyer                map[string]any `json:"buyer"`
	CalculationMode      string         `json:"calculation_mode"`
	GrossAmount          int64          `json:"gross_amount"`
	DiscountAmount       int64          `json:"discount_amount"`
	NetAmount            int64          `json:"net_amount"`
	TaxableAmount        int64          `json:"taxable_amount"`
	ExemptAmount         int64          `json:"exempt_amount"`
	TaxAmount            int64          `json:"tax_amount"`
	TotalAmount          int64          `json:"total_amount"`
	PaidAmount           int64          `json:"paid_amount"`
	DueAmount            int64          `json:"due_amount"`
	Items                []PrintLine    `json:"items"`
}
