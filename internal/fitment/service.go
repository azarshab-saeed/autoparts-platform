package fitment

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) Catalog(ctx context.Context) ([]VehicleMake, error) {
	rows, err := s.db.Query(ctx, `
		SELECT mk.id,mk.name,mo.id,mo.name,v.id,v.name,COALESCE(v.engine_code,''),v.year_from,v.year_to
		FROM vehicle_makes mk
		JOIN vehicle_models mo ON mo.make_id=mk.id
		JOIN vehicle_variants v ON v.model_id=mo.id AND v.active
		ORDER BY mk.name,mo.name,v.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	makes := make([]VehicleMake, 0, 8)
	makeIndex := map[uuid.UUID]int{}
	modelIndex := map[uuid.UUID]struct{ makePos, modelPos int }{}
	for rows.Next() {
		var makeID, modelID, variantID uuid.UUID
		var makeName, modelName, variantName, engine string
		var yearFrom, yearTo pgtype.Int4
		if err := rows.Scan(&makeID, &makeName, &modelID, &modelName, &variantID, &variantName, &engine, &yearFrom, &yearTo); err != nil {
			return nil, err
		}
		mi, ok := makeIndex[makeID]
		if !ok {
			mi = len(makes)
			makeIndex[makeID] = mi
			makes = append(makes, VehicleMake{ID: makeID, Name: makeName, Models: make([]VehicleModel, 0, 4)})
		}
		idx, ok := modelIndex[modelID]
		if !ok {
			idx = struct{ makePos, modelPos int }{mi, len(makes[mi].Models)}
			modelIndex[modelID] = idx
			makes[mi].Models = append(makes[mi].Models, VehicleModel{ID: modelID, Name: modelName, Variants: make([]VehicleVariant, 0, 4)})
		}
		v := VehicleVariant{ID: variantID, Name: variantName, EngineCode: engine}
		if yearFrom.Valid {
			y := int(yearFrom.Int32)
			v.YearFrom = &y
		}
		if yearTo.Valid {
			y := int(yearTo.Int32)
			v.YearTo = &y
		}
		makes[idx.makePos].Models[idx.modelPos].Variants = append(makes[idx.makePos].Models[idx.modelPos].Variants, v)
	}
	return makes, rows.Err()
}

func (s *Service) GetProductMetadata(ctx context.Context, tenantID, productID uuid.UUID) (ProductMetadata, error) {
	out := ProductMetadata{ProductID: productID, Terms: []SearchTerm{}, Fitments: []ProductFitment{}}
	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL)`, productID, tenantID).Scan(&exists); err != nil {
		return out, err
	}
	if !exists {
		return out, errors.New("product not found")
	}
	termRows, err := s.db.Query(ctx, `SELECT kind,term FROM product_search_terms WHERE tenant_id=$1 AND product_id=$2 ORDER BY kind,term`, tenantID, productID)
	if err != nil {
		return out, err
	}
	for termRows.Next() {
		var x SearchTerm
		if err := termRows.Scan(&x.Kind, &x.Term); err != nil {
			termRows.Close()
			return out, err
		}
		out.Terms = append(out.Terms, x)
	}
	if err := termRows.Err(); err != nil {
		termRows.Close()
		return out, err
	}
	termRows.Close()

	rows, err := s.db.Query(ctx, `
		SELECT pf.vehicle_variant_id,mk.name,mo.name,v.name,COALESCE(v.engine_code,''),COALESCE(pf.year_from,v.year_from),COALESCE(pf.year_to,v.year_to),COALESCE(pf.notes,'')
		FROM product_fitments pf
		JOIN vehicle_variants v ON v.id=pf.vehicle_variant_id
		JOIN vehicle_models mo ON mo.id=v.model_id
		JOIN vehicle_makes mk ON mk.id=mo.make_id
		WHERE pf.tenant_id=$1 AND pf.product_id=$2
		ORDER BY mk.name,mo.name,v.name`, tenantID, productID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var x ProductFitment
		var yf, yt pgtype.Int4
		if err := rows.Scan(&x.VehicleVariantID, &x.MakeName, &x.ModelName, &x.VariantName, &x.EngineCode, &yf, &yt, &x.Notes); err != nil {
			return out, err
		}
		if yf.Valid {
			y := int(yf.Int32)
			x.YearFrom = &y
		}
		if yt.Valid {
			y := int(yt.Int32)
			x.YearTo = &y
		}
		out.Fitments = append(out.Fitments, x)
	}
	return out, rows.Err()
}

