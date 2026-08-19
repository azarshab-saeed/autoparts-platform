# Release Notes — Phase 15.15

## Invoice Designer, Document Templates & Barcode Label Printing

- Added store-scoped document templates with structured, safe layout settings.
- Added configurable sales/A4, thermal, quotation, purchase, return, payment-receipt and barcode-label template kinds.
- Added live document preview, logo URL/data-URL, editable column labels, show/hide fields, margins, font scale, header/footer/terms and signature controls.
- Added browser Print / Save as PDF workflow.
- Added active/default template management and new/duplicate/delete flows.
- Added selectable print templates in POS; the server validates tenant/store ownership and snapshots the selected template when the sale is posted.
- Preserved selected template identity through Store Edge offline sale sync.
- Updated sale print to consume the historical template snapshot and keep internal pricing/margin audit outside the customer print.
- Added physical barcode-label template sizing, sheet columns/gaps and field visibility controls.
- Added multi-unit label catalog and batch quantities for piece/package/carton barcodes.
- Added direct Store Edge template-aware ZPL printing and browser label-sheet/PDF printing.
- Added professional label/template navigation from the new-product workflow.

Migration: `020_document_templates_barcode_labels.sql`.
