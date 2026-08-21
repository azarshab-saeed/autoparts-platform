package workshop

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type Job struct {
	ID            uuid.UUID  `json:"id"`
	VehicleID     uuid.UUID  `json:"vehicle_id"`
	PublicToken   uuid.UUID  `json:"public_token"`
	VehicleLabel  string     `json:"vehicle_label"`
	Plate         string     `json:"plate,omitempty"`
	CustomerName  string     `json:"customer_name,omitempty"`
	CustomerPhone string     `json:"customer_phone,omitempty"`
	Mileage       *int       `json:"mileage,omitempty"`
	Complaint     string     `json:"complaint,omitempty"`
	Diagnosis     string     `json:"diagnosis,omitempty"`
	Status        string     `json:"status"`
	LaborAmount   int64      `json:"labor_amount"`
	PartsAmount   int64      `json:"parts_amount"`
	TotalAmount   int64      `json:"total_amount"`
	PaidAmount    int64      `json:"paid_amount"`
	DueAmount     int64      `json:"due_amount"`
	OpenedAt      time.Time  `json:"opened_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Items         []Item     `json:"items,omitempty"`
	Payments      []Payment  `json:"payments,omitempty"`
}

type Item struct {
	ID            uuid.UUID  `json:"id"`
	ItemType      string     `json:"item_type"`
	Title         string     `json:"title"`
	ProductID     *uuid.UUID `json:"product_id,omitempty"`
	SourceStoreID *uuid.UUID `json:"source_store_id,omitempty"`
	SourceStore   string     `json:"source_store_name,omitempty"`
	ReservationID *uuid.UUID `json:"reservation_id,omitempty"`
	Qty           float64    `json:"qty"`
	UnitPrice     int64      `json:"unit_price"`
	LineTotal     int64      `json:"line_total"`
	Notes         string     `json:"notes,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Payment struct {
	ID        uuid.UUID `json:"id"`
	Method    string    `json:"method"`
	Amount    int64     `json:"amount"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateJob struct {
	VehicleToken  uuid.UUID `json:"vehicle_token"`
	CustomerName  string    `json:"customer_name,omitempty"`
	CustomerPhone string    `json:"customer_phone,omitempty"`
	Mileage       *int      `json:"mileage,omitempty"`
	Complaint     string    `json:"complaint,omitempty"`
	Diagnosis     string    `json:"diagnosis,omitempty"`
}

type AddItem struct {
	ItemType      string     `json:"item_type"`
	Title         string     `json:"title"`
	ProductID     *uuid.UUID `json:"product_id,omitempty"`
	SourceStoreID *uuid.UUID `json:"source_store_id,omitempty"`
	ReservationID *uuid.UUID `json:"reservation_id,omitempty"`
	Qty           float64    `json:"qty,omitempty"`
	UnitPrice     int64      `json:"unit_price,omitempty"`
	Notes         string     `json:"notes,omitempty"`
}

type AddPayment struct {
	Method string `json:"method"`
	Amount int64  `json:"amount"`
	Note   string `json:"note,omitempty"`
}

type Summary struct {
	OpenJobs          int64 `json:"open_jobs"`
	Completed30Days   int64 `json:"completed_30_days"`
	ReceivableAmount  int64 `json:"customer_receivable_amount"`
	Revenue30Days     int64 `json:"revenue_30_days"`
	VehicleCount      int64 `json:"vehicle_count"`
	NetworkPartAmount int64 `json:"network_part_amount_30_days"`
}

func (s *Service) ListJobs(ctx context.Context, mechanicID uuid.UUID, status string, limit int) ([]Job, error) {
	if mechanicID == uuid.Nil {
		return nil, errors.New("authenticated mechanic is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 60
	}
	where := `j.mechanic_user_id=$1`
	args := []any{mechanicID}
	if status != "" && status != "all" {
		if status != "open" && status != "completed" && status != "cancelled" {
			return nil, errors.New("invalid job status")
		}
		args = append(args, status)
		where += fmt.Sprintf(" AND j.status=$%d", len(args))
	}
	args = append(args, limit)
	rows, err := s.db.Query(ctx, jobSelect+` WHERE `+where+fmt.Sprintf(` ORDER BY j.updated_at DESC LIMIT $%d`, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Service) GetJob(ctx context.Context, mechanicID, jobID uuid.UUID) (Job, error) {
	j, err := scanJob(s.db.QueryRow(ctx, jobSelect+` WHERE j.id=$1 AND j.mechanic_user_id=$2`, jobID, mechanicID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, errors.New("workshop job not found")
	}
	if err != nil {
		return Job{}, err
	}
	items, err := s.items(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	payments, err := s.payments(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	j.Items, j.Payments = items, payments
	return j, nil
}

func (s *Service) CreateJob(ctx context.Context, mechanicID uuid.UUID, in CreateJob) (Job, error) {
	if mechanicID == uuid.Nil || in.VehicleToken == uuid.Nil {
		return Job{}, errors.New("mechanic and vehicle token are required")
	}
	if in.Mileage != nil && *in.Mileage < 0 {
		return Job{}, errors.New("mileage must be zero or greater")
	}
	var vehicleID uuid.UUID
	if err := s.db.QueryRow(ctx, `SELECT id FROM vehicle_notebooks WHERE public_token=$1 AND deleted_at IS NULL`, in.VehicleToken).Scan(&vehicleID); errors.Is(err, pgx.ErrNoRows) {
		return Job{}, errors.New("vehicle QR token was not found")
	} else if err != nil {
		return Job{}, err
	}
	id := uuid.New()
	_, err := s.db.Exec(ctx, `INSERT INTO workshop_jobs(id,mechanic_user_id,vehicle_id,customer_name,customer_phone,mileage,complaint,diagnosis)
		VALUES($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,NULLIF($7,''),NULLIF($8,''))`, id, mechanicID, vehicleID, strings.TrimSpace(in.CustomerName), strings.TrimSpace(in.CustomerPhone), in.Mileage, strings.TrimSpace(in.Complaint), strings.TrimSpace(in.Diagnosis))
	if err != nil {
		return Job{}, err
	}
	return s.GetJob(ctx, mechanicID, id)
}

func (s *Service) AddItem(ctx context.Context, mechanicID, jobID uuid.UUID, in AddItem) (Job, error) {
	if in.ItemType != "service" && in.ItemType != "part" && in.ItemType != "labor" {
		return Job{}, errors.New("item_type must be service, part, or labor")
	}
	if in.UnitPrice < 0 || in.Qty < 0 {
		return Job{}, errors.New("quantity and unit price cannot be negative")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM workshop_jobs WHERE id=$1 AND mechanic_user_id=$2 FOR UPDATE`, jobID, mechanicID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return Job{}, errors.New("workshop job not found")
	}
	if err != nil {
		return Job{}, err
	}
	if status != "open" {
		return Job{}, errors.New("only an open job can be edited")
	}
	qty := in.Qty
	title := strings.TrimSpace(in.Title)
	productID, sourceStoreID := in.ProductID, in.SourceStoreID
	unitPrice := in.UnitPrice
	if in.ReservationID != nil {
		var buyer, product, store uuid.UUID
		var reservationQty float64
		var reservationPrice int64
		var reservationStatus, productTitle string
		err = tx.QueryRow(ctx, `SELECT r.buyer_user_id,r.product_id,r.store_id,r.qty::float8,r.unit_price,r.status,p.title
			FROM network_reservations r JOIN products p ON p.id=r.product_id
			WHERE r.id=$1`, *in.ReservationID).Scan(&buyer, &product, &store, &reservationQty, &reservationPrice, &reservationStatus, &productTitle)
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, errors.New("network reservation not found")
		}
		if err != nil {
			return Job{}, err
		}
		if buyer != mechanicID {
			return Job{}, errors.New("network reservation does not belong to authenticated mechanic")
		}
		if reservationStatus != "fulfilled" {
			return Job{}, errors.New("only fulfilled network purchases can be attached to a workshop job")
		}
		productID, sourceStoreID = &product, &store
		if qty <= 0 {
			qty = reservationQty
		}
		if unitPrice == 0 {
			unitPrice = reservationPrice
		}
		if title == "" {
			title = productTitle
		}
		in.ItemType = "part"
	}
	if qty <= 0 {
		qty = 1
	}
	if title == "" {
		return Job{}, errors.New("item title is required")
	}
	line := int64(math.Round(qty * float64(unitPrice)))
	if line < 0 {
		return Job{}, errors.New("line total is invalid")
	}
	_, err = tx.Exec(ctx, `INSERT INTO workshop_job_items(job_id,item_type,title,product_id,source_store_id,reservation_id,qty,unit_price,line_total,notes)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,''))`, jobID, in.ItemType, title, productID, sourceStoreID, in.ReservationID, qty, unitPrice, line, strings.TrimSpace(in.Notes))
	if err != nil {
		return Job{}, err
	}
	if err = recalcTx(ctx, tx, jobID); err != nil {
		return Job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return s.GetJob(ctx, mechanicID, jobID)
}

