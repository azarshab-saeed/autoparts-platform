package tax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Service) GetSettings(ctx context.Context, tenantID, storeID uuid.UUID) (Settings, error) {
	return loadSettings(ctx, s.db, tenantID, storeID)
}

func loadSettings(ctx context.Context, q rowQuerier, tenantID, storeID uuid.UUID) (Settings, error) {
	var out Settings
	err := q.QueryRow(ctx, `
      SELECT COALESCE(tp.legal_name,''),COALESCE(tp.national_id,''),COALESCE(tp.economic_code,''),COALESCE(tp.registration_number,''),
             COALESCE(tp.postal_code,''),COALESCE(tp.province,''),COALESCE(tp.city,''),COALESCE(tp.address,''),COALESCE(tp.phone,''),
             st.tax_enabled,st.tax_on_normal_sales,st.calculation_mode,st.default_invoice_mode,COALESCE(st.default_tax_code,''),st.official_series,st.next_official_number,st.invoice_number_width
      FROM store_tax_settings st
      LEFT JOIN tenant_tax_profiles tp ON tp.tenant_id=st.tenant_id
      WHERE st.tenant_id=$1 AND st.store_id=$2`, tenantID, storeID).Scan(
		&out.LegalName, &out.NationalID, &out.EconomicCode, &out.RegistrationNumber, &out.PostalCode, &out.Province, &out.City, &out.Address, &out.Phone,
		&out.TaxEnabled, &out.TaxOnNormalSales, &out.CalculationMode, &out.DefaultInvoiceMode, &out.DefaultTaxCode, &out.OfficialSeries, &out.NextOfficialNumber, &out.InvoiceNumberWidth,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, errors.New("tax settings are not provisioned for authenticated store")
	}
	return out, err
}

func (s *Service) UpdateSettings(ctx context.Context, tenantID, storeID uuid.UUID, in Settings) (Settings, error) {
	in.LegalName = strings.TrimSpace(in.LegalName)
	in.NationalID = strings.TrimSpace(in.NationalID)
	in.EconomicCode = strings.TrimSpace(in.EconomicCode)
	in.RegistrationNumber = strings.TrimSpace(in.RegistrationNumber)
	in.PostalCode = strings.TrimSpace(in.PostalCode)
	in.Province = strings.TrimSpace(in.Province)
	in.City = strings.TrimSpace(in.City)
	in.Address = strings.TrimSpace(in.Address)
	in.Phone = strings.TrimSpace(in.Phone)
	in.DefaultTaxCode = strings.TrimSpace(in.DefaultTaxCode)
	in.OfficialSeries = strings.TrimSpace(in.OfficialSeries)
	if in.CalculationMode != "exclusive" && in.CalculationMode != "inclusive" {
		return Settings{}, errors.New("calculation_mode must be exclusive or inclusive")
	}
	if in.DefaultInvoiceMode != "normal" && in.DefaultInvoiceMode != "official" {
		return Settings{}, errors.New("default_invoice_mode must be normal or official")
	}
	if in.OfficialSeries == "" || len(in.OfficialSeries) > 20 {
		return Settings{}, errors.New("official_series is required and must be at most 20 characters")
	}
	if in.InvoiceNumberWidth < 1 || in.InvoiceNumberWidth > 12 {
		return Settings{}, errors.New("invoice_number_width must be between 1 and 12")
	}
	if in.NextOfficialNumber < 1 {
		return Settings{}, errors.New("next_official_number must be positive")
	}
	if in.TaxEnabled && in.DefaultTaxCode != "" {
		var exists bool
		if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tax_rates WHERE tenant_id=$1 AND store_id=$2 AND code=$3)`, tenantID, storeID, in.DefaultTaxCode).Scan(&exists); err != nil {
			return Settings{}, err
		}
		if !exists {
			return Settings{}, errors.New("default_tax_code has no configured rate versions")
		}
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Settings{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
      INSERT INTO tenant_tax_profiles(tenant_id,legal_name,national_id,economic_code,registration_number,postal_code,province,city,address,phone,updated_at)
      VALUES($1,NULLIF($2,''),NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),now())
      ON CONFLICT(tenant_id) DO UPDATE SET legal_name=EXCLUDED.legal_name,national_id=EXCLUDED.national_id,economic_code=EXCLUDED.economic_code,
        registration_number=EXCLUDED.registration_number,postal_code=EXCLUDED.postal_code,province=EXCLUDED.province,city=EXCLUDED.city,address=EXCLUDED.address,phone=EXCLUDED.phone,updated_at=now()`,
		tenantID, in.LegalName, in.NationalID, in.EconomicCode, in.RegistrationNumber, in.PostalCode, in.Province, in.City, in.Address, in.Phone)
	if err != nil {
		return Settings{}, err
	}
	_, err = tx.Exec(ctx, `
      UPDATE store_tax_settings SET tax_enabled=$3,tax_on_normal_sales=$4,calculation_mode=$5,default_invoice_mode=$6,default_tax_code=NULLIF($7,''),
        official_series=$8,invoice_number_width=$9,next_official_number=GREATEST(next_official_number,$10),updated_at=now() WHERE tenant_id=$1 AND store_id=$2`,
		tenantID, storeID, in.TaxEnabled, in.TaxOnNormalSales, in.CalculationMode, in.DefaultInvoiceMode, in.DefaultTaxCode, in.OfficialSeries, in.InvoiceNumberWidth, in.NextOfficialNumber)
	if err != nil {
		return Settings{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Settings{}, err
	}
	return s.GetSettings(ctx, tenantID, storeID)
}