func (s *Service) UpdateProductMetadata(ctx context.Context, tenantID, productID uuid.UUID, in UpdateMetadata) (ProductMetadata, error) {
	if len(in.Terms) > 50 || len(in.Fitments) > 50 {
		return ProductMetadata{}, errors.New("at most 50 search terms and 50 fitments are allowed")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ProductMetadata{}, err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL)`, productID, tenantID).Scan(&exists); err != nil {
		return ProductMetadata{}, err
	}
	if !exists {
		return ProductMetadata{}, errors.New("product not found")
	}
	if _, err = tx.Exec(ctx, `DELETE FROM product_search_terms WHERE tenant_id=$1 AND product_id=$2`, tenantID, productID); err != nil {
		return ProductMetadata{}, err
	}
	seenTerms := map[string]bool{}
	for _, term := range in.Terms {
		kind := strings.ToLower(strings.TrimSpace(term.Kind))
		if kind != "alias" && kind != "oem" && kind != "equivalent" {
			return ProductMetadata{}, errors.New("search term kind must be alias, oem, or equivalent")
		}
		raw := strings.TrimSpace(term.Term)
		normalized := normalize(raw)
		if len([]rune(normalized)) < 2 || len([]rune(raw)) > 120 {
			return ProductMetadata{}, errors.New("search terms must contain 2 to 120 characters")
		}
		key := kind + "\x00" + normalized
		if seenTerms[key] {
			continue
		}
		seenTerms[key] = true
		if _, err = tx.Exec(ctx, `INSERT INTO product_search_terms(tenant_id,product_id,kind,term,normalized_term) VALUES($1,$2,$3,$4,$5)`, tenantID, productID, kind, raw, normalized); err != nil {
			return ProductMetadata{}, err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM product_fitments WHERE tenant_id=$1 AND product_id=$2`, tenantID, productID); err != nil {
		return ProductMetadata{}, err
	}
	seenFitment := map[string]bool{}
	for _, f := range in.Fitments {
		if f.VehicleVariantID == uuid.Nil {
			return ProductMetadata{}, errors.New("vehicle_variant_id is required")
		}
		if f.YearFrom != nil && (*f.YearFrom < 1200 || *f.YearFrom > 2200) || f.YearTo != nil && (*f.YearTo < 1200 || *f.YearTo > 2200) {
			return ProductMetadata{}, errors.New("fitment year must be between 1200 and 2200")
		}
		if f.YearFrom != nil && f.YearTo != nil && *f.YearTo < *f.YearFrom {
			return ProductMetadata{}, errors.New("fitment year_to must be greater than or equal to year_from")
		}
		key := f.VehicleVariantID.String()
		if f.YearFrom != nil {
			key += ":" + strconv.Itoa(*f.YearFrom)
		}
		if f.YearTo != nil {
			key += ":" + strconv.Itoa(*f.YearTo)
		}
		if seenFitment[key] {
			continue
		}
		seenFitment[key] = true
		ct, e := tx.Exec(ctx, `
			INSERT INTO product_fitments(tenant_id,product_id,vehicle_variant_id,year_from,year_to,notes)
			SELECT $1,$2,v.id,$4,$5,$6 FROM vehicle_variants v WHERE v.id=$3 AND v.active`, tenantID, productID, f.VehicleVariantID, f.YearFrom, f.YearTo, strings.TrimSpace(f.Notes))
		if e != nil {
			return ProductMetadata{}, e
		}
		if ct.RowsAffected() != 1 {
			return ProductMetadata{}, errors.New("vehicle variant not found")
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return ProductMetadata{}, err
	}
	return s.GetProductMetadata(ctx, tenantID, productID)
}

func normalize(s string) string {
	replacer := strings.NewReplacer(
		"ي", "ی", "ى", "ی", "ك", "ک", "ۀ", "ه", "ة", "ه",
		"۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4", "۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
		"٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4", "٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9",
	)
	return strings.ToLower(strings.Join(strings.Fields(replacer.Replace(strings.TrimSpace(s))), " "))
}
