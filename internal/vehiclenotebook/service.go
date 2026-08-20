package vehiclenotebook

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type Vehicle struct {
	ID              uuid.UUID  `json:"id"`
	PublicToken     uuid.UUID  `json:"public_token"`
	OwnerCustomerID *uuid.UUID `json:"owner_customer_id,omitempty"`
	OwnerName       *string    `json:"owner_name,omitempty"`
	OwnerPhone      *string    `json:"owner_phone,omitempty"`
	Plate           *string    `json:"plate,omitempty"`
	VIN             *string    `json:"vin,omitempty"`
	Make            *string    `json:"make,omitempty"`
	Model           *string    `json:"model,omitempty"`
	Trim            *string    `json:"trim,omitempty"`
	ModelYear       *int       `json:"model_year,omitempty"`
	OwnerCode       string     `json:"owner_code,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Entry struct {
	ID             uuid.UUID  `json:"id"`
	Kind           string     `json:"kind"`
	Title          string     `json:"title"`
	Mileage        *int       `json:"mileage,omitempty"`
	OccurredOn     time.Time  `json:"occurred_on"`
	NextDueMileage *int       `json:"next_due_mileage,omitempty"`
	NextDueDate    *time.Time `json:"next_due_date,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
	ActorRole      string     `json:"actor_role"`
	ActorName      string     `json:"actor_name"`
	OwnerReported  bool       `json:"owner_reported"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Detail struct {
	Vehicle Vehicle `json:"vehicle"`
	Entries []Entry `json:"entries"`
}

type PublicVehicle struct {
	PublicToken uuid.UUID `json:"public_token"`
	PlateMasked string    `json:"plate_masked,omitempty"`
	Make        *string   `json:"make,omitempty"`
	Model       *string   `json:"model,omitempty"`
	Trim        *string   `json:"trim,omitempty"`
	ModelYear   *int      `json:"model_year,omitempty"`
}

type PublicDetail struct {
	Vehicle PublicVehicle `json:"vehicle"`
	Entries []Entry       `json:"entries"`
}

type CreateVehicle struct {
	OwnerCustomerID *uuid.UUID `json:"owner_customer_id,omitempty"`
	OwnerName       *string    `json:"owner_name,omitempty"`
	OwnerPhone      *string    `json:"owner_phone,omitempty"`
	Plate           *string    `json:"plate,omitempty"`
	VIN             *string    `json:"vin,omitempty"`
	Make            *string    `json:"make,omitempty"`
	Model           *string    `json:"model,omitempty"`
	Trim            *string    `json:"trim,omitempty"`
	ModelYear       *int       `json:"model_year,omitempty"`
}

type AddEntry struct {
	Kind           string     `json:"kind"`
	Title          string     `json:"title"`
	Mileage        *int       `json:"mileage,omitempty"`
	OccurredOn     *time.Time `json:"occurred_on,omitempty"`
	NextDueMileage *int       `json:"next_due_mileage,omitempty"`
	NextDueDate    *time.Time `json:"next_due_date,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
}

type OwnerMileageInput struct {
	OwnerCode  string     `json:"owner_code"`
	Mileage    int        `json:"mileage"`
	OccurredOn *time.Time `json:"occurred_on,omitempty"`
}

