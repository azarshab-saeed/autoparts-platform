BEGIN;

CREATE TABLE document_templates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  store_id uuid NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('sales_invoice','receipt_thermal','quotation','purchase_invoice','sales_return','payment_receipt','barcode_label')),
  name text NOT NULL,
  paper_size text NOT NULL DEFAULT 'A4',
  is_default boolean NOT NULL DEFAULT false,
  active boolean NOT NULL DEFAULT true,
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id,store_id,kind,name)
);
CREATE INDEX idx_document_templates_list ON document_templates(tenant_id,store_id,kind,active,is_default DESC,name);
CREATE UNIQUE INDEX ux_document_templates_default ON document_templates(tenant_id,store_id,kind) WHERE is_default AND active;

INSERT INTO document_templates(tenant_id,store_id,kind,name,paper_size,is_default,settings)
SELECT tenant_id,id,'sales_invoice','فاکتور استاندارد A4','A4',true,
       '{"direction":"rtl","show_logo":true,"show_store_name":true,"show_store_address":true,"show_store_phone":true,"show_customer":true,"show_tax_identity":true,"show_sku":true,"show_oem":true,"show_brand":true,"show_barcode":false,"show_unit":true,"show_discount":true,"show_tax":true,"show_payment":true,"show_qr":true,"show_signature":true,"header_text":"","footer_text":"سپاس از خرید شما","terms_text":"کالای فروخته‌شده مطابق شرایط درج‌شده در فاکتور قابل مرجوعی است.","margin_mm":10,"font_scale":100}'::jsonb
FROM stores
ON CONFLICT DO NOTHING;

INSERT INTO document_templates(tenant_id,store_id,kind,name,paper_size,is_default,settings)
SELECT tenant_id,id,'receipt_thermal','رسید حرارتی ۸۰ میلی‌متر','80mm',true,
       '{"direction":"rtl","show_logo":false,"show_store_name":true,"show_store_address":false,"show_store_phone":true,"show_customer":true,"show_sku":false,"show_oem":false,"show_brand":false,"show_barcode":false,"show_unit":true,"show_discount":true,"show_tax":true,"show_payment":true,"show_qr":false,"show_signature":false,"header_text":"","footer_text":"سپاس از خرید شما","terms_text":"","margin_mm":3,"font_scale":90}'::jsonb
FROM stores
ON CONFLICT DO NOTHING;

INSERT INTO document_templates(tenant_id,store_id,kind,name,paper_size,is_default,settings)
SELECT tenant_id,id,'barcode_label','لیبل استاندارد کالا','50x30mm',true,
       '{"width_mm":50,"height_mm":30,"columns":1,"gap_mm":2,"padding_mm":2,"barcode_format":"auto","show_product_name":true,"show_sku":true,"show_oem":false,"show_brand":false,"show_price":true,"show_unit":true,"show_pack_qty":true,"show_store_name":false,"show_barcode_text":true,"show_print_date":false,"show_qr":false,"name_font_size":10,"price_font_size":12,"barcode_height_mm":10}'::jsonb
FROM stores
ON CONFLICT DO NOTHING;

ALTER TABLE sales
  ADD COLUMN IF NOT EXISTS document_template_id uuid REFERENCES document_templates(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS document_template_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb;

COMMIT;
