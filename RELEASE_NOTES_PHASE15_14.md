# Release Notes — Phase 15.14

## Tax, VAT & Official Invoicing Foundation

- Added versioned, effective-date tax/VAT configuration without hard-coded production rates.
- Added seller and buyer legal/economic identity fields used as immutable invoice snapshots.
- Added normal/official invoice modes and atomic server-side official invoice numbering.
- Added taxable, exempt and non-taxable product treatment through tax-code mapping.
- Added server-side tax quote/revalidation so POS totals cannot be trusted from the browser alone.
- Split sales revenue from `VAT_PAYABLE` in double-entry accounting.
- Added tax-aware sale-return reversal from the original historical tax snapshot.
- Added tax invoice register, accountant CSV foundation and printable official invoice dataset.
- Added local correction/cancellation request prerequisites for the next Modian integration phase.
- Added store UI for tax settings, rate versions, product tax mapping, customer identity and official invoice review.
- Tax-applied/official sales intentionally stay online-only so Store Edge cannot create an invoice with missing server numbering or tax snapshot.

Migration: `019_tax_official_invoicing.sql`.