func (s *Service) Create(ctx context.Context, tenantID, storeID, actorID uuid.UUID, in CreateVehicle) (Vehicle, error) {
	if tenantID == uuid.Nil || storeID == uuid.Nil {
		return Vehicle{}, errors.New("store context is required")
	}
	if blank(in.Plate) && blank(in.VIN) && blank(in.OwnerPhone) {
		return Vehicle{}, errors.New("plate, VIN or owner phone is required")
	}
	if in.ModelYear != nil && (*in.ModelYear < 1200 || *in.ModelYear > 2200) {
		return Vehicle{}, errors.New("model year is out of range")
	}
	if in.OwnerCustomerID != nil {
		var ok bool
		if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND deleted_at IS NULL)`, *in.OwnerCustomerID, tenantID, storeID).Scan(&ok); err != nil {
			return Vehicle{}, err
		}
		if !ok {
			return Vehicle{}, errors.New("customer does not belong to authenticated store")
		}
	}
	publicToken := uuid.New()
	ownerCode, err := generateOwnerCode()
	if err != nil {
		return Vehicle{}, err
	}
	ownerHash := hashOwnerCode(publicToken, ownerCode)
	var out Vehicle
	err = s.db.QueryRow(ctx, `
		INSERT INTO vehicle_notebooks(
			tenant_id,store_id,public_token,owner_customer_id,owner_name,owner_phone,plate,vin,make,model,trim,model_year,owner_code_hash,created_by
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id,public_token,owner_customer_id,owner_name,owner_phone,plate,vin,make,model,trim,model_year,updated_at`,
		tenantID, storeID, publicToken, in.OwnerCustomerID, clean(in.OwnerName), clean(in.OwnerPhone), clean(in.Plate), clean(in.VIN), clean(in.Make), clean(in.Model), clean(in.Trim), in.ModelYear, ownerHash, actorID,
	).Scan(&out.ID, &out.PublicToken, &out.OwnerCustomerID, &out.OwnerName, &out.OwnerPhone, &out.Plate, &out.VIN, &out.Make, &out.Model, &out.Trim, &out.ModelYear, &out.UpdatedAt)
	if err != nil {
		return Vehicle{}, err
	}
	out.OwnerCode = ownerCode
	return out, nil
}

func (s *Service) List(ctx context.Context, tenantID, storeID uuid.UUID, q string, limit int) ([]Vehicle, error) {
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	needle := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	rows, err := s.db.Query(ctx, `
		SELECT id,public_token,owner_customer_id,owner_name,owner_phone,plate,vin,make,model,trim,model_year,updated_at
		FROM vehicle_notebooks
		WHERE tenant_id=$1 AND store_id=$2 AND deleted_at IS NULL
		  AND ($3='%%' OR lower(COALESCE(plate,'')) LIKE $3 OR lower(COALESCE(vin,'')) LIKE $3 OR lower(COALESCE(owner_name,'')) LIKE $3 OR lower(COALESCE(owner_phone,'')) LIKE $3 OR lower(COALESCE(make,'')||' '||COALESCE(model,'')||' '||COALESCE(trim,'')) LIKE $3)
		ORDER BY updated_at DESC,id DESC LIMIT $4`, tenantID, storeID, needle, limit)
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

func (s *Service) ByToken(ctx context.Context, token uuid.UUID, viewerTenantID, viewerStoreID uuid.UUID) (Detail, error) {
	var v Vehicle
	var ownerTenant, ownerStore uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT id,public_token,tenant_id,store_id,owner_customer_id,owner_name,owner_phone,plate,vin,make,model,trim,model_year,updated_at
		FROM vehicle_notebooks WHERE public_token=$1 AND deleted_at IS NULL`, token,
	).Scan(&v.ID, &v.PublicToken, &ownerTenant, &ownerStore, &v.OwnerCustomerID, &v.OwnerName, &v.OwnerPhone, &v.Plate, &v.VIN, &v.Make, &v.Model, &v.Trim, &v.ModelYear, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, errors.New("vehicle notebook not found")
	}
	if err != nil {
		return Detail{}, err
	}
	if viewerTenantID != ownerTenant || viewerStoreID != ownerStore {
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

func (s *Service) PublicByToken(ctx context.Context, token uuid.UUID) (PublicDetail, error) {
	var v PublicVehicle
	var plate *string
	err := s.db.QueryRow(ctx, `SELECT public_token,plate,make,model,trim,model_year FROM vehicle_notebooks WHERE public_token=$1 AND deleted_at IS NULL`, token).Scan(&v.PublicToken, &plate, &v.Make, &v.Model, &v.Trim, &v.ModelYear)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicDetail{}, errors.New("vehicle notebook not found")
	}
	if err != nil {
		return PublicDetail{}, err
	}
	v.PlateMasked = maskPlate(plate)
	var vehicleID uuid.UUID
	if err := s.db.QueryRow(ctx, `SELECT id FROM vehicle_notebooks WHERE public_token=$1 AND deleted_at IS NULL`, token).Scan(&vehicleID); err != nil {
		return PublicDetail{}, err
	}
	entries, err := s.entries(ctx, vehicleID)
	if err != nil {
		return PublicDetail{}, err
	}
	return PublicDetail{Vehicle: v, Entries: entries}, nil
}

func (s *Service) AddEntryByToken(ctx context.Context, token uuid.UUID, actorID uuid.UUID, actorRole, actorName string, tenantID, storeID uuid.UUID, in AddEntry) (Entry, error) {
	if err := validateEntry(in); err != nil {
		return Entry{}, err
	}
	var vehicleID uuid.UUID
	if err := s.db.QueryRow(ctx, `SELECT id FROM vehicle_notebooks WHERE public_token=$1 AND deleted_at IS NULL`, token).Scan(&vehicleID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, errors.New("vehicle notebook not found")
		}
		return Entry{}, err
	}
	provider := strings.TrimSpace(actorName)
	var tenantArg, storeArg any
	if tenantID != uuid.Nil {
		tenantArg = tenantID
	}
	if storeID != uuid.Nil {
		storeArg = storeID
		var storeName string
		if err := s.db.QueryRow(ctx, `SELECT name FROM stores WHERE id=$1`, storeID).Scan(&storeName); err == nil && strings.TrimSpace(storeName) != "" {
			provider = storeName
		}
	}
	if provider == "" {
		if actorRole == "mechanic" {
			provider = "مکانیک"
		} else {
			provider = "کاربر فروشگاه"
		}
	}
	occurred := time.Now()
	if in.OccurredOn != nil {
		occurred = *in.OccurredOn
	}
	var out Entry
	err := s.db.QueryRow(ctx, `
		INSERT INTO vehicle_notebook_entries(vehicle_id,tenant_id,store_id,actor_user_id,actor_role,actor_name,kind,title,mileage,occurred_on,next_due_mileage,next_due_date,notes,owner_reported)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,false)
		RETURNING id,kind,title,mileage,occurred_on,next_due_mileage,next_due_date,notes,actor_role,actor_name,owner_reported,created_at`,
		vehicleID, tenantArg, storeArg, actorID, actorRole, provider, in.Kind, strings.TrimSpace(in.Title), in.Mileage, occurred, in.NextDueMileage, in.NextDueDate, clean(in.Notes),
	).Scan(&out.ID, &out.Kind, &out.Title, &out.Mileage, &out.OccurredOn, &out.NextDueMileage, &out.NextDueDate, &out.Notes, &out.ActorRole, &out.ActorName, &out.OwnerReported, &out.CreatedAt)
	if err != nil {
		return Entry{}, err
	}
	_, _ = s.db.Exec(ctx, `UPDATE vehicle_notebooks SET updated_at=now() WHERE id=$1`, vehicleID)
	return out, nil
}