func (s *Service) AddPayment(ctx context.Context, mechanicID, jobID uuid.UUID, in AddPayment) (Job, error) {
	if in.Amount <= 0 {
		return Job{}, errors.New("payment amount must be positive")
	}
	if in.Method != "cash" && in.Method != "card" && in.Method != "transfer" && in.Method != "credit" {
		return Job{}, errors.New("invalid payment method")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)
	var status string
	var total, paid int64
	if err = tx.QueryRow(ctx, `SELECT status,total_amount,paid_amount FROM workshop_jobs WHERE id=$1 AND mechanic_user_id=$2 FOR UPDATE`, jobID, mechanicID).Scan(&status, &total, &paid); errors.Is(err, pgx.ErrNoRows) {
		return Job{}, errors.New("workshop job not found")
	}
	if err != nil {
		return Job{}, err
	}
	if status == "cancelled" {
		return Job{}, errors.New("cancelled job cannot receive payment")
	}
	if paid+in.Amount > total {
		return Job{}, errors.New("payment exceeds job balance")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workshop_job_payments(job_id,mechanic_user_id,method,amount,note) VALUES($1,$2,$3,$4,NULLIF($5,''))`, jobID, mechanicID, in.Method, in.Amount, strings.TrimSpace(in.Note)); err != nil {
		return Job{}, err
	}
	if err = recalcTx(ctx, tx, jobID); err != nil {
		return Job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return s.GetJob(ctx, mechanicID, jobID)
}

func (s *Service) Complete(ctx context.Context, mechanicID, jobID uuid.UUID, mechanicName string) (Job, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)
	var vehicleID uuid.UUID
	var status string
	var mileage *int
	if err = tx.QueryRow(ctx, `SELECT vehicle_id,status,mileage FROM workshop_jobs WHERE id=$1 AND mechanic_user_id=$2 FOR UPDATE`, jobID, mechanicID).Scan(&vehicleID, &status, &mileage); errors.Is(err, pgx.ErrNoRows) {
		return Job{}, errors.New("workshop job not found")
	}
	if err != nil {
		return Job{}, err
	}
	if status != "open" {
		return Job{}, errors.New("only an open job can be completed")
	}
	provider := strings.TrimSpace(mechanicName)
	if provider == "" {
		provider = "مکانیک"
	}
	rows, err := tx.Query(ctx, `SELECT id,item_type,title,product_id,source_store_id,notes FROM workshop_job_items WHERE job_id=$1 ORDER BY created_at,id`, jobID)
	if err != nil {
		return Job{}, err
	}
	type x struct {
		id             uuid.UUID
		kind, title    string
		product, store *uuid.UUID
		notes          *string
	}
	all := []x{}
	for rows.Next() {
		var a x
		if err := rows.Scan(&a.id, &a.kind, &a.title, &a.product, &a.store, &a.notes); err != nil {
			rows.Close()
			return Job{}, err
		}
		all = append(all, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Job{}, err
	}
	for _, a := range all {
		kind := "service"
		if a.kind == "part" {
			kind = "part"
		}
		if _, err = tx.Exec(ctx, `INSERT INTO vehicle_notebook_entries(
			vehicle_id,actor_user_id,actor_role,actor_name,kind,title,mileage,occurred_on,notes,owner_reported,product_id,source_store_id,workshop_job_id,workshop_job_item_id)
			VALUES($1,$2,'mechanic',$3,$4,$5,$6,CURRENT_DATE,$7,false,$8,$9,$10,$11)`, vehicleID, mechanicID, provider, kind, a.title, mileage, a.notes, a.product, a.store, jobID, a.id); err != nil {
			return Job{}, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE workshop_jobs SET status='completed',completed_at=now(),updated_at=now() WHERE id=$1`, jobID); err != nil {
		return Job{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE vehicle_notebooks SET updated_at=now() WHERE id=$1`, vehicleID); err != nil {
		return Job{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return s.GetJob(ctx, mechanicID, jobID)
}

func (s *Service) Summary(ctx context.Context, mechanicID uuid.UUID) (Summary, error) {
	var out Summary
	if err := s.db.QueryRow(ctx, `SELECT
		COUNT(*) FILTER (WHERE status='open'),
		COUNT(*) FILTER (WHERE status='completed' AND completed_at>=now()-interval '30 days'),
		COALESCE(SUM(due_amount) FILTER (WHERE status<>'cancelled'),0)::bigint,
		COALESCE(SUM(total_amount) FILTER (WHERE status='completed' AND completed_at>=now()-interval '30 days'),0)::bigint
		FROM workshop_jobs WHERE mechanic_user_id=$1`, mechanicID).Scan(&out.OpenJobs, &out.Completed30Days, &out.ReceivableAmount, &out.Revenue30Days); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COUNT(DISTINCT vehicle_id) FROM workshop_jobs WHERE mechanic_user_id=$1`, mechanicID).Scan(&out.VehicleCount); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(i.line_total),0)::bigint FROM workshop_job_items i JOIN workshop_jobs j ON j.id=i.job_id
		WHERE j.mechanic_user_id=$1 AND i.source_store_id IS NOT NULL AND i.created_at>=now()-interval '30 days'`, mechanicID).Scan(&out.NetworkPartAmount); err != nil {
		return out, err
	}
	return out, nil
}

func recalcTx(ctx context.Context, tx pgx.Tx, jobID uuid.UUID) error {
	var labor, parts, total, paid int64
	if err := tx.QueryRow(ctx, `SELECT
		COALESCE(SUM(line_total) FILTER (WHERE item_type IN ('labor','service')),0)::bigint,
		COALESCE(SUM(line_total) FILTER (WHERE item_type='part'),0)::bigint,
		COALESCE(SUM(line_total),0)::bigint
		FROM workshop_job_items WHERE job_id=$1`, jobID).Scan(&labor, &parts, &total); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(amount),0)::bigint FROM workshop_job_payments WHERE job_id=$1`, jobID).Scan(&paid); err != nil {
		return err
	}
	if paid > total {
		return errors.New("recorded payments exceed job total")
	}
	_, err := tx.Exec(ctx, `UPDATE workshop_jobs SET labor_amount=$2,parts_amount=$3,total_amount=$4,paid_amount=$5,due_amount=$4-$5,updated_at=now() WHERE id=$1`, jobID, labor, parts, total, paid)
	return err
}

