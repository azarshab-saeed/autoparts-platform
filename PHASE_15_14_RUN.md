# Phase 15.14 Runbook — Tax, VAT & Official Invoicing Foundation

## Scope

Phase 15.14 makes the local transaction model ready for configurable tax/VAT and official invoicing without coupling sale correctness to an external Modian transport.

Delivered:

- migration `019_tax_official_invoicing.sql`;
- tenant legal/economic identity profile;
- store tax settings and official invoice numbering policy;
- versioned tax codes/rates with effective dates;
- taxable, exempt and non-taxable categories;
- product -> tax code mapping;
- customer legal/tax identity fields;
- normal vs official sale mode;
- server-side tax quote and sale revalidation;
- immutable seller, buyer and line-level tax snapshots;
- separate `VAT_PAYABLE` accounting entry instead of treating tax as sales revenue;
- tax-aware sale returns using original invoice snapshots;
- official invoice list and printable snapshot dataset;
- correction/cancellation request prerequisites;
- accountant CSV/reporting foundation;
- POS tax calculation and official invoice UX;
- official/tax-applied sales are online-only until the external submission workflow exists.

## Migration

Apply migrations through the normal API migration path. Readiness is pinned to:

```text
019_tax_official_invoicing.sql
```

## Smoke acceptance

After applying the package:

1. Start the stack:

   ```bash
   docker compose up -d --build
   ```

2. Open **مالیات و فاکتور رسمی**.
3. Create at least one tax code/rate version and set its effective date.
4. Complete seller identity, choose a default tax code and enable tax.
5. Map one product explicitly and leave another on the default code.
6. Create a normal sale and verify tax follows `tax_on_normal_sales`.
7. Create an official sale and verify:
   - server-generated invoice number;
   - net/tax/total are persisted;
   - official printable snapshot opens;
   - changing the current tax rate later does not change the old invoice.
8. Return part of the official sale and verify the returned tax is proportional to the original tax snapshot.
9. Confirm the sale journal balances and VAT is posted to `VAT_PAYABLE`, not sales revenue.
10. Request correction/cancellation from the tax invoice list and verify the local prerequisite state is recorded.

## Automated/static acceptance performed for this package

- `go test internal/tax/calc.go internal/tax/calc_test.go` — PASS
- `./ops/rc-static-check.sh` — PASS
- `git diff --check` — PASS
- TypeScript contract check for all Phase 15.14 touched web modules with local React/Next stubs — PASS

Full `go test ./...` cannot run in the artifact sandbox because the baseline repository still does not contain `go.sum` and outbound dependency download is unavailable there. The user's Docker build remains the authoritative full compile gate.

## Not in this phase

Phase 15.14 does **not** claim external Modian submission/acceptance. Protocol mapping, signing/credentials, submission state, retry, tracking identifiers, current official validation rules and external correction/cancellation transport belong to Phase 15.15 and must be implemented against the then-current official specification.
