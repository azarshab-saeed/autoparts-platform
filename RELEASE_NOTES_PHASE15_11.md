# Release Notes — Phase 15.11 Advanced Pricing & Trade Terms

Phase 15.11 turns pricing from a single store selling price into a tenant-safe commercial pricing system designed for auto-parts retail, mechanics and wholesale.

## Highlights

- Configurable price lists such as retail, mechanic, colleague, wholesale, VIP and dealer.
- Customer-specific default price tiers.
- Quantity-break pricing per product and price list.
- Automatic POS repricing when customer or quantity changes.
- Line discounts plus invoice-level percentage/fixed discounts.
- Margin Guard based on weighted-average inventory cost without exposing raw cost through the Cashier quote API.
- Cashier price-override policy and reasoned/audited Owner/Admin below-margin authority.
- Historical pricing audit: list price, final price, discount, source, reason, actor and margin decision.
- Gross, discount and net sale facts persisted for reporting.
- Sales history displays discount and below-margin indicators.
- Local private tiers are separate from public/network `store_product_offers` pricing.
- Import Center initializes the local default retail tier from imported selling prices.
- Store Edge snapshot carries default-retail quantity breaks and the cashier override policy; offline POS reprices quantity breaks while rejecting manual overrides when policy forbids them.
- Main web POS offline fallback preserves the resolved customer-tier price and carries `customer_id` through Store Edge to Cloud, where pricing policy is revalidated.

## Database

Adds migration `016_advanced_pricing.sql` with:

- `price_lists`
- `store_pricing_settings`
- `product_price_breaks`
- `customers.price_list_id`
- immutable pricing/audit fields on `sales` and `sale_items`

Existing sales are backfilled with `gross_amount = total_amount` and zero discount.

## Compatibility

Existing network reservation/procurement-generated sales remain compatible and are recorded as gross=net with zero discount. Existing network offer pricing is not replaced by private local price lists.

## Next product phase

**Phase 15.12 — Multi-Unit & Packaging**: piece/pair/set/pack/carton units, conversions, package barcodes, purchase in carton and sale in piece, and commercial unit-aware pricing/documents.