func (s *Service) ListRates(ctx context.Context, tenantID, storeID uuid.UUID) ([]Rate, error) {
	rows, err := s.db.Query(ctx, `SELECT id,code,name,category,rate_bps,effective_from,effective_to,COALESCE(exemption_reason,''),active FROM tax_rates WHERE tenant_id=$1 AND store_id=$2 ORDER BY code,effective_from DESC`, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rate{}
	for rows.Next() {
		var r Rate
		var from time.Time
		var to *time.Time
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.Category, &r.RateBPS, &from, &to, &r.ExemptionReason, &r.Active); err != nil {
			return nil, err
		}
		r.EffectiveFrom = from.Format("2006-01-02")
		if to != nil {
			r.EffectiveTo = to.Format("2006-01-02")
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) UpsertRate(ctx context.Context, tenantID, storeID uuid.UUID, in UpsertRate) (Rate, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	in.Category = strings.TrimSpace(in.Category)
	in.ExemptionReason = strings.TrimSpace(in.ExemptionReason)
	if in.Code == "" || in.Name == "" {
		return Rate{}, errors.New("tax rate code and name are required")
	}
	if in.Category != "taxable" && in.Category != "exempt" && in.Category != "non_taxable" {
		return Rate{}, errors.New("category must be taxable, exempt, or non_taxable")
	}
	if in.RateBPS < 0 || in.RateBPS > 10000 {
		return Rate{}, errors.New("rate_bps must be between 0 and 10000")
	}
	if in.Category != "taxable" && in.RateBPS != 0 {
		return Rate{}, errors.New("exempt and non_taxable rates must have rate_bps=0")
	}
	from, err := time.Parse("2006-01-02", strings.TrimSpace(in.EffectiveFrom))
	if err != nil {
		return Rate{}, errors.New("effective_from must be YYYY-MM-DD")
	}
	var to *time.Time
	if strings.TrimSpace(in.EffectiveTo) != "" {
		v, e := time.Parse("2006-01-02", strings.TrimSpace(in.EffectiveTo))
		if e != nil {
			return Rate{}, errors.New("effective_to must be YYYY-MM-DD")
		}
		if v.Before(from) {
			return Rate{}, errors.New("effective_to cannot be before effective_from")
		}
		to = &v
	}
	if !in.Active {
		// Explicit inactive versions are supported; the UI defaults this field to true.
	}
	var id uuid.UUID
	err = s.db.QueryRow(ctx, `
      INSERT INTO tax_rates(tenant_id,store_id,code,name,category,rate_bps,effective_from,effective_to,exemption_reason,active)
      VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10)
      ON CONFLICT(tenant_id,store_id,code,effective_from) DO UPDATE SET name=EXCLUDED.name,category=EXCLUDED.category,rate_bps=EXCLUDED.rate_bps,
        effective_to=EXCLUDED.effective_to,exemption_reason=EXCLUDED.exemption_reason,active=EXCLUDED.active,updated_at=now()
      RETURNING id`, tenantID, storeID, in.Code, in.Name, in.Category, in.RateBPS, from, to, in.ExemptionReason, in.Active).Scan(&id)
	if err != nil {
		return Rate{}, err
	}
	items, err := s.ListRates(ctx, tenantID, storeID)
	if err != nil {
		return Rate{}, err
	}
	for _, r := range items {
		if r.ID == id {
			return r, nil
		}
	}
	return Rate{}, errors.New("saved tax rate could not be reloaded")
}

func (s *Service) ListProductTaxProfiles(ctx context.Context, tenantID, storeID uuid.UUID, query string, limit int, at time.Time) ([]ProductTaxRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	settings, err := s.GetSettings(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	like := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := s.db.Query(ctx, `
      SELECT p.id,p.title,COALESCE(p.sku,''),COALESCE(ptp.tax_code,''),COALESCE(ptp.tax_code,$4) AS effective_code,
             COALESCE((SELECT tr.name FROM tax_rates tr WHERE tr.tenant_id=$1 AND tr.store_id=$2 AND tr.code=COALESCE(ptp.tax_code,$4) AND tr.active AND tr.effective_from <= $5::date AND (tr.effective_to IS NULL OR tr.effective_to >= $5::date) ORDER BY tr.effective_from DESC LIMIT 1),''),
             COALESCE((SELECT tr.category FROM tax_rates tr WHERE tr.tenant_id=$1 AND tr.store_id=$2 AND tr.code=COALESCE(ptp.tax_code,$4) AND tr.active AND tr.effective_from <= $5::date AND (tr.effective_to IS NULL OR tr.effective_to >= $5::date) ORDER BY tr.effective_from DESC LIMIT 1),''),
             COALESCE((SELECT tr.rate_bps FROM tax_rates tr WHERE tr.tenant_id=$1 AND tr.store_id=$2 AND tr.code=COALESCE(ptp.tax_code,$4) AND tr.active AND tr.effective_from <= $5::date AND (tr.effective_to IS NULL OR tr.effective_to >= $5::date) ORDER BY tr.effective_from DESC LIMIT 1),0)
      FROM products p LEFT JOIN product_tax_profiles ptp ON ptp.product_id=p.id AND ptp.tenant_id=$1 AND ptp.store_id=$2
      WHERE p.tenant_id=$1 AND p.deleted_at IS NULL AND ($3='%%' OR lower(p.title) LIKE $3 OR lower(COALESCE(p.sku,'')) LIKE $3 OR lower(COALESCE(p.oem_code,'')) LIKE $3)
      ORDER BY p.title,p.id LIMIT $6`, tenantID, storeID, like, settings.DefaultTaxCode, at, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProductTaxRow{}
	for rows.Next() {
		var x ProductTaxRow
		if err := rows.Scan(&x.ProductID, &x.Title, &x.SKU, &x.ExplicitTaxCode, &x.EffectiveTaxCode, &x.RateName, &x.Category, &x.RateBPS); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) AssignProductTaxCode(ctx context.Context, tenantID, storeID, productID uuid.UUID, code string) (ProductTaxProfile, error) {
	code = strings.TrimSpace(code)
	var productOK bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL)`, productID, tenantID).Scan(&productOK); err != nil {
		return ProductTaxProfile{}, err
	}
	if !productOK {
		return ProductTaxProfile{}, errors.New("product not found for authenticated tenant")
	}
	if code == "" {
		_, err := s.db.Exec(ctx, `DELETE FROM product_tax_profiles WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3`, tenantID, storeID, productID)
		return ProductTaxProfile{ProductID: productID}, err
	}
	var rateOK bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tax_rates WHERE tenant_id=$1 AND store_id=$2 AND code=$3)`, tenantID, storeID, code).Scan(&rateOK); err != nil {
		return ProductTaxProfile{}, err
	}
	if !rateOK {
		return ProductTaxProfile{}, errors.New("tax code has no configured rate versions")
	}
	_, err := s.db.Exec(ctx, `INSERT INTO product_tax_profiles(tenant_id,store_id,product_id,tax_code) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,store_id,product_id) DO UPDATE SET tax_code=EXCLUDED.tax_code,updated_at=now()`, tenantID, storeID, productID, code)
	return ProductTaxProfile{ProductID: productID, TaxCode: code}, err
}

func (s *Service) UpdateCustomerIdentity(ctx context.Context, tenantID, storeID, customerID uuid.UUID, in CustomerIdentity) (CustomerIdentity, error) {
	in.LegalType = strings.TrimSpace(in.LegalType)
	if in.LegalType != "" && in.LegalType != "natural" && in.LegalType != "legal" && in.LegalType != "other" {
		return CustomerIdentity{}, errors.New("legal_type must be natural, legal, other, or empty")
	}
	var out CustomerIdentity
	err := s.db.QueryRow(ctx, `
      UPDATE customers SET legal_type=NULLIF($4,''),national_id=NULLIF($5,''),economic_code=NULLIF($6,''),registration_number=NULLIF($7,''),postal_code=NULLIF($8,''),address=NULLIF($9,''),updated_at=now()
      WHERE id=$3 AND tenant_id=$1 AND store_id=$2 AND deleted_at IS NULL
      RETURNING id,name,COALESCE(legal_type,''),COALESCE(national_id,''),COALESCE(economic_code,''),COALESCE(registration_number,''),COALESCE(postal_code,''),COALESCE(address,'')`,
		tenantID, storeID, customerID, in.LegalType, strings.TrimSpace(in.NationalID), strings.TrimSpace(in.EconomicCode), strings.TrimSpace(in.RegistrationNumber), strings.TrimSpace(in.PostalCode), strings.TrimSpace(in.Address)).Scan(
		&out.CustomerID, &out.Name, &out.LegalType, &out.NationalID, &out.EconomicCode, &out.RegistrationNumber, &out.PostalCode, &out.Address)
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerIdentity{}, errors.New("customer not found")
	}
	return out, err
}

func (s *Service) Quote(ctx context.Context, tenantID, storeID uuid.UUID, customerID *uuid.UUID, invoiceMode string, at time.Time, items []QuoteLineInput) (Quote, error) {
	return ResolveQuote(ctx, s.db, tenantID, storeID, customerID, invoiceMode, at, items)
}

func ResolveQuote(ctx context.Context, q rowQuerier, tenantID, storeID uuid.UUID, customerID *uuid.UUID, invoiceMode string, at time.Time, items []QuoteLineInput) (Quote, error) {
	settings, err := loadSettings(ctx, q, tenantID, storeID)
	if err != nil {
		return Quote{}, err
	}
	invoiceMode = strings.TrimSpace(invoiceMode)
	if invoiceMode == "" {
		invoiceMode = settings.DefaultInvoiceMode
	}
	if invoiceMode != "normal" && invoiceMode != "official" {
		return Quote{}, errors.New("invoice_mode must be normal or official")
	}
	out := Quote{InvoiceMode: invoiceMode, CalculationMode: settings.CalculationMode, Warnings: []string{}, Items: make([]QuoteLine, 0, len(items))}
	out.SellerSnapshot = sellerSnapshot(settings)
	out.SellerReady = strings.TrimSpace(settings.LegalName) != "" && strings.TrimSpace(settings.NationalID) != ""
	if !out.SellerReady && invoiceMode == "official" {
		out.Warnings = append(out.Warnings, "seller legal name and national id are required for an official invoice")
	}
	out.BuyerReady = true
	out.BuyerSnapshot = map[string]any{}
	if customerID != nil && *customerID != uuid.Nil {
		var name, legalType, nationalID, economicCode, reg, postal, address string
		err = q.QueryRow(ctx, `SELECT name,COALESCE(legal_type,''),COALESCE(national_id,''),COALESCE(economic_code,''),COALESCE(registration_number,''),COALESCE(postal_code,''),COALESCE(address,'') FROM customers WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND deleted_at IS NULL`, *customerID, tenantID, storeID).Scan(&name, &legalType, &nationalID, &economicCode, &reg, &postal, &address)
		if errors.Is(err, pgx.ErrNoRows) {
			return Quote{}, errors.New("customer not found for authenticated store")
		}
		if err != nil {
			return Quote{}, err
		}
		out.BuyerSnapshot = map[string]any{"customer_id": customerID.String(), "name": name, "legal_type": legalType, "national_id": nationalID, "economic_code": economicCode, "registration_number": reg, "postal_code": postal, "address": address}
		if invoiceMode == "official" && nationalID == "" {
			out.BuyerReady = false
			out.Warnings = append(out.Warnings, "selected customer has no national id; buyer identity will be incomplete")
		}
	}
	out.Applied = settings.TaxEnabled && (invoiceMode == "official" || settings.TaxOnNormalSales)
	for _, in := range items {
		if in.ProductID == uuid.Nil || in.Amount < 0 {
			return Quote{}, errors.New("invalid tax quote item")
		}
		line := QuoteLine{ProductID: in.ProductID, Category: "not_applied", TaxBaseAmount: in.Amount, TotalWithTax: in.Amount}
		if !out.Applied {
			out.NetAmount += in.Amount
			out.TotalAmount += in.Amount
			out.Items = append(out.Items, line)
			continue
		}
		var code string
		err = q.QueryRow(ctx, `SELECT COALESCE((SELECT tax_code FROM product_tax_profiles WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3),$4)`, tenantID, storeID, in.ProductID, settings.DefaultTaxCode).Scan(&code)
		if err != nil {
			return Quote{}, err
		}
		code = strings.TrimSpace(code)
		if code == "" {
			return Quote{}, fmt.Errorf("tax code is not configured for product %s", in.ProductID)
		}
		var name, category, exemption string
		var bps int
		err = q.QueryRow(ctx, `
          SELECT name,category,rate_bps,COALESCE(exemption_reason,'') FROM tax_rates
          WHERE tenant_id=$1 AND store_id=$2 AND code=$3 AND active AND effective_from <= $4::date AND (effective_to IS NULL OR effective_to >= $4::date)
          ORDER BY effective_from DESC LIMIT 1`, tenantID, storeID, code, at).Scan(&name, &category, &bps, &exemption)
		if errors.Is(err, pgx.ErrNoRows) {
			return Quote{}, fmt.Errorf("no effective tax rate version for code %s on %s", code, at.Format("2006-01-02"))
		}
		if err != nil {
			return Quote{}, err
		}
		base, taxAmount, total, e := CalculateLine(in.Amount, bps, category, settings.CalculationMode)
		if e != nil {
			return Quote{}, e
		}
		line.Category, line.TaxCode, line.TaxRateName, line.TaxRateBPS = category, code, name, bps
		line.TaxBaseAmount, line.TaxAmount, line.TotalWithTax, line.ExemptionReason = base, taxAmount, total, exemption
		out.NetAmount += base
		out.TaxAmount += taxAmount
		out.TotalAmount += total
		if category == "taxable" {
			out.TaxableAmount += base
		} else {
			out.ExemptAmount += base
		}
		out.Items = append(out.Items, line)
	}
	return out, nil
}

func sellerSnapshot(s Settings) map[string]any {
	return map[string]any{"legal_name": s.LegalName, "national_id": s.NationalID, "economic_code": s.EconomicCode, "registration_number": s.RegistrationNumber, "postal_code": s.PostalCode, "province": s.Province, "city": s.City, "address": s.Address, "phone": s.Phone}
}

func AllocateOfficialNumber(ctx context.Context, tx pgx.Tx, tenantID, storeID uuid.UUID) (series string, number int64, display string, err error) {
	var width int
	err = tx.QueryRow(ctx, `UPDATE store_tax_settings SET next_official_number=next_official_number+1,updated_at=now() WHERE tenant_id=$1 AND store_id=$2 RETURNING official_series,next_official_number-1,invoice_number_width`, tenantID, storeID).Scan(&series, &number, &width)
	if err != nil {
		return "", 0, "", err
	}
	series = strings.TrimSpace(series)
	if series == "" {
		return "", 0, "", errors.New("official invoice series is not configured")
	}
	return series, number, fmt.Sprintf("%s-%0*d", series, width, number), nil
}

func (s *Service) ListInvoices(ctx context.Context, tenantID, storeID uuid.UUID, from, to time.Time, mode string, limit, offset int) ([]InvoiceListItem, int, error) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "all"
	}
	if mode != "all" && mode != "normal" && mode != "official" {
		return nil, 0, errors.New("mode must be all, normal, or official")
	}
	rows, err := s.db.Query(ctx, `
      SELECT s.id,s.invoice_mode,s.invoice_state,COALESCE(s.invoice_number_display,''),s.customer_id,COALESCE(c.name,''),s.net_amount,s.tax_amount,s.total_amount,s.created_at,COUNT(*) OVER()::int
      FROM sales s LEFT JOIN customers c ON c.id=s.customer_id AND c.tenant_id=s.tenant_id AND c.store_id=s.store_id
      WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.created_at >= $3 AND s.created_at < $4 + interval '1 day' AND ($5='all' OR s.invoice_mode=$5)
      ORDER BY s.created_at DESC,s.id DESC LIMIT $6 OFFSET $7`, tenantID, storeID, from, to, mode, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []InvoiceListItem{}
	total := 0
	for rows.Next() {
		var x InvoiceListItem
		var count int
		if err := rows.Scan(&x.SaleID, &x.InvoiceMode, &x.InvoiceState, &x.InvoiceNumberDisplay, &x.CustomerID, &x.CustomerName, &x.NetAmount, &x.TaxAmount, &x.TotalAmount, &x.CreatedAt, &count); err != nil {
			return nil, 0, err
		}
		total = count
		out = append(out, x)
	}
	return out, total, rows.Err()
}

func (s *Service) PrintData(ctx context.Context, tenantID, storeID, saleID uuid.UUID) (PrintData, error) {
	var out PrintData
	var sellerRaw, buyerRaw []byte
	var issued *time.Time
	err := s.db.QueryRow(ctx, `
      SELECT id,invoice_mode,invoice_kind,invoice_state,COALESCE(invoice_number_display,''),invoice_issued_at,seller_snapshot,buyer_snapshot,tax_calculation_mode,
             gross_amount,discount_amount,net_amount,taxable_amount,exempt_amount,tax_amount,total_amount,paid_amount,due_amount
      FROM sales WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, saleID, tenantID, storeID).Scan(
		&out.SaleID, &out.InvoiceMode, &out.InvoiceKind, &out.InvoiceState, &out.InvoiceNumberDisplay, &issued, &sellerRaw, &buyerRaw, &out.CalculationMode,
		&out.GrossAmount, &out.DiscountAmount, &out.NetAmount, &out.TaxableAmount, &out.ExemptAmount, &out.TaxAmount, &out.TotalAmount, &out.PaidAmount, &out.DueAmount)
	if errors.Is(err, pgx.ErrNoRows) {
		return PrintData{}, errors.New("sale not found")
	}
	if err != nil {
		return PrintData{}, err
	}
	out.IssuedAt = issued
	out.Seller, out.Buyer = map[string]any{}, map[string]any{}
	_ = json.Unmarshal(sellerRaw, &out.Seller)
	_ = json.Unmarshal(buyerRaw, &out.Buyer)
	rows, err := s.db.Query(ctx, `
      SELECT si.product_id,p.title,COALESCE(si.commercial_unit_name,'عدد'),COALESCE(si.commercial_qty,si.qty)::float8,si.unit_price,
             si.tax_base_amount,si.tax_category,COALESCE(si.tax_code,''),COALESCE(si.tax_rate_name,''),si.tax_rate_bps,si.tax_amount,si.total_with_tax,COALESCE(si.tax_exemption_reason,'')
      FROM sale_items si JOIN products p ON p.id=si.product_id AND p.tenant_id=si.tenant_id
      WHERE si.tenant_id=$1 AND si.sale_id=$2 ORDER BY si.created_at,si.id`, tenantID, saleID)
	if err != nil {
		return PrintData{}, err
	}
	defer rows.Close()
	out.Items = []PrintLine{}
	for rows.Next() {
		var x PrintLine
		if err := rows.Scan(&x.ProductID, &x.Title, &x.UnitName, &x.Qty, &x.UnitPrice, &x.NetAmount, &x.TaxCategory, &x.TaxCode, &x.TaxRateName, &x.TaxRateBPS, &x.TaxAmount, &x.TotalWithTax, &x.ExemptionReason); err != nil {
			return PrintData{}, err
		}
		out.Items = append(out.Items, x)
	}
	return out, rows.Err()
}

func (s *Service) RequestInvoiceAction(ctx context.Context, tenantID, storeID, actorUserID, saleID uuid.UUID, actionType, reason string) (InvoiceAction, error) {
	actionType, reason = strings.TrimSpace(actionType), strings.TrimSpace(reason)
	if actionType != "correction" && actionType != "cancellation" {
		return InvoiceAction{}, errors.New("action_type must be correction or cancellation")
	}
	if reason == "" {
		return InvoiceAction{}, errors.New("reason is required")
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return InvoiceAction{}, err
	}
	defer tx.Rollback(ctx)
	var mode, state string
	if err = tx.QueryRow(ctx, `SELECT invoice_mode,invoice_state FROM sales WHERE id=$1 AND tenant_id=$2 AND store_id=$3 FOR UPDATE`, saleID, tenantID, storeID).Scan(&mode, &state); errors.Is(err, pgx.ErrNoRows) {
		return InvoiceAction{}, errors.New("sale not found")
	}
	if err != nil {
		return InvoiceAction{}, err
	}
	if mode != "official" {
		return InvoiceAction{}, errors.New("only official invoices can request correction or cancellation")
	}
	if state != "issued" {
		return InvoiceAction{}, fmt.Errorf("invoice action cannot be requested while invoice_state=%s", state)
	}
	var out InvoiceAction
	err = tx.QueryRow(ctx, `INSERT INTO official_invoice_actions(tenant_id,store_id,sale_id,action_type,reason,actor_user_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,sale_id,action_type,reason,status,actor_user_id,replacement_sale_id,created_at`, tenantID, storeID, saleID, actionType, reason, actorUserID).Scan(&out.ID, &out.SaleID, &out.ActionType, &out.Reason, &out.Status, &out.ActorUserID, &out.ReplacementSaleID, &out.CreatedAt)
	if err != nil {
		return InvoiceAction{}, err
	}
	newState := actionType + "_requested"
	if _, err = tx.Exec(ctx, `UPDATE sales SET invoice_state=$2 WHERE id=$1`, saleID, newState); err != nil {
		return InvoiceAction{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return InvoiceAction{}, err
	}
	return out, nil
}
