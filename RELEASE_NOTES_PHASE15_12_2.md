# Release Notes — Phase 15.12.2

Build-only hotfix for the web application.

Fixed `web/lib/mock.ts` so `SaleHistoryItem` mock fixtures match the required type definition:
- removed a duplicate `below_margin_count` property;
- supplied the missing `below_margin_count` property on the next fixture.

No migration and no product behavior change. Roadmap remains on Phase 15.13 next.
