# Phase 15.11 Runbook — Advanced Pricing & Trade Terms

## Goal

Close the multi-price / wholesale pricing migration blocker while preserving the platform's network-price separation.

## Migration

Phase 15.11 requires:

```text
migrations/016_advanced_pricing.sql
```

The API readiness gate is pinned to `016_advanced_pricing.sql`.

## Delivered workflows

- Store-scoped configurable price lists.
- Default retail list provisioned for existing and new stores.
- Customer default price-list assignment.
- Quantity-based product price breaks.
- Server-side quote resolution by customer + quantity + warehouse.
- Default-list fallback when a customer tier has no price for a product.
- Legacy offer fallback only when no local price-list value is configured.
- POS automatic repricing when customer or quantity changes.
- Manual line discounts and invoice-level percentage/fixed discounts.
- Server-side cashier override policy; pricing quote does not expose raw weighted-average cost to Cashier.
- Weighted-average-cost margin guard.
- Below-margin sale limited to Owner/Admin and requires an auditable reason.
- Immutable historical list price, final price, gross, discount and net amounts.
- Override reason + actor persisted on the sale line.
- Historical margin threshold / below-margin decision persisted on the sale line.
- Sales history exposes discounts and below-margin activity.
- Store Edge snapshot carries default-retail quantity breaks, the local default retail price, and the cashier price-override policy before falling back to the legacy/public offer price.
- Offline Store Edge automatically reprices non-manual lines when quantity crosses a default-retail break; explicit manual prices remain explicit overrides only when policy allows them.
- Main web POS offline fallback carries `customer_id` and preserves the already-resolved customer-tier price, so mechanic/wholesale pricing is not silently replaced by default retail while offline.
- Store Edge strips local-only pricing flags before cloud sync; Cloud revalidates customer tier, Margin Guard and override policy server-side.
- Import Center seeds the local default retail tier from imported `selling_price`.

## Network price isolation

Local price lists live in `product_price_breaks` and are never published as network offers by the pricing API.

`store_product_offers.selling_price` remains the network/public offer price. Editing a local mechanic, wholesale or VIP list does not update `store_product_offers`.

## Acceptance checks

Run from repository root:

```bash
bash ops/rc-static-check.sh
GOTOOLCHAIN=local go test internal/pricing/calc.go internal/pricing/calc_test.go
GOTOOLCHAIN=local go test ./...
```

Frontend, when dependencies are installed:

```bash
cd web
npm run build
```

## Manual acceptance

1. Open **Store > Pricing & Discounts** as Owner/Admin.
2. Create a `mechanic` or `wholesale` price list.
3. Set product price breaks for quantity 1, 5 and 10.
4. Assign that list to a customer.
5. Open POS and select the customer; verify the active price list changes.
6. Change quantity across a break and verify the suggested unit price changes.
7. Apply a line discount; verify a reason is recorded.
8. Apply an invoice percentage or fixed discount; verify the net total changes.
9. As Cashier, attempt a sale below Margin Guard; it must be rejected server-side.
10. As Owner/Admin, attempt a below-margin sale without a reason; it must be rejected, then add a reason and verify sale detail/history records the actor and exception.
11. Change the price list after the sale; verify historical sale detail still shows the original list/final prices and discount.
12. In Store Edge offline POS, add a product with default-retail breaks, cross a quantity threshold and verify the unit price changes locally before sync.
13. Disable cashier price override, open the standalone Store Edge POS, and verify manual price editing is disabled/rejected.
14. Select a mechanic/wholesale customer in the main web POS, simulate API/server loss, queue a cash/card sale, then sync later; verify `customer_id` and the resolved tier price survive the offline path.
15. Confirm Network offer price did not change when local mechanic/wholesale prices changed.

## Environment note

The supplied source archive does not contain `go.sum` or `web/node_modules`. In an offline sandbox, full `go test ./...` stops before compilation because Go cannot verify/fetch module checksums, and `npm run build` cannot run without installed frontend dependencies. The dependency-free pricing calculation tests and static safety gate are still runnable.
