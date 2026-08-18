package productunit

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Unit struct {
	ID                     uuid.UUID
	Code                   string
	Name                   string
	FactorToBase           float64
	Barcode                *string
	IsBase                 bool
	AllowSale              bool
	AllowPurchase          bool
	AllowFractionalBaseQty bool
}

func Resolve(ctx context.Context, q Querier, tenantID, productID uuid.UUID, unitID *uuid.UUID) (Unit, error) {
	var u Unit
	if unitID == nil || *unitID == uuid.Nil {
		err := q.QueryRow(ctx, `SELECT pu.id,pu.code,pu.name,pu.factor_to_base::float8,pu.barcode,pu.is_base,pu.allow_sale,pu.allow_purchase,p.allow_fractional_base_qty
			FROM product_units pu JOIN products p ON p.id=pu.product_id AND p.tenant_id=pu.tenant_id
			WHERE pu.tenant_id=$1 AND pu.product_id=$2 AND pu.is_base AND pu.active AND p.deleted_at IS NULL`, tenantID, productID).Scan(&u.ID, &u.Code, &u.Name, &u.FactorToBase, &u.Barcode, &u.IsBase, &u.AllowSale, &u.AllowPurchase, &u.AllowFractionalBaseQty)
		if errors.Is(err, pgx.ErrNoRows) {
			return Unit{}, errors.New("product base unit not found")
		}
		return u, err
	}
	err := q.QueryRow(ctx, `SELECT pu.id,pu.code,pu.name,pu.factor_to_base::float8,pu.barcode,pu.is_base,pu.allow_sale,pu.allow_purchase,p.allow_fractional_base_qty
		FROM product_units pu JOIN products p ON p.id=pu.product_id AND p.tenant_id=pu.tenant_id
		WHERE pu.id=$1 AND pu.tenant_id=$2 AND pu.product_id=$3 AND pu.active AND p.deleted_at IS NULL`, *unitID, tenantID, productID).Scan(&u.ID, &u.Code, &u.Name, &u.FactorToBase, &u.Barcode, &u.IsBase, &u.AllowSale, &u.AllowPurchase, &u.AllowFractionalBaseQty)
	if errors.Is(err, pgx.ErrNoRows) {
		return Unit{}, errors.New("product unit not found")
	}
	return u, err
}

func BaseQty(commercialQty float64, u Unit) (float64, error) {
	if commercialQty <= 0 || math.IsNaN(commercialQty) || math.IsInf(commercialQty, 0) {
		return 0, errors.New("quantity must be positive")
	}
	if !u.AllowFractionalBaseQty && math.Abs(commercialQty-math.Round(commercialQty)) > 1e-9 {
		return 0, errors.New("commercial quantity must be a whole unit for this product")
	}
	base := commercialQty * u.FactorToBase
	if base <= 0 || math.IsNaN(base) || math.IsInf(base, 0) {
		return 0, errors.New("invalid converted quantity")
	}
	if !u.AllowFractionalBaseQty && math.Abs(base-math.Round(base)) > 1e-9 {
		return 0, errors.New("quantity converts to a fractional base unit")
	}
	return base, nil
}

func BaseMoney(commercialMoney int64, u Unit) int64 {
	if u.FactorToBase <= 0 {
		return commercialMoney
	}
	return int64(math.Round(float64(commercialMoney) / u.FactorToBase))
}

func CommercialMoney(baseMoney int64, u Unit) int64 {
	return int64(math.Round(float64(baseMoney) * u.FactorToBase))
}
