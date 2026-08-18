# Phase 15.12 Runbook — Multi-Unit & Packaging + Product Setup Wizard

## Goal

Close the mature auto-parts-store packaging gap without sacrificing inventory/accounting correctness, and make new-product setup a single workflow instead of a sequence of disconnected screens.

## Migration

Phase 15.12 requires:

```text
migrations/017_multi_unit_packaging.sql
```

The API readiness gate is pinned to `017_multi_unit_packaging.sql`.

## Canonical inventory rule

Inventory is always stored and reconciled in the product's **base unit**.

Example:

```text
Base unit: piece
1 carton = 12 pieces
Purchase: 2 cartons  => +24 pieces inventory
Sale:     5 pieces   => -5 pieces inventory
Remaining:             19 pieces
```

Sale/purchase/return documents preserve the commercial unit, quantity and conversion factor used at transaction time so historical documents remain explainable even if unit setup changes later.

## Delivered workflows

- Canonical base unit for every product.
- Alternate sale/purchase units with conversion factors.
- Piece, pair, set, pack, box, carton and custom product-unit codes/names.
- Barcode per active unit/packaging, unique inside the tenant.
- Unit-specific retail/price-list breaks.
- Derived package pricing from base-unit price when no explicit package price exists.
- Purchase in carton/pack with base-unit weighted-average cost.
- Sale in selected commercial unit with base-unit stock enforcement.
- Sale/purchase returns in the original commercial unit.
- Inventory views and adjustments explicitly use the base unit.
- Import/export of package definitions and conversion factors.
- Existing products receive a canonical base unit during migration.
- Existing Phase 15.11 price breaks are attached to the base unit.

## New Product Wizard

Open:

```text
/store/products/new
```

The wizard supports, in one transaction-oriented setup flow:

1. product title, SKU, brand and OEM code;
2. base unit type/name;
3. base-unit barcode by scanner/manual entry;
4. generation of an internal EAN-13-style barcode;
5. initial base-unit retail price;
6. fractional-stock policy for weighted/liquid products;
7. any number of package units;
8. package name, code and conversion factor;
9. package-specific barcode;
10. optional package-specific initial retail price;
11. purchase/sale permissions per package;
12. label printing through Store Edge after creation.

Client and server both reject duplicated unit codes/barcodes and impossible conversion policies. Tenant-wide barcode uniqueness remains enforced by the database.

## Offline Store Edge continuity

Phase 15.12 keeps package workflows available when Cloud connectivity is unavailable:

- Snapshot carries active sale units and package barcodes.
- Store Edge catalog search includes alternate-unit barcodes.
- Offline package sale records commercial quantity + canonical base quantity.
- Local availability is reduced by base quantity.
- Package-specific default-retail breaks work offline.
- If no package price is configured, Store Edge derives package price from the base-unit price × conversion factor.
- Already-resolved customer-tier prices from the web POS can still be preserved through the offline queue.
- `product_unit_id` is pushed to Cloud during sync.
- Cloud revalidates the selected unit, inventory, pricing and cashier policy through the normal sales service.
- Pending sales created before Phase 15.12 remain compatible; old queue items without `base_qty` fall back to their previous quantity behavior.

## Import format

The Import Center supports alternate packages in the packages field using entries separated by `;`:

```text
code|name|factor|barcode|sale|purchase|price
```

Example:

```text
carton|کارتن|12|2900000000025|true|true|10800000;pack|بسته|4|2900000000026|true|true|3700000
```

Package price may be omitted/zero to use derived base pricing.

## Acceptance checks

Run from repository root:

```bash
bash ops/rc-static-check.sh
go test ./internal/storeedge
git diff --check
```

When Go module checksums/dependencies are available:

```bash
go test ./...
```

Frontend, when dependencies are installed:

```bash
cd web
npm run build
```

## Manual acceptance

1. Open **Store > Inventory > New Product**.
2. Create a product with base unit `piece`, scan/type a base barcode and set an initial retail price.
3. Add `carton`, factor `12`, scan/type a separate carton barcode, and optionally set a carton price.
4. Create the product and print both labels through Store Edge.
5. Search the inventory/POS by the carton barcode and verify it resolves the product + carton unit.
6. Purchase 2 cartons and verify inventory increases by 24 base pieces while the purchase document shows 2 cartons.
7. Sell 5 pieces and verify base inventory decreases by 5.
8. Sell 1 carton and verify base inventory decreases by 12 while the sales document shows 1 carton.
9. Remove the explicit carton price and verify price derives from base price × 12.
10. Add carton-specific quantity breaks and verify package quantity pricing resolves independently.
11. Return a carton sale/purchase and verify document quantity is commercial while stock movement is base-unit correct.
12. Simulate Cloud loss, scan the carton barcode in POS, queue an offline cash/card sale, and verify Store Edge reduces 12 base pieces per carton.
13. Sync the offline sale and verify Cloud receives/revalidates `product_unit_id`.
14. Import a CSV row containing package definitions and export the catalog; verify unit codes, factors and barcodes round-trip.
15. Attempt duplicate barcodes, duplicate unit codes or fractional carton factors on a non-fractional product; verify the operation is rejected.

## Environment note

The supplied source baseline does not contain `go.sum` or `web/node_modules`. In an offline sandbox, full `go test ./...` stops before compilation when packages requiring pgx/uuid checksums are loaded, and full Next build cannot run without frontend dependencies. `internal/storeedge` is dependency-light and is tested directly; static gates, TypeScript syntax transpilation and diff hygiene remain runnable.
