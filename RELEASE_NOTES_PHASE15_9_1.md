# Release notes — Phase 15.9.1

Hotfix for the Checks & Banking page discovered during runtime acceptance.

The frontend previously requested 200 cheque rows while the shared API pagination contract allows at most 100, causing `limit must be between 1 and 100`. The page now requests 100 rows. The cheque form also exposes the selected customer/supplier outstanding balance and prevents entering a cheque above that balance before submission.
