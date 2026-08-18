# Release Notes — Phase 15.12 Multi-Unit & Packaging + Product Setup Wizard

Phase 15.12 adds real auto-parts packaging semantics while keeping one canonical inventory quantity, and turns new-product creation into a barcode/unit-aware setup workflow.

## Highlights

- Base unit plus alternate piece/pair/set/pack/box/carton units.
- Conversion factors with safe fractional-stock rules.
- Barcode per packaging unit with tenant-level uniqueness.
- New Product Wizard for title/SKU/OEM, base unit, package units, barcodes and initial prices in one flow.
- Barcode-scanner friendly fields plus generated internal EAN-13-style barcodes.
- Label printing from the product setup/unit management flow through Store Edge.
- Purchase cartons and sell pieces without corrupting inventory or weighted-average cost.
- Commercial unit, commercial quantity and conversion facts preserved on sale/purchase lines.
- Returns preserve the commercial unit while inventory moves in canonical base quantity.
- Unit-specific price breaks; package prices derive from base pricing when an explicit package tier is absent.
- Inventory UI identifies the base unit and keeps manual adjustments base-unit explicit.
- Import/export carries package definitions, factors and barcodes.
- Existing products and existing Phase 15.11 pricing migrate to canonical base units.
- Store Edge offline POS supports package barcodes, package pricing, base-stock deduction and `product_unit_id` cloud sync.

## Database

Adds migration `017_multi_unit_packaging.sql` with:

- `products.allow_fractional_base_qty`
- `product_units`
- commercial-unit archive fields on `sale_items` and `purchase_items`
- `product_unit_id` on `product_price_breaks`
- tenant-scoped active package-barcode uniqueness

## Compatibility

- Existing products are backfilled with a base unit using their existing `products.unit` and barcode.
- Existing price breaks become base-unit price breaks.
- Existing sale/purchase items are backfilled with commercial quantity equal to their historical base quantity.
- Network/public offer pricing remains independent from local packaging price lists.
- Pre-15.12 Store Edge queue entries remain readable; pending deductions fall back to legacy quantity when `base_qty` is absent.

## Next product phase

**Phase 15.13 — Bank Reconciliation & Check Intelligence**: check maturity averaging (راس‌گیری), future cash calendar, bank reconciliation, owner/accountant control surface and customer check-risk visibility.
