# Phase 15.15 Runbook — Invoice Designer, Document Templates & Barcode Label Printing

## Scope

Phase 15.15 removes day-to-day document-layout and barcode-label printing friction before the external Modian transport phase.

Delivered:

- migration `020_document_templates_barcode_labels.sql`;
- tenant/store-scoped structured document templates;
- configurable template kinds for sales invoices, thermal receipts, quotations, purchase invoices, sales returns, payment receipts and barcode labels;
- A4/80mm/custom paper settings;
- logo URL/data-URL, store/customer/tax identity visibility, editable column captions, header/footer/terms and signature controls;
- live preview plus browser Print / Save as PDF workflow;
- active/default template management, duplicate/new/delete operations and existing mutation audit coverage;
- POS selection of an active sales/thermal print template;
- immutable document-template snapshot on every new cloud sale;
- selected template identity preserved through Store Edge offline queue/sync;
- customer-facing sale print driven by the stored historical snapshot while internal margin/audit data stays out of the print;
- barcode-label templates with physical width/height/padding, sheet columns/gaps, barcode height and field visibility controls;
- multi-unit label catalog for piece/pair/set/pack/carton barcodes and per-commercial-unit retail price;
- batch label quantities and browser sheet/PDF printing;
- direct Store Edge template-aware ZPL label printing with unit/package metadata;
- EAN-13 browser barcode rendering for 13-digit EAN values and direct Code 128 through Store Edge/ZPL for other identifiers;
- product-create flow links directly to the professional label/template workspace.

## Migration

Apply migrations through the normal API migration path. Readiness is pinned to:

```text
020_document_templates_barcode_labels.sql
```

## Smoke acceptance

After applying the package:

1. Build/start the stack:

   ```bash
   docker compose up -d --build
   ```

2. Open **طراحی فاکتور و بارکد**.
3. Duplicate or create a sales template, change:
   - logo URL;
   - visible columns;
   - one column title;
   - footer/terms;
   - paper size or margin.
4. Save it and make it active.
5. Open **فروش جدید**, select the new print template, post a sale and open its detail page.
6. Verify the printed sale follows that template.
7. Change the template again and verify the already-posted sale still shows the old snapshot.
8. In **قالب بارکد**, create/edit a label size and choose which fields to print.
9. In **چاپ لیبل**, search a product that has base + package/carton units and verify each unit exposes its own barcode/price.
10. Select multiple label counts and test:
    - browser Print / Save as PDF;
    - Store Edge direct label print when a label printer is configured.
11. Disconnect the cloud, post an eligible offline sale with a selected print template, sync later and verify the cloud sale retained that template selection.

## Automated/static acceptance performed for this package

- `./ops/rc-static-check.sh` — PASS
- `git diff --check` — PASS
- `go test ./cmd/store-edge ./internal/storeedge` — PASS
- `tsc --noEmit --strict --target ES2022 --lib ES2022,DOM web/lib/types.ts web/lib/print-utils.ts` — PASS
- TypeScript syntax/transpile checks for the changed TS/TSX modules — PASS

Full API `go test ./...` cannot run in the artifact sandbox because the baseline repository still has no committed `go.sum` and outbound module download is unavailable. The user's `docker compose up -d --build` remains the authoritative full Go + Next.js compile gate.

## Constraint

The designer is intentionally a structured/block configuration system, not an arbitrary HTML/JavaScript editor. This keeps printed documents tenant-safe and snapshot-stable.

Browser/PDF barcode rendering is guaranteed as a real EAN-13 pattern for 13-digit EAN values. Direct Store Edge ZPL uses printer-native Code 128 for other barcode identifiers. A non-EAN browser fallback is visual only and should not be treated as a guaranteed scannable Code 128 symbol.

## Next phase

**Phase 15.16 — Iranian Modian Integration**. It must be implemented against the current official protocol/specification at that time and must not be inferred from stale Modian documentation.
