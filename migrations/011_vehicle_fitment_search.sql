BEGIN;

CREATE TABLE IF NOT EXISTS vehicle_makes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  normalized_name text NOT NULL,
  UNIQUE (normalized_name)
);

CREATE TABLE IF NOT EXISTS vehicle_models (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  make_id uuid NOT NULL REFERENCES vehicle_makes(id) ON DELETE CASCADE,
  name text NOT NULL,
  normalized_name text NOT NULL,
  UNIQUE (make_id, normalized_name)
);

CREATE TABLE IF NOT EXISTS vehicle_variants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  model_id uuid NOT NULL REFERENCES vehicle_models(id) ON DELETE CASCADE,
  name text NOT NULL,
  normalized_name text NOT NULL,
  engine_code text,
  year_from integer,
  year_to integer,
  active boolean NOT NULL DEFAULT true,
  CHECK (year_from IS NULL OR year_from BETWEEN 1200 AND 2200),
  CHECK (year_to IS NULL OR year_to BETWEEN 1200 AND 2200),
  CHECK (year_from IS NULL OR year_to IS NULL OR year_to >= year_from),
  UNIQUE (model_id, normalized_name, engine_code)
);

CREATE TABLE IF NOT EXISTS product_search_terms (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('alias','oem','equivalent')),
  term text NOT NULL,
  normalized_term text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, product_id, kind, normalized_term)
);

CREATE TABLE IF NOT EXISTS product_fitments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  vehicle_variant_id uuid NOT NULL REFERENCES vehicle_variants(id) ON DELETE CASCADE,
  year_from integer,
  year_to integer,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (year_from IS NULL OR year_from BETWEEN 1200 AND 2200),
  CHECK (year_to IS NULL OR year_to BETWEEN 1200 AND 2200),
  CHECK (year_from IS NULL OR year_to IS NULL OR year_to >= year_from),
  UNIQUE (tenant_id, product_id, vehicle_variant_id, year_from, year_to)
);

CREATE INDEX IF NOT EXISTS idx_product_search_terms_trgm
  ON product_search_terms USING gin (normalized_term gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_product_search_terms_product
  ON product_search_terms(tenant_id, product_id);
CREATE INDEX IF NOT EXISTS idx_product_fitments_variant
  ON product_fitments(vehicle_variant_id, tenant_id, product_id);
CREATE INDEX IF NOT EXISTS idx_product_fitments_product
  ON product_fitments(tenant_id, product_id);

-- Shared vehicle catalog for the first Iranian-market fitment slice.
INSERT INTO vehicle_makes(id,name,normalized_name) VALUES
 ('90000000-0000-0000-0000-000000000001','پژو','پژو'),
 ('90000000-0000-0000-0000-000000000002','ایران خودرو','ایران خودرو')
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,normalized_name=EXCLUDED.normalized_name;

INSERT INTO vehicle_models(id,make_id,name,normalized_name) VALUES
 ('91000000-0000-0000-0000-000000000001','90000000-0000-0000-0000-000000000001','206','206'),
 ('91000000-0000-0000-0000-000000000002','90000000-0000-0000-0000-000000000001','405','405'),
 ('91000000-0000-0000-0000-000000000003','90000000-0000-0000-0000-000000000002','سمند','سمند')
ON CONFLICT (id) DO UPDATE SET make_id=EXCLUDED.make_id,name=EXCLUDED.name,normalized_name=EXCLUDED.normalized_name;

INSERT INTO vehicle_variants(id,model_id,name,normalized_name,engine_code,year_from,year_to,active) VALUES
 ('92000000-0000-0000-0000-000000000001','91000000-0000-0000-0000-000000000001','تیپ 5','تیپ 5','TU5',1380,1400,true),
 ('92000000-0000-0000-0000-000000000002','91000000-0000-0000-0000-000000000001','تیپ 2','تیپ 2','TU3',1380,1400,true),
 ('92000000-0000-0000-0000-000000000003','91000000-0000-0000-0000-000000000002','GLX','glx','XU7',1372,1400,true),
 ('92000000-0000-0000-0000-000000000004','91000000-0000-0000-0000-000000000003','LX EF7','lx ef7','EF7',1388,1402,true)
ON CONFLICT (id) DO UPDATE SET model_id=EXCLUDED.model_id,name=EXCLUDED.name,normalized_name=EXCLUDED.normalized_name,
 engine_code=EXCLUDED.engine_code,year_from=EXCLUDED.year_from,year_to=EXCLUDED.year_to,active=true;

COMMIT;
