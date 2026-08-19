package documents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

var validKinds = map[string]bool{
	"sales_invoice": true, "receipt_thermal": true, "quotation": true,
	"purchase_invoice": true, "sales_return": true, "payment_receipt": true, "barcode_label": true,
}

type Template struct {
	ID        uuid.UUID       `json:"id"`
	Kind      string          `json:"kind"`
	Name      string          `json:"name"`
	PaperSize string          `json:"paper_size"`
	IsDefault bool            `json:"is_default"`
	Active    bool            `json:"active"`
	Settings  json.RawMessage `json:"settings"`
}

type TemplateInput struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	PaperSize string         `json:"paper_size"`
	IsDefault bool           `json:"is_default"`
	Active    bool           `json:"active"`
	Settings  map[string]any `json:"settings"`
}

type LabelItem struct {
	ProductID     uuid.UUID `json:"product_id"`
	ProductTitle  string    `json:"product_title"`
	SKU           string    `json:"sku,omitempty"`
	Brand         string    `json:"brand,omitempty"`
	OEMCode       string    `json:"oem_code,omitempty"`
	ProductUnitID uuid.UUID `json:"product_unit_id"`
	UnitCode      string    `json:"unit_code"`
	UnitName      string    `json:"unit_name"`
	FactorToBase  float64   `json:"factor_to_base"`
	Barcode       string    `json:"barcode,omitempty"`
	Price         int64     `json:"price"`
}

func normalizeInput(in TemplateInput) (TemplateInput, []byte, error) {
	in.Kind = strings.TrimSpace(in.Kind)
	in.Name = strings.TrimSpace(in.Name)
	in.PaperSize = strings.TrimSpace(in.PaperSize)
	if !validKinds[in.Kind] {
		return in, nil, errors.New("invalid document template kind")
	}
	if in.Name == "" || len([]rune(in.Name)) > 120 {
		return in, nil, errors.New("template name is required and must be at most 120 characters")
	}
	if in.PaperSize == "" || len(in.PaperSize) > 40 {
		return in, nil, errors.New("paper_size is required")
	}
	if in.Settings == nil {
		in.Settings = map[string]any{}
	}
	settings, err := json.Marshal(in.Settings)
	if err != nil || len(settings) > 64<<10 {
		return in, nil, errors.New("template settings are invalid or too large")
	}
	return in, settings, nil
}

