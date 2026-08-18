# Phase 15.9.1 — Checks UX Hotfix

- fixes the checks list request to respect the API pagination maximum (`limit=100`)
- shows the selected party's outstanding balance next to the cheque amount
- blocks cheque amounts above the outstanding balance before the API request
- translates the outstanding-balance backend error to Persian

No database migration is required.
