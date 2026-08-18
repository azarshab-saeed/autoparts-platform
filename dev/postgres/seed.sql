BEGIN;

INSERT INTO tenants(id, name)
VALUES ('11111111-1111-1111-1111-111111111111', 'فروشگاه نمونه')
ON CONFLICT (id) DO NOTHING;

INSERT INTO stores(id, tenant_id, name, code, active)
VALUES ('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', 'یدکی رضایی', 'DEMO-001', true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO warehouses(id, tenant_id, store_id, name)
VALUES ('33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222', 'انبار اصلی')
ON CONFLICT (id) DO NOTHING;

INSERT INTO products(id, tenant_id, sku, title, brand, oem_code, unit, active, normalized_title)
VALUES
  ('55555555-5555-5555-5555-555555555551', '11111111-1111-1111-1111-111111111111', 'BRK-206-TXT', 'لنت جلو پژو ۲۰۶ تیپ ۵', 'Textar', '206-FB-01', 'pcs', true, 'لنت جلو پژو ۲۰۶ تیپ ۵'),
  ('55555555-5555-5555-5555-555555555552', '11111111-1111-1111-1111-111111111111', 'FLT-206-SRK', 'فیلتر روغن پژو ۲۰۶', 'سرکان', '206-OF-02', 'pcs', true, 'فیلتر روغن پژو ۲۰۶'),
  ('55555555-5555-5555-5555-555555555553', '11111111-1111-1111-1111-111111111111', 'WP-EF7-ISK', 'واترپمپ EF7', 'ایساکو', 'EF7-WP-03', 'pcs', true, 'واترپمپ ef7')
ON CONFLICT (id) DO NOTHING;

INSERT INTO inventory_balances(tenant_id, warehouse_id, product_id, on_hand, reserved, avg_unit_cost)
VALUES
  ('11111111-1111-1111-1111-111111111111', '33333333-3333-3333-3333-333333333333', '55555555-5555-5555-5555-555555555551', 12, 0, 1350000),
  ('11111111-1111-1111-1111-111111111111', '33333333-3333-3333-3333-333333333333', '55555555-5555-5555-5555-555555555552', 24, 0, 210000),
  ('11111111-1111-1111-1111-111111111111', '33333333-3333-3333-3333-333333333333', '55555555-5555-5555-5555-555555555553', 7, 0, 1180000)
ON CONFLICT (tenant_id, warehouse_id, product_id) DO NOTHING;

INSERT INTO customers(id, tenant_id, store_id, code, name, phone)
VALUES
  ('66666666-6666-6666-6666-666666666661', '11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222', 'C-001', 'تعمیرگاه احمدی', '09120000001'),
  ('66666666-6666-6666-6666-666666666662', '11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222', 'C-002', 'مکانیکی امیر', '09120000002')
ON CONFLICT (id) DO NOTHING;


INSERT INTO suppliers(id, tenant_id, store_id, code, name, phone, notes)
VALUES
  ('77777777-7777-7777-7777-777777777771', '11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222', 'SUP-001', 'پخش قطعات ایران', '02155551010', 'تأمین‌کننده نمونه'),
  ('77777777-7777-7777-7777-777777777772', '11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222', 'SUP-002', 'بازرگانی پارس یدک', '02155552020', 'تأمین‌کننده نمونه')
ON CONFLICT (id) DO NOTHING;

INSERT INTO inventory_reorder_points(tenant_id, warehouse_id, product_id, min_qty, target_qty)
VALUES
  ('11111111-1111-1111-1111-111111111111', '33333333-3333-3333-3333-333333333333', '55555555-5555-5555-5555-555555555551', 4, 10),
  ('11111111-1111-1111-1111-111111111111', '33333333-3333-3333-3333-333333333333', '55555555-5555-5555-5555-555555555553', 8, 15)
ON CONFLICT (tenant_id, warehouse_id, product_id)
DO UPDATE SET min_qty=EXCLUDED.min_qty, target_qty=EXCLUDED.target_qty, updated_at=now();

COMMIT;

-- Phase 7: demo network inventory. These shops intentionally live in separate
-- tenants to prove that network search is cross-tenant while internal APIs are not.
UPDATE stores SET
  network_enabled=true, public_address='تهران، خیابان آزادی، نبش خیابان بهبودی',
  public_phone='02166000001', city='تهران', latitude=35.700700, longitude=51.371600,
  network_updated_at=now()
WHERE id='22222222-2222-2222-2222-222222222222';

INSERT INTO tenants(id,name) VALUES
 ('11111111-1111-1111-1111-111111111112','فروشگاه شبکه نمونه ۲'),
 ('11111111-1111-1111-1111-111111111113','فروشگاه شبکه نمونه ۳')
ON CONFLICT(id) DO NOTHING;

INSERT INTO stores(id,tenant_id,name,code,active,network_enabled,public_address,public_phone,city,latitude,longitude)
VALUES
 ('22222222-2222-2222-2222-222222222223','11111111-1111-1111-1111-111111111112','یدکی مرکزی','NET-002',true,true,'تهران، خیابان جمهوری، بعد از اسکندری','02166990002','تهران',35.695900,51.391700),
 ('22222222-2222-2222-2222-222222222224','11111111-1111-1111-1111-111111111113','پارس یدک','NET-003',true,true,'تهران، ستارخان، خیابان حبیب‌الله','02166550003','تهران',35.721300,51.365000)
ON CONFLICT(id) DO UPDATE SET network_enabled=true,public_address=EXCLUDED.public_address,public_phone=EXCLUDED.public_phone,city=EXCLUDED.city,latitude=EXCLUDED.latitude,longitude=EXCLUDED.longitude,network_updated_at=now();

INSERT INTO warehouses(id,tenant_id,store_id,name) VALUES
 ('33333333-3333-3333-3333-333333333334','11111111-1111-1111-1111-111111111112','22222222-2222-2222-2222-222222222223','انبار اصلی'),
 ('33333333-3333-3333-3333-333333333335','11111111-1111-1111-1111-111111111113','22222222-2222-2222-2222-222222222224','انبار اصلی')
ON CONFLICT(id) DO NOTHING;

INSERT INTO products(id,tenant_id,sku,title,brand,oem_code,unit,active,normalized_title) VALUES
 ('55555555-5555-5555-5555-555555555561','11111111-1111-1111-1111-111111111112','BRK-206-TXT','لنت جلو پژو ۲۰۶ تیپ ۵','Textar','206-FB-01','pcs',true,'لنت جلو پژو ۲۰۶ تیپ ۵'),
 ('55555555-5555-5555-5555-555555555562','11111111-1111-1111-1111-111111111112','WP-EF7-ISK','واترپمپ EF7','ایساکو','EF7-WP-03','pcs',true,'واترپمپ ef7'),
 ('55555555-5555-5555-5555-555555555571','11111111-1111-1111-1111-111111111113','BRK-206-TXT','لنت جلو پژو ۲۰۶ تیپ ۵','Textar','206-FB-01','pcs',true,'لنت جلو پژو ۲۰۶ تیپ ۵'),
 ('55555555-5555-5555-5555-555555555572','11111111-1111-1111-1111-111111111113','WP-EF7-ISK','واترپمپ EF7','ایساکو','EF7-WP-03','pcs',true,'واترپمپ ef7')
ON CONFLICT(id) DO NOTHING;

INSERT INTO inventory_balances(tenant_id,warehouse_id,product_id,on_hand,reserved,avg_unit_cost) VALUES
 ('11111111-1111-1111-1111-111111111112','33333333-3333-3333-3333-333333333334','55555555-5555-5555-5555-555555555561',8,0,1310000),
 ('11111111-1111-1111-1111-111111111112','33333333-3333-3333-3333-333333333334','55555555-5555-5555-5555-555555555562',4,0,1150000),
 ('11111111-1111-1111-1111-111111111113','33333333-3333-3333-3333-333333333335','55555555-5555-5555-5555-555555555571',16,0,1330000),
 ('11111111-1111-1111-1111-111111111113','33333333-3333-3333-3333-333333333335','55555555-5555-5555-5555-555555555572',10,0,1170000)
ON CONFLICT(tenant_id,warehouse_id,product_id) DO NOTHING;

INSERT INTO store_product_offers(tenant_id,store_id,warehouse_id,product_id,selling_price,visible,allow_reservation,last_verified_at) VALUES
 ('11111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','33333333-3333-3333-3333-333333333333','55555555-5555-5555-5555-555555555551',1780000,true,true,now()),
 ('11111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','33333333-3333-3333-3333-333333333333','55555555-5555-5555-5555-555555555553',1590000,true,true,now()),
 ('11111111-1111-1111-1111-111111111112','22222222-2222-2222-2222-222222222223','33333333-3333-3333-3333-333333333334','55555555-5555-5555-5555-555555555561',1720000,true,false,now()),
 ('11111111-1111-1111-1111-111111111112','22222222-2222-2222-2222-222222222223','33333333-3333-3333-3333-333333333334','55555555-5555-5555-5555-555555555562',1540000,true,true,now()),
 ('11111111-1111-1111-1111-111111111113','22222222-2222-2222-2222-222222222224','33333333-3333-3333-3333-333333333335','55555555-5555-5555-5555-555555555571',1750000,true,true,now()),
 ('11111111-1111-1111-1111-111111111113','22222222-2222-2222-2222-222222222224','33333333-3333-3333-3333-333333333335','55555555-5555-5555-5555-555555555572',1610000,true,false,now())
ON CONFLICT(tenant_id,store_id,warehouse_id,product_id)
DO UPDATE SET selling_price=EXCLUDED.selling_price,visible=EXCLUDED.visible,allow_reservation=EXCLUDED.allow_reservation,last_verified_at=now(),updated_at=now();

-- Phase 13: demo search aliases, OEM equivalents and vehicle fitments.
INSERT INTO product_search_terms(tenant_id,product_id,kind,term,normalized_term) VALUES
 ('11111111-1111-1111-1111-111111111111','55555555-5555-5555-5555-555555555551','alias','لنت 206','لنت 206'),
 ('11111111-1111-1111-1111-111111111111','55555555-5555-5555-5555-555555555551','alias','لنت تیپ 5','لنت تیپ 5'),
 ('11111111-1111-1111-1111-111111111111','55555555-5555-5555-5555-555555555551','oem','4254.97','4254.97'),
 ('11111111-1111-1111-1111-111111111111','55555555-5555-5555-5555-555555555552','alias','فیلتر روغن 206','فیلتر روغن 206'),
 ('11111111-1111-1111-1111-111111111111','55555555-5555-5555-5555-555555555553','alias','واتر پمپ سمند ef7','واتر پمپ سمند ef7'),
 ('11111111-1111-1111-1111-111111111112','55555555-5555-5555-5555-555555555561','alias','لنت 206','لنت 206'),
 ('11111111-1111-1111-1111-111111111112','55555555-5555-5555-5555-555555555561','oem','4254.97','4254.97'),
 ('11111111-1111-1111-1111-111111111112','55555555-5555-5555-5555-555555555562','alias','واتر پمپ سمند ef7','واتر پمپ سمند ef7'),
 ('11111111-1111-1111-1111-111111111113','55555555-5555-5555-5555-555555555571','alias','لنت 206','لنت 206'),
 ('11111111-1111-1111-1111-111111111113','55555555-5555-5555-5555-555555555571','oem','4254.97','4254.97'),
 ('11111111-1111-1111-1111-111111111113','55555555-5555-5555-5555-555555555572','alias','واتر پمپ سمند ef7','واتر پمپ سمند ef7')
ON CONFLICT(tenant_id,product_id,kind,normalized_term) DO UPDATE SET term=EXCLUDED.term;

INSERT INTO product_fitments(tenant_id,product_id,vehicle_variant_id,year_from,year_to,notes) VALUES
 ('11111111-1111-1111-1111-111111111111','55555555-5555-5555-5555-555555555551','92000000-0000-0000-0000-000000000001',1380,1400,'محور جلو'),
 ('11111111-1111-1111-1111-111111111111','55555555-5555-5555-5555-555555555552','92000000-0000-0000-0000-000000000001',1380,1400,'TU5'),
 ('11111111-1111-1111-1111-111111111111','55555555-5555-5555-5555-555555555552','92000000-0000-0000-0000-000000000002',1380,1400,'TU3'),
 ('11111111-1111-1111-1111-111111111111','55555555-5555-5555-5555-555555555553','92000000-0000-0000-0000-000000000004',1388,1402,'موتور EF7'),
 ('11111111-1111-1111-1111-111111111112','55555555-5555-5555-5555-555555555561','92000000-0000-0000-0000-000000000001',1380,1400,'محور جلو'),
 ('11111111-1111-1111-1111-111111111112','55555555-5555-5555-5555-555555555562','92000000-0000-0000-0000-000000000004',1388,1402,'موتور EF7'),
 ('11111111-1111-1111-1111-111111111113','55555555-5555-5555-5555-555555555571','92000000-0000-0000-0000-000000000001',1380,1400,'محور جلو'),
 ('11111111-1111-1111-1111-111111111113','55555555-5555-5555-5555-555555555572','92000000-0000-0000-0000-000000000004',1388,1402,'موتور EF7')
ON CONFLICT(tenant_id,product_id,vehicle_variant_id,year_from,year_to) DO UPDATE SET notes=EXCLUDED.notes,updated_at=now();

-- Phase 15.11: advanced local pricing. Network offer prices remain independent.
INSERT INTO price_lists(tenant_id,store_id,code,name,is_default,active)
SELECT s.tenant_id,s.id,'retail','خرده / مصرف‌کننده',false,true FROM stores s
ON CONFLICT(tenant_id,store_id,code) DO UPDATE SET name=EXCLUDED.name,active=true,updated_at=now();
UPDATE price_lists pl SET is_default=true,updated_at=now()
WHERE pl.code='retail' AND pl.active
  AND NOT EXISTS (SELECT 1 FROM price_lists x WHERE x.tenant_id=pl.tenant_id AND x.store_id=pl.store_id AND x.is_default AND x.active);
INSERT INTO store_pricing_settings(tenant_id,store_id,min_margin_bps,cashier_may_override)
SELECT s.tenant_id,s.id,1000,true FROM stores s
ON CONFLICT(tenant_id,store_id) DO NOTHING;

INSERT INTO price_lists(tenant_id,store_id,code,name,is_default,active) VALUES
 ('11111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','mechanic','مکانیک',false,true),
 ('11111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','wholesale','عمده',false,true)
ON CONFLICT(tenant_id,store_id,code) DO UPDATE SET name=EXCLUDED.name,active=true,updated_at=now();

INSERT INTO product_price_breaks(tenant_id,store_id,product_id,price_list_id,min_qty,unit_price)
SELECT DISTINCT ON (o.tenant_id,o.store_id,o.product_id)
       o.tenant_id,o.store_id,o.product_id,pl.id,1,o.selling_price
FROM store_product_offers o
JOIN price_lists pl ON pl.tenant_id=o.tenant_id AND pl.store_id=o.store_id AND pl.code='retail' AND pl.active
ORDER BY o.tenant_id,o.store_id,o.product_id,o.updated_at DESC,o.id DESC
ON CONFLICT(tenant_id,store_id,product_id,price_list_id,min_qty)
DO UPDATE SET unit_price=EXCLUDED.unit_price,updated_at=now();

-- Demo local prices include quantity breaks; these do not change public network prices.
INSERT INTO product_price_breaks(tenant_id,store_id,product_id,price_list_id,min_qty,unit_price)
SELECT '11111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222',v.product_id,pl.id,v.min_qty,v.unit_price
FROM (VALUES
 ('55555555-5555-5555-5555-555555555551'::uuid,1::numeric,1710000::bigint),
 ('55555555-5555-5555-5555-555555555551'::uuid,5::numeric,1660000::bigint),
 ('55555555-5555-5555-5555-555555555551'::uuid,10::numeric,1610000::bigint),
 ('55555555-5555-5555-5555-555555555552'::uuid,1::numeric,315000::bigint),
 ('55555555-5555-5555-5555-555555555552'::uuid,10::numeric,295000::bigint),
 ('55555555-5555-5555-5555-555555555553'::uuid,1::numeric,1530000::bigint),
 ('55555555-5555-5555-5555-555555555553'::uuid,5::numeric,1490000::bigint)
) AS v(product_id,min_qty,unit_price)
JOIN price_lists pl ON pl.tenant_id='11111111-1111-1111-1111-111111111111' AND pl.store_id='22222222-2222-2222-2222-222222222222' AND pl.code='mechanic'
ON CONFLICT(tenant_id,store_id,product_id,price_list_id,min_qty)
DO UPDATE SET unit_price=EXCLUDED.unit_price,updated_at=now();

INSERT INTO product_price_breaks(tenant_id,store_id,product_id,price_list_id,min_qty,unit_price)
SELECT '11111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222',v.product_id,pl.id,v.min_qty,v.unit_price
FROM (VALUES
 ('55555555-5555-5555-5555-555555555551'::uuid,1::numeric,1660000::bigint),
 ('55555555-5555-5555-5555-555555555551'::uuid,10::numeric,1580000::bigint),
 ('55555555-5555-5555-5555-555555555552'::uuid,1::numeric,300000::bigint),
 ('55555555-5555-5555-5555-555555555552'::uuid,20::numeric,275000::bigint),
 ('55555555-5555-5555-5555-555555555553'::uuid,1::numeric,1490000::bigint),
 ('55555555-5555-5555-5555-555555555553'::uuid,10::numeric,1440000::bigint)
) AS v(product_id,min_qty,unit_price)
JOIN price_lists pl ON pl.tenant_id='11111111-1111-1111-1111-111111111111' AND pl.store_id='22222222-2222-2222-2222-222222222222' AND pl.code='wholesale'
ON CONFLICT(tenant_id,store_id,product_id,price_list_id,min_qty)
DO UPDATE SET unit_price=EXCLUDED.unit_price,updated_at=now();

UPDATE customers c SET price_list_id=pl.id,updated_at=now()
FROM price_lists pl
WHERE c.id='66666666-6666-6666-6666-666666666661'
  AND pl.tenant_id=c.tenant_id AND pl.store_id=c.store_id AND pl.code='mechanic';
UPDATE customers c SET price_list_id=pl.id,updated_at=now()
FROM price_lists pl
WHERE c.id='66666666-6666-6666-6666-666666666662'
  AND pl.tenant_id=c.tenant_id AND pl.store_id=c.store_id AND pl.code='wholesale';