func (s *Service) List(ctx context.Context, tenantID, storeID uuid.UUID, kind string) ([]Template, error) {
	kind = strings.TrimSpace(kind)
	if kind != "" && !validKinds[kind] {
		return nil, errors.New("invalid document template kind")
	}
	rows, err := s.db.Query(ctx, `SELECT id,kind,name,paper_size,is_default,active,settings FROM document_templates WHERE tenant_id=$1 AND store_id=$2 AND ($3='' OR kind=$3) ORDER BY kind,is_default DESC,active DESC,name,id`, tenantID, storeID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Template{}
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.ID, &t.Kind, &t.Name, &t.PaperSize, &t.IsDefault, &t.Active, &t.Settings); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Service) Create(ctx context.Context, tenantID, storeID uuid.UUID, input TemplateInput) (Template, error) {
	in, settings, err := normalizeInput(input)
	if err != nil {
		return Template{}, err
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)
	if in.IsDefault {
		if _, err = tx.Exec(ctx, `UPDATE document_templates SET is_default=false,updated_at=now() WHERE tenant_id=$1 AND store_id=$2 AND kind=$3 AND is_default`, tenantID, storeID, in.Kind); err != nil {
			return Template{}, err
		}
	}
	var out Template
	err = tx.QueryRow(ctx, `INSERT INTO document_templates(tenant_id,store_id,kind,name,paper_size,is_default,active,settings) VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb) RETURNING id,kind,name,paper_size,is_default,active,settings`, tenantID, storeID, in.Kind, in.Name, in.PaperSize, in.IsDefault, in.Active, settings).Scan(&out.ID, &out.Kind, &out.Name, &out.PaperSize, &out.IsDefault, &out.Active, &out.Settings)
	if err != nil {
		return Template{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, tenantID, storeID, id uuid.UUID, input TemplateInput) (Template, error) {
	in, settings, err := normalizeInput(input)
	if err != nil {
		return Template{}, err
	}
	if !in.Active && in.IsDefault {
		return Template{}, errors.New("default template must stay active")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)
	var oldKind string
	if err = tx.QueryRow(ctx, `SELECT kind FROM document_templates WHERE id=$1 AND tenant_id=$2 AND store_id=$3 FOR UPDATE`, id, tenantID, storeID).Scan(&oldKind); errors.Is(err, pgx.ErrNoRows) {
		return Template{}, errors.New("template not found")
	} else if err != nil {
		return Template{}, err
	}
	if oldKind != in.Kind {
		return Template{}, errors.New("template kind cannot be changed")
	}
	if in.IsDefault {
		if _, err = tx.Exec(ctx, `UPDATE document_templates SET is_default=false,updated_at=now() WHERE tenant_id=$1 AND store_id=$2 AND kind=$3 AND id<>$4 AND is_default`, tenantID, storeID, in.Kind, id); err != nil {
			return Template{}, err
		}
	}
	var out Template
	err = tx.QueryRow(ctx, `UPDATE document_templates SET name=$4,paper_size=$5,is_default=$6,active=$7,settings=$8::jsonb,updated_at=now() WHERE id=$3 AND tenant_id=$1 AND store_id=$2 RETURNING id,kind,name,paper_size,is_default,active,settings`, tenantID, storeID, id, in.Name, in.PaperSize, in.IsDefault, in.Active, settings).Scan(&out.ID, &out.Kind, &out.Name, &out.PaperSize, &out.IsDefault, &out.Active, &out.Settings)
	if err != nil {
		return Template{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, tenantID, storeID, id uuid.UUID) error {
	var isDefault bool
	err := s.db.QueryRow(ctx, `SELECT is_default FROM document_templates WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, id, tenantID, storeID).Scan(&isDefault)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("template not found")
	}
	if err != nil {
		return err
	}
	if isDefault {
		return errors.New("default template cannot be deleted; choose another default first")
	}
	cmd, err := s.db.Exec(ctx, `DELETE FROM document_templates WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, id, tenantID, storeID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("template not found")
	}
	return nil
}

func (s *Service) LabelCatalog(ctx context.Context, tenantID, storeID uuid.UUID, q string, limit int) ([]LabelItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	pat := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"
	rows, err := s.db.Query(ctx, `
		WITH default_list AS (
		  SELECT id FROM price_lists WHERE tenant_id=$1 AND store_id=$2 AND is_default AND active ORDER BY created_at LIMIT 1
		), base_price AS (
		  SELECT ppb.product_id,ppb.unit_price
		  FROM product_price_breaks ppb JOIN product_units pu ON pu.id=ppb.product_unit_id AND pu.is_base
		  WHERE ppb.tenant_id=$1 AND ppb.store_id=$2 AND ppb.price_list_id=(SELECT id FROM default_list) AND ppb.min_qty=1
		)
		SELECT p.id,p.title,COALESCE(p.sku,''),COALESCE(p.brand,''),COALESCE(p.oem_code,''),pu.id,pu.code,pu.name,pu.factor_to_base::float8,COALESCE(pu.barcode,''),
		       COALESCE(ppb.unit_price,ROUND(bp.unit_price::numeric*pu.factor_to_base)::bigint,0)
		FROM products p JOIN product_units pu ON pu.tenant_id=p.tenant_id AND pu.product_id=p.id AND pu.active AND pu.allow_sale
		LEFT JOIN product_price_breaks ppb ON ppb.tenant_id=$1 AND ppb.store_id=$2 AND ppb.product_id=p.id AND ppb.product_unit_id=pu.id AND ppb.price_list_id=(SELECT id FROM default_list) AND ppb.min_qty=1
		LEFT JOIN base_price bp ON bp.product_id=p.id
		WHERE p.tenant_id=$1 AND p.deleted_at IS NULL AND p.active AND ($3='%%' OR lower(p.title) LIKE $3 OR lower(COALESCE(p.sku,'')) LIKE $3 OR lower(COALESCE(p.brand,'')) LIKE $3 OR lower(COALESCE(p.oem_code,'')) LIKE $3 OR lower(COALESCE(pu.barcode,'')) LIKE $3)
		ORDER BY lower(p.title),pu.is_base DESC,pu.factor_to_base,pu.name LIMIT $4`, tenantID, storeID, pat, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LabelItem{}
	for rows.Next() {
		var x LabelItem
		if err := rows.Scan(&x.ProductID, &x.ProductTitle, &x.SKU, &x.Brand, &x.OEMCode, &x.ProductUnitID, &x.UnitCode, &x.UnitName, &x.FactorToBase, &x.Barcode, &x.Price); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
