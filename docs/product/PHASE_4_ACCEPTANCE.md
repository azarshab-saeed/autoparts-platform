# Phase 4 Acceptance — Frontend Foundation + Store Dashboard + Sales UI

## Goal
Deliver the first visible, runnable store product surface and connect the primary sale workflow to the existing Go API contract.

## Accepted scope
- Next.js + React + TypeScript frontend under `web/`
- Persian RTL design system and responsive store shell
- Login screen
- Persistent local browser session for Phase 4
- Store dashboard with KPI cards, recent sales, low-stock preview, weekly chart placeholder and quick actions
- Sales screen with product search, cart lines, quantity, editable unit price, customer search, payment method and final submission
- Mock Mode (`NEXT_PUBLIC_MOCK_MODE=true`) so UI can be demonstrated without backend/database
- Real API Mode using Phase 3 auth, `/v1/me`, `/v1/products`, `/v1/customers`, and `/v1/sales`
- Idempotency-Key generated for every real sale submit
- Credit sale UI requires a selected customer
- Responsive desktop/tablet/mobile layout
- Frontend Dockerfile and local development instructions

## Deliberately not in Phase 4
- Purchase UI
- Full inventory UI
- Customer management CRUD screens
- Supplier screens
- Accounting screens
- Mechanic/consumer frontend
- Production-grade refresh tokens / httpOnly auth cookies
- Dashboard analytics API (dashboard uses representative mock data until reporting backend exists)

## Product rule from this phase onward
No backend-only feature phase is considered complete unless the corresponding user-facing screen or an explicit infrastructure-only justification is included.
