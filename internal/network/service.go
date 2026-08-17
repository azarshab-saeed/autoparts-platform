package network

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type candidate struct {
	SearchResult
	lat pgtype.Float8
	lng pgtype.Float8
}

func (s *Service) Search(ctx context.Context, q string, vehicleVariantID *uuid.UUID, vehicleYear *int, lat, lng *float64, order string, limit int) ([]SearchResult, error) {
	q = normalizeSearchText(q)
	if q == "" && vehicleVariantID == nil {
		return nil, errors.New("search query or vehicle filter is required")
	}
	if q != "" && len([]rune(q)) < 2 {
		return nil, errors.New("search query must contain at least 2 characters")
	}
	if vehicleYear != nil && vehicleVariantID == nil {
		return nil, errors.New("vehicle year requires vehicle_variant_id")
	}
	if vehicleYear != nil && (*vehicleYear < 1200 || *vehicleYear > 2200) {
		return nil, errors.New("vehicle year must be between 1200 and 2200")
	}
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.Query(ctx, `
		SELECT o.id,p.id,p.title,COALESCE(p.sku,''),COALESCE(p.brand,''),COALESCE(p.oem_code,''),
		       s.id,s.name,COALESCE(s.city,''),COALESCE(s.public_address,''),COALESCE(s.public_phone,''),
		       s.latitude::float8,s.longitude::float8,o.selling_price,
		       (ib.on_hand-ib.reserved)::float8,o.allow_reservation,o.allow_procurement,
		       GREATEST(ib.updated_at,o.last_verified_at) AS last_updated_at,
		       COALESCE(terms.exact_match,false),COALESCE(terms.exact_code_match,false),COALESCE(fit.variant_match,false),COALESCE(fit.summary,'')
		FROM store_product_offers o
		JOIN stores s ON s.id=o.store_id AND s.tenant_id=o.tenant_id
		JOIN products p ON p.id=o.product_id AND p.tenant_id=o.tenant_id
		JOIN inventory_balances ib ON ib.tenant_id=o.tenant_id AND ib.warehouse_id=o.warehouse_id AND ib.product_id=o.product_id
		LEFT JOIN LATERAL (
		  SELECT string_agg(pst.normalized_term,' ') AS all_terms,
		         bool_or(pst.normalized_term=$1) AS exact_match,
		         bool_or(pst.normalized_term=$1 AND pst.kind IN ('oem','equivalent')) AS exact_code_match
		  FROM product_search_terms pst
		  WHERE pst.tenant_id=p.tenant_id AND pst.product_id=p.id
		) terms ON true
		LEFT JOIN LATERAL (
		  SELECT bool_or(pf.vehicle_variant_id=$2::uuid AND
		                 ($3::int IS NULL OR ((COALESCE(pf.year_from,v.year_from) IS NULL OR COALESCE(pf.year_from,v.year_from) <= $3) AND (COALESCE(pf.year_to,v.year_to) IS NULL OR COALESCE(pf.year_to,v.year_to) >= $3)))) AS variant_match,
		         string_agg(DISTINCT concat_ws(' ',mk.name,mo.name,v.name,NULLIF(v.engine_code,'')),'، ') AS summary
		  FROM product_fitments pf
		  JOIN vehicle_variants v ON v.id=pf.vehicle_variant_id
		  JOIN vehicle_models mo ON mo.id=v.model_id
		  JOIN vehicle_makes mk ON mk.id=mo.make_id
		  WHERE pf.tenant_id=p.tenant_id AND pf.product_id=p.id
		) fit ON true
		CROSS JOIN LATERAL (
		  SELECT translate(lower(concat_ws(' ',p.normalized_title,p.brand,p.oem_code,p.sku,COALESCE(terms.all_terms,''))),
		    '۰۱۲۳۴۵۶۷۸۹٠١٢٣٤٥٦٧٨٩','01234567890123456789') AS value
		) search_text
		WHERE s.network_enabled AND s.active AND o.visible AND p.active AND p.deleted_at IS NULL
		  AND (ib.on_hand-ib.reserved) > 0
		  AND ($1='' OR NOT EXISTS (
		    SELECT 1 FROM unnest(regexp_split_to_array($1, '[[:space:]]+')) token
		    WHERE token <> '' AND search_text.value NOT ILIKE '%' || token || '%'
		  ))
		  AND ($2::uuid IS NULL OR COALESCE(fit.variant_match,false))
		ORDER BY GREATEST(ib.updated_at,o.last_verified_at) DESC
		LIMIT 300`, q, vehicleVariantID, vehicleYear)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	items := make([]candidate, 0, 48)
	for rows.Next() {
		var c candidate
		var exactAlias, exactCodeTerm, variantMatch bool
		if err := rows.Scan(&c.OfferID, &c.ProductID, &c.Title, &c.SKU, &c.Brand, &c.OEMCode,
			&c.StoreID, &c.StoreName, &c.City, &c.Address, &c.Phone, &c.lat, &c.lng, &c.SellingPrice,
			&c.Available, &c.AllowReservation, &c.AllowProcurement, &c.LastUpdatedAt, &exactAlias, &exactCodeTerm, &variantMatch, &c.FitmentSummary); err != nil {
			return nil, err
		}
		age := now.Sub(c.LastUpdatedAt)
		switch {
		case age <= 15*time.Minute:
			c.Freshness = "live"
		case age <= 24*time.Hour:
			c.Freshness = "recent"
		default:
			c.Freshness = "stale"
		}
		if lat != nil && lng != nil && c.lat.Valid && c.lng.Valid {
			d := haversine(*lat, *lng, c.lat.Float64, c.lng.Float64)
			c.DistanceKM = &d
		}
		c.FitmentMatch = variantMatch
		if q == "" {
			c.Score = 40
			c.MatchReason = "vehicle_fitment"
		} else {
			normOEM := normalizeSearchText(c.OEMCode)
			normSKU := normalizeSearchText(c.SKU)
			normTitle := normalizeSearchText(c.Title)
			switch {
			case q == normOEM || q == normSKU || exactCodeTerm:
				c.Score = 100
				c.MatchReason = "exact_code"
			case exactAlias:
				c.Score = 90
				c.MatchReason = "exact_alias"
			case strings.Contains(normTitle, q):
				c.Score = 75
				c.MatchReason = "title"
			default:
				c.Score = 55
				c.MatchReason = "keyword"
			}
		}
		if variantMatch {
			c.Score += 40
			if c.MatchReason == "" || c.MatchReason == "keyword" {
				c.MatchReason = "vehicle_fitment"
			}
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	freshRank := func(v string) int {
		if v == "live" {
			return 0
		}
		if v == "recent" {
			return 1
		}
		return 2
	}
	distance := func(c candidate) float64 {
		if c.DistanceKM == nil {
			return math.MaxFloat64
		}
		return *c.DistanceKM
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch order {
		case "price":
			if a.SellingPrice != b.SellingPrice {
				return a.SellingPrice < b.SellingPrice
			}
			return a.Score > b.Score
		case "distance":
			if distance(a) != distance(b) {
				return distance(a) < distance(b)
			}
			return a.Score > b.Score
		case "fresh":
			if !a.LastUpdatedAt.Equal(b.LastUpdatedAt) {
				return a.LastUpdatedAt.After(b.LastUpdatedAt)
			}
			return a.Score > b.Score
		default:
			if a.Score != b.Score {
				return a.Score > b.Score
			}
			if freshRank(a.Freshness) != freshRank(b.Freshness) {
				return freshRank(a.Freshness) < freshRank(b.Freshness)
			}
			if distance(a) != distance(b) {
				return distance(a) < distance(b)
			}
			return a.SellingPrice < b.SellingPrice
		}
	})
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]SearchResult, len(items))
	for i := range items {
		out[i] = items[i].SearchResult
	}
	return out, nil
}