func (s *Service) AddOwnerMileage(ctx context.Context, token uuid.UUID, in OwnerMileageInput) (Entry, error) {
	if in.Mileage < 0 {
		return Entry{}, errors.New("mileage must be zero or greater")
	}
	var vehicleID uuid.UUID
	var expected string
	if err := s.db.QueryRow(ctx, `SELECT id,owner_code_hash FROM vehicle_notebooks WHERE public_token=$1 AND deleted_at IS NULL`, token).Scan(&vehicleID, &expected); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, errors.New("vehicle notebook not found")
		}
		return Entry{}, err
	}
	if hashOwnerCode(token, strings.TrimSpace(in.OwnerCode)) != expected {
		return Entry{}, errors.New("owner code is invalid")
	}
	occurred := time.Now()
	if in.OccurredOn != nil {
		occurred = *in.OccurredOn
	}
	var out Entry
	err := s.db.QueryRow(ctx, `
		INSERT INTO vehicle_notebook_entries(vehicle_id,actor_role,actor_name,kind,title,mileage,occurred_on,owner_reported)
		VALUES($1,'owner','مالک خودرو','mileage','ثبت کیلومتر توسط مالک',$2,$3,true)
		RETURNING id,kind,title,mileage,occurred_on,next_due_mileage,next_due_date,notes,actor_role,actor_name,owner_reported,created_at`, vehicleID, in.Mileage, occurred,
	).Scan(&out.ID, &out.Kind, &out.Title, &out.Mileage, &out.OccurredOn, &out.NextDueMileage, &out.NextDueDate, &out.Notes, &out.ActorRole, &out.ActorName, &out.OwnerReported, &out.CreatedAt)
	if err != nil {
		return Entry{}, err
	}
	_, _ = s.db.Exec(ctx, `UPDATE vehicle_notebooks SET updated_at=now() WHERE id=$1`, vehicleID)
	return out, nil
}

