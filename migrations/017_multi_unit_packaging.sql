BEGIN;

ALTER TABLE products
  ADD COLUMN IF NOT EXISTS allow_fractional_base_qty boolean NOT NULL DEFAULT false;

CREATE TABLE product_units (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  code text NOT NULL,
  name text NOT NULL,
  factor_to_base numeric(18,6) NOT NULL CHECK (factor_to_base > 0),
  barcode text,
  is_base boolean NOT NULL DEFAULT false,
  allow_sale boolean NOT NULL DEFAULT true,
  allow_purchase boolean NOT NULL DEFAULT true,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (NOT is_base OR factor_to_base = 1),
  UNIQUE (tenant_id, product_id, code)
);
CREATE UNIQUE INDEX ux_product_units_one_base ON product_units(tenant_id,product_id) WHERE is_base;
CREATE UNIQUE INDEX ux_product_units_tenant_barcode ON product_units(tenant_id,barcode) WHERE barcode IS NOT NULL AND btrim(barcode)<>'' AND active;
CREATE INDEX idx_product_units_product ON product_units(tenant_id,product_id,active,is_base DESC,name);

-- Every existing product receives a canonical base unit and keeps its old barcode.
INSERT INTO product_units(tenant_id,product_id,code,name,factor_to_base,barcode,is_base,allow_sale,allow_purchase,active)
SELECT p.tenant_id,p.id,
       COALESCE(NULLIF(btrim(p.unit),''),'pcs'),
       CASE COALESCE(NULLIF(btrim(p.unit),''),'pcs')
         WHEN 'pcs' THEN 'عدد'
         WHEN 'pair' THEN 'جفت'
         WHEN 'set' THEN 'دست'
         WHEN 'pack' THEN 'بسته'
         WHEN 'carton' THEN 'کارتن'
         ELSE COALESCE(NULLIF(btrim(p.unit),''),'واحد')
       END,
       1,p.barcode,true,true,true,true
FROM products p
WHERE NOT EXISTS (
  SELECT 1 FROM product_units pu WHERE pu.tenant_id=p.tenant_id AND pu.product_id=p.id AND pu.is_base
);

ALTER TABLE sale_items
  ADD COLUMN IF NOT EXISTS product_unit_id uuid REFERENCES product_units(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS commercial_qty numeric(18,3),
  ADD COLUMN IF NOT EXISTS commercial_unit_code text,
  ADD COLUMN IF NOT EXISTS commercial_unit_name text,
  ADD COLUMN IF NOT EXISTS conversion_factor numeric(18,6) NOT NULL DEFAULT 1;

UPDATE sale_items si SET
  product_unit_id=pu.id,
  commercial_qty=si.qty,
  commercial_unit_code=pu.code,
  commercial_unit_name=pu.name,
  conversion_factor=1
FROM product_units pu
WHERE pu.tenant_id=si.tenant_id AND pu.product_id=si.product_id AND pu.is_base
  AND si.product_unit_id IS NULL;

ALTER TABLE purchase_items
  ADD COLUMN IF NOT EXISTS product_unit_id uuid REFERENCES product_units(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS commercial_qty numeric(18,3),
  ADD COLUMN IF NOT EXISTS commercial_unit_code text,
  ADD COLUMN IF NOT EXISTS commercial_unit_name text,
  ADD COLUMN IF NOT EXISTS conversion_factor numeric(18,6) NOT NULL DEFAULT 1;

UPDATE purchase_items pi SET
  product_unit_id=pu.id,
  commercial_qty=pi.qty,
  commercial_unit_code=pu.code,
  commercial_unit_name=pu.name,
  conversion_factor=1
FROM product_units pu
WHERE pu.tenant_id=pi.tenant_id AND pu.product_id=pi.product_id AND pu.is_base
  AND pi.product_unit_id IS NULL;

-- Price breaks are packaging-aware. Existing prices belong to the base unit.
ALTER TABLE product_price_breaks ADD COLUMN IF NOT EXISTS product_unit_id uuid REFERENCES product_units(id) ON DELETE CASCADE;
UPDATE product_price_breaks ppb SET product_unit_id=pu.id
FROM product_units pu
WHERE pu.tenant_id=ppb.tenant_id AND pu.product_id=ppb.product_id AND pu.is_base
  AND ppb.product_unit_id IS NULL;
ALTER TABLE product_price_breaks ALTER COLUMN product_unit_id SET NOT NULL;
DO $$
DECLARE r record;
BEGIN
  FOR r IN
    SELECT conname FROM pg_constraint
    WHERE conrelid='product_price_breaks'::regclass AND contype='u'
  LOOP
    EXECUTE format('ALTER TABLE product_price_breaks DROP CONSTRAINT %I', r.conname);
  END LOOP;
END $$;
ALTER TABLE product_price_breaks
  ADD CONSTRAINT ux_product_price_breaks_unit UNIQUE (tenant_id,store_id,product_id,price_list_id,product_unit_id,min_qty);
CREATE INDEX IF NOT EXISTS idx_product_price_breaks_unit_lookup
  ON product_price_breaks(tenant_id,store_id,product_id,price_list_id,product_unit_id,min_qty DESC);

COMMIT;
