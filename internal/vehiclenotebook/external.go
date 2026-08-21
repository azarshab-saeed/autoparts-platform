package vehiclenotebook

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateExternal creates a vehicle-owned notebook from an external mechanic.
// Store/tenant provenance is intentionally nullable; owner PII remains private.
func (s *Service) CreateExternal(ctx context.Context, actorID uuid.UUID, actorRole string, in CreateVehicle) (Vehicle, error) {
	if actorID == uuid.Nil || actorRole != "mechanic" {
		return Vehicle{}, errors.New("authenticated mechanic is required")
	}
	if blank(in.Plate) && blank(in.VIN) {
		return Vehicle{}, errors.New("plate or VIN is required")
	}
	if in.ModelYear != nil && (*in.ModelYear < 1200 || *in.ModelYear > 2200) {
		return Vehicle{}, errors.New("model year is out of range")
	}
	// A mechanic cannot bind an arbitrary store customer record.
	in.OwnerCustomerID = nil
	publicToken := uuid.New()
	ownerCode, err := generateOwnerCode()
	if err != nil {
		return Vehicle{}, err
	}
	ownerHash := hashOwnerCode(publicToken, ownerCode)
	var out Vehicle
	err = s.db.QueryRow(ctx, `
		INSERT INTO vehicle_notebooks(
			tenant_id,store_id,public_token,owner_customer_id,owner_name,owner_phone,plate,vin,make,model,trim,model_year,owner_code_hash,created_by,origin_role,origin_user_id
		) VALUES(NULL,NULL,$1,NULL,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'mechanic',$11)
		RETURNING id,public_token,owner_customer_id,owner_name,owner_phone,plate,vin,make,model,trim,model_year,updated_at`,
		publicToken, clean(in.OwnerName), clean(in.OwnerPhone), clean(in.Plate), clean(in.VIN), clean(in.Make), clean(in.Model), clean(in.Trim), in.ModelYear, ownerHash, actorID,
	).Scan(&out.ID, &out.PublicToken, &out.OwnerCustomerID, &out.OwnerName, &out.OwnerPhone, &out.Plate, &out.VIN, &out.Make, &out.Model, &out.Trim, &out.ModelYear, &out.UpdatedAt)
	if err != nil {
		return Vehicle{}, err
	}
	out.OwnerCode = ownerCode
	return out, nil
}

func (s *Service) ListExternal(ctx context.Context, actorID uuid.UUID, q string, limit int) ([]Vehicle, error) {
	if actorID == uuid.Nil {
		return nil, errors.New("authenticated mechanic is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	needle := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	rows, err := s.db.Query(ctx, `
		SELECT id,public_token,owner_customer_id,owner_name,owner_phone,plate,vin,make,model,trim,model_year,updated_at
		FROM vehicle_notebooks
		WHERE origin_role='mechanic' AND origin_user_id=$1 AND deleted_at IS NULL
		  AND ($2='%%' OR lower(COALESCE(plate,'')) LIKE $2 OR lower(COALESCE(vin,'')) LIKE $2
		       OR lower(COALESCE(owner_name,'')) LIKE $2 OR lower(COALESCE(owner_phone,'')) LIKE $2
		       OR lower(COALESCE(make,'')||' '||COALESCE(model,'')||' '||COALESCE(trim,'')) LIKE $2)
		ORDER BY updated_at DESC,id DESC LIMIT $3`, actorID, needle, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Vehicle, 0, limit)
	for rows.Next() {
		var v Vehicle
		if err := rows.Scan(&v.ID, &v.PublicToken, &v.OwnerCustomerID, &v.OwnerName, &v.OwnerPhone, &v.Plate, &v.VIN, &v.Make, &v.Model, &v.Trim, &v.ModelYear, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ByTokenForViewer preserves owner PII only for the creating mechanic or the
// owning store. All other mechanics/stores receive a privacy-reduced view.
func (s *Service) ByTokenForViewer(ctx context.Context, token, viewerUserID, viewerTenantID, viewerStoreID uuid.UUID) (Detail, error) {
	var v Vehicle
	var ownerTenant, ownerStore, originUser *uuid.UUID
	var originRole string
	err := s.db.QueryRow(ctx, `
		SELECT id,public_token,tenant_id,store_id,origin_role,origin_user_id,owner_customer_id,owner_name,owner_phone,plate,vin,make,model,trim,model_year,updated_at
		FROM vehicle_notebooks WHERE public_token=$1 AND deleted_at IS NULL`, token,
	).Scan(&v.ID, &v.PublicToken, &ownerTenant, &ownerStore, &originRole, &originUser, &v.OwnerCustomerID, &v.OwnerName, &v.OwnerPhone, &v.Plate, &v.VIN, &v.Make, &v.Model, &v.Trim, &v.ModelYear, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, errors.New("vehicle notebook not found")
	}
	if err != nil {
		return Detail{}, err
	}
	allowedPII := originRole == "mechanic" && originUser != nil && *originUser == viewerUserID
	if ownerTenant != nil && ownerStore != nil && viewerTenantID != uuid.Nil && viewerStoreID != uuid.Nil {
		allowedPII = allowedPII || (*ownerTenant == viewerTenantID && *ownerStore == viewerStoreID)
	}
	if !allowedPII {
		v.OwnerCustomerID = nil
		v.OwnerName = nil
		v.OwnerPhone = nil
		v.VIN = nil
	}
	entries, err := s.entries(ctx, v.ID)
	if err != nil {
		return Detail{}, err
	}
	return Detail{Vehicle: v, Entries: entries}, nil
}

func (s *Service) RotateExternalOwnerCode(ctx context.Context, actorID, vehicleID uuid.UUID) (string, error) {
	code, err := generateOwnerCode()
	if err != nil {
		return "", err
	}
	var token uuid.UUID
	if err := s.db.QueryRow(ctx, `SELECT public_token FROM vehicle_notebooks WHERE id=$1 AND origin_role='mechanic' AND origin_user_id=$2 AND deleted_at IS NULL`, vehicleID, actorID).Scan(&token); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("vehicle notebook does not belong to authenticated mechanic")
		}
		return "", err
	}
	if _, err := s.db.Exec(ctx, `UPDATE vehicle_notebooks SET owner_code_hash=$3,updated_at=now() WHERE id=$1 AND origin_user_id=$2`, vehicleID, actorID, hashOwnerCode(token, code)); err != nil {
		return "", err
	}
	return code, nil
}