func (s *Service) SearchProcurement(ctx context.Context, buyerStoreID uuid.UUID, q string, lat, lng *float64, order string, limit int) ([]SearchResult, error) {
	if buyerStoreID == uuid.Nil {
		return nil, errors.New("buyer store is required")
	}
	want := limit
	if want < 1 || want > 100 {
		want = 30
	}
	items, err := s.Search(ctx, q, nil, nil, lat, lng, order, 100)
	if err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, want)
	for _, item := range items {
		if item.StoreID == buyerStoreID || !item.AllowProcurement {
			continue
		}
		out = append(out, item)
		if len(out) == want {
			break
		}
	}
	return out, nil
}

func (s *Service) ListStoreOffers(ctx context.Context, tenantID, storeID, warehouseID uuid.UUID) ([]StoreOffer, error) {
	rows, err := s.db.Query(ctx, `
		SELECT p.id,p.title,COALESCE(p.sku,''),COALESCE(p.brand,''),
		       ib.on_hand::float8,ib.reserved::float8,(ib.on_hand-ib.reserved)::float8,
		       COALESCE(o.selling_price,0),COALESCE(o.visible,false),COALESCE(o.allow_reservation,false),COALESCE(o.allow_procurement,false),
		       COALESCE(o.last_verified_at,ib.updated_at)
		FROM inventory_balances ib
		JOIN warehouses w ON w.id=ib.warehouse_id AND w.tenant_id=ib.tenant_id AND w.store_id=$2
		JOIN products p ON p.id=ib.product_id AND p.tenant_id=ib.tenant_id
		LEFT JOIN store_product_offers o ON o.tenant_id=ib.tenant_id AND o.store_id=$2 AND o.warehouse_id=ib.warehouse_id AND o.product_id=ib.product_id
		WHERE ib.tenant_id=$1 AND ib.warehouse_id=$3 AND p.active AND p.deleted_at IS NULL
		ORDER BY p.title`, tenantID, storeID, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoreOffer
	for rows.Next() {
		var x StoreOffer
		if err := rows.Scan(&x.ProductID, &x.Title, &x.SKU, &x.Brand, &x.OnHand, &x.Reserved, &x.Available, &x.SellingPrice, &x.Visible, &x.AllowReservation, &x.AllowProcurement, &x.LastVerifiedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) UpsertOffer(ctx context.Context, tenantID, storeID, productID uuid.UUID, in UpdateOffer) error {
	if in.WarehouseID == uuid.Nil {
		return errors.New("warehouse_id is required")
	}
	if in.SellingPrice <= 0 {
		return errors.New("selling_price must be positive")
	}
	var ok bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND tenant_id=$2 AND store_id=$3)`, in.WarehouseID, tenantID, storeID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("warehouse does not belong to store")
	}
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND tenant_id=$2 AND active AND deleted_at IS NULL)`, productID, tenantID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("product does not belong to tenant")
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO store_product_offers(tenant_id,store_id,warehouse_id,product_id,selling_price,visible,allow_reservation,allow_procurement,last_verified_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,now())
		ON CONFLICT(tenant_id,store_id,warehouse_id,product_id)
		DO UPDATE SET selling_price=EXCLUDED.selling_price,visible=EXCLUDED.visible,allow_reservation=EXCLUDED.allow_reservation,allow_procurement=EXCLUDED.allow_procurement,last_verified_at=now(),updated_at=now()`,
		tenantID, storeID, in.WarehouseID, productID, in.SellingPrice, in.Visible, in.AllowReservation, in.AllowProcurement)
	return err
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earth = 6371.0
	r := math.Pi / 180
	dlat := (lat2 - lat1) * r
	dlon := (lon2 - lon1) * r
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1*r)*math.Cos(lat2*r)*math.Sin(dlon/2)*math.Sin(dlon/2)
	return earth * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func (s *Service) GetStoreProfile(ctx context.Context, tenantID, storeID uuid.UUID) (StoreProfile, error) {
	var out StoreProfile
	var lat, lng pgtype.Float8
	err := s.db.QueryRow(ctx, `SELECT id,name,network_enabled,COALESCE(public_address,''),COALESCE(public_phone,''),COALESCE(city,''),latitude::float8,longitude::float8 FROM stores WHERE id=$1 AND tenant_id=$2 AND active`, storeID, tenantID).
		Scan(&out.StoreID, &out.StoreName, &out.NetworkEnabled, &out.Address, &out.Phone, &out.City, &lat, &lng)
	if err == nil {
		if lat.Valid {
			v := lat.Float64
			out.Latitude = &v
		}
		if lng.Valid {
			v := lng.Float64
			out.Longitude = &v
		}
	}
	return out, err
}

func (s *Service) UpdateStoreProfile(ctx context.Context, tenantID, storeID uuid.UUID, in UpdateStoreProfile) error {
	if (in.Latitude == nil) != (in.Longitude == nil) {
		return errors.New("latitude and longitude must be provided together")
	}
	if in.Latitude != nil && (*in.Latitude < -90 || *in.Latitude > 90) {
		return errors.New("latitude is outside valid range")
	}
	if in.Longitude != nil && (*in.Longitude < -180 || *in.Longitude > 180) {
		return errors.New("longitude is outside valid range")
	}
	ct, err := s.db.Exec(ctx, `UPDATE stores SET network_enabled=$3,public_address=$4,public_phone=$5,city=$6,latitude=$7,longitude=$8,network_updated_at=now(),updated_at=now() WHERE id=$1 AND tenant_id=$2 AND active`, storeID, tenantID, in.NetworkEnabled, strings.TrimSpace(in.Address), strings.TrimSpace(in.Phone), strings.TrimSpace(in.City), in.Latitude, in.Longitude)
	if err != nil {
		return err
	}
	if ct.RowsAffected() != 1 {
		return errors.New("store not found")
	}
	return nil
}

func normalizeSearchText(v string) string {
	r := strings.NewReplacer(
		"۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4", "۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
		"٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4", "٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9",
		"ي", "ی", "ى", "ی", "ك", "ک",
	)
	return strings.ToLower(strings.TrimSpace(r.Replace(v)))
}
