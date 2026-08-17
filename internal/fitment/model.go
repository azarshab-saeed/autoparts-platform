package fitment

import "github.com/google/uuid"

type VehicleVariant struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	EngineCode string    `json:"engine_code,omitempty"`
	YearFrom   *int      `json:"year_from,omitempty"`
	YearTo     *int      `json:"year_to,omitempty"`
}

type VehicleModel struct {
	ID       uuid.UUID        `json:"id"`
	Name     string           `json:"name"`
	Variants []VehicleVariant `json:"variants"`
}

type VehicleMake struct {
	ID     uuid.UUID      `json:"id"`
	Name   string         `json:"name"`
	Models []VehicleModel `json:"models"`
}

type SearchTerm struct {
	Kind string `json:"kind"`
	Term string `json:"term"`
}

type ProductFitment struct {
	VehicleVariantID uuid.UUID `json:"vehicle_variant_id"`
	MakeName         string    `json:"make_name"`
	ModelName        string    `json:"model_name"`
	VariantName      string    `json:"variant_name"`
	EngineCode       string    `json:"engine_code,omitempty"`
	YearFrom         *int      `json:"year_from,omitempty"`
	YearTo           *int      `json:"year_to,omitempty"`
	Notes            string    `json:"notes,omitempty"`
}

type ProductMetadata struct {
	ProductID uuid.UUID        `json:"product_id"`
	Terms     []SearchTerm     `json:"terms"`
	Fitments  []ProductFitment `json:"fitments"`
}

type FitmentInput struct {
	VehicleVariantID uuid.UUID `json:"vehicle_variant_id"`
	YearFrom         *int      `json:"year_from,omitempty"`
	YearTo           *int      `json:"year_to,omitempty"`
	Notes            string    `json:"notes,omitempty"`
}

type UpdateMetadata struct {
	Terms    []SearchTerm   `json:"terms"`
	Fitments []FitmentInput `json:"fitments"`
}