const jobSelect = `SELECT j.id,j.vehicle_id,v.public_token,
	trim(concat_ws(' ',v.make,v.model,v.trim)),COALESCE(v.plate,''),COALESCE(j.customer_name,''),COALESCE(j.customer_phone,''),j.mileage,
	COALESCE(j.complaint,''),COALESCE(j.diagnosis,''),j.status,j.labor_amount,j.parts_amount,j.total_amount,j.paid_amount,j.due_amount,j.opened_at,j.completed_at,j.updated_at
	FROM workshop_jobs j JOIN vehicle_notebooks v ON v.id=j.vehicle_id`

type scanner interface{ Scan(...any) error }

func scanJob(row scanner) (Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.VehicleID, &j.PublicToken, &j.VehicleLabel, &j.Plate, &j.CustomerName, &j.CustomerPhone, &j.Mileage, &j.Complaint, &j.Diagnosis, &j.Status, &j.LaborAmount, &j.PartsAmount, &j.TotalAmount, &j.PaidAmount, &j.DueAmount, &j.OpenedAt, &j.CompletedAt, &j.UpdatedAt)
	return j, err
}

func (s *Service) items(ctx context.Context, jobID uuid.UUID) ([]Item, error) {
	rows, err := s.db.Query(ctx, `SELECT i.id,i.item_type,i.title,i.product_id,i.source_store_id,COALESCE(st.name,''),i.reservation_id,i.qty::float8,i.unit_price,i.line_total,COALESCE(i.notes,''),i.created_at
		FROM workshop_job_items i LEFT JOIN stores st ON st.id=i.source_store_id WHERE i.job_id=$1 ORDER BY i.created_at,i.id`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var x Item
		if err := rows.Scan(&x.ID, &x.ItemType, &x.Title, &x.ProductID, &x.SourceStoreID, &x.SourceStore, &x.ReservationID, &x.Qty, &x.UnitPrice, &x.LineTotal, &x.Notes, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) payments(ctx context.Context, jobID uuid.UUID) ([]Payment, error) {
	rows, err := s.db.Query(ctx, `SELECT id,method,amount,COALESCE(note,''),created_at FROM workshop_job_payments WHERE job_id=$1 ORDER BY created_at DESC,id DESC`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Payment{}
	for rows.Next() {
		var x Payment
		if err := rows.Scan(&x.ID, &x.Method, &x.Amount, &x.Note, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