func (s *Service) RotateOwnerCode(ctx context.Context, tenantID, storeID, vehicleID uuid.UUID) (string, error) {
	code, err := generateOwnerCode()
	if err != nil {
		return "", err
	}
	var token uuid.UUID
	if err := s.db.QueryRow(ctx, `SELECT public_token FROM vehicle_notebooks WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND deleted_at IS NULL`, vehicleID, tenantID, storeID).Scan(&token); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("vehicle notebook not found")
		}
		return "", err
	}
	if _, err := s.db.Exec(ctx, `UPDATE vehicle_notebooks SET owner_code_hash=$4,updated_at=now() WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, vehicleID, tenantID, storeID, hashOwnerCode(token, code)); err != nil {
		return "", err
	}
	return code, nil
}

func (s *Service) entries(ctx context.Context, vehicleID uuid.UUID) ([]Entry, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id,kind,title,mileage,occurred_on,next_due_mileage,next_due_date,notes,actor_role,actor_name,owner_reported,created_at
		FROM vehicle_notebook_entries WHERE vehicle_id=$1 ORDER BY occurred_on DESC,created_at DESC LIMIT 250`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Entry, 0, 32)
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Kind, &e.Title, &e.Mileage, &e.OccurredOn, &e.NextDueMileage, &e.NextDueDate, &e.Notes, &e.ActorRole, &e.ActorName, &e.OwnerReported, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func validateEntry(in AddEntry) error {
	switch in.Kind {
	case "service", "part", "mileage", "note":
	default:
		return errors.New("entry kind is invalid")
	}
	if strings.TrimSpace(in.Title) == "" {
		return errors.New("title is required")
	}
	if in.Mileage != nil && *in.Mileage < 0 {
		return errors.New("mileage must be zero or greater")
	}
	if in.NextDueMileage != nil && *in.NextDueMileage < 0 {
		return errors.New("next due mileage must be zero or greater")
	}
	return nil
}

func generateOwnerCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", fmt.Errorf("generate owner code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func hashOwnerCode(token uuid.UUID, code string) string {
	sum := sha256.Sum256([]byte(token.String() + ":" + strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}

func maskPlate(plate *string) string {
	if plate == nil {
		return ""
	}
	s := strings.TrimSpace(*plate)
	r := []rune(s)
	if len(r) <= 3 {
		return s
	}
	keep := 2
	if len(r) < 5 {
		keep = 1
	}
	return string(r[:keep]) + "•••" + string(r[len(r)-keep:])
}

func clean(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func blank(v *string) bool { return clean(v) == nil }
