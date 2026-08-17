# Roadmap - Auto Parts Network

## Phase 1 - UX / Screen Spec / API Contract
Status: Completed

## Phase 2 - Backend Foundation
Status: Completed
- tenant/store foundation
- catalog
- customer/supplier
- error envelope and pagination
- Docker packaging and test foundation

## Phase 3 - Inventory + Purchase Backend
Status: Completed
- purchase posting
- supplier payable journal
- perpetual weighted-average valuation
- warehouse receiving
- inventory ledger/balances
- physical stock adjustment
- reorder points and low-stock query
- COGS/inventory accounting on sale posting

## Phase 4 - Frontend Foundation + Sales + Identity
Status: Completed
- Next.js / React RTL frontend
- store dashboard
- sale entry UI
- mock/real API mode
- Keycloak OIDC + PKCE
- Go JWT/JWKS verification
- PostgreSQL 18.4 development stack

## Phase 5 - Inventory + Purchase UI
Status: Completed
- inventory screen
- low-stock filter
- physical stock adjustment UI
- reorder point UI
- purchase entry UI
- supplier selection
- weighted-average cost visibility
- demo suppliers/reorder points

## Phase 6 - Settlement + Returns
Status: Completed
- mixed and partial payments
- customer/supplier party ledger
- customer receipts and supplier payments
- sales return with inventory/COGS reversal
- purchase return with valuation protection
- UI for receivables/payables and settlement
- versioned migration runner for existing DB volumes
- toman (`IRT` application code) as the explicit V1 money unit

## Phase 7 - Network Search
Status: Completed
- store publish policy
- availability/search index
- mechanic/consumer search UI
- ranking by availability, freshness, price and distance

## Phase 8 - Reservation + Orders
Status: Completed
- inventory-backed reservation lifecycle
- store confirmation / ready state
- mechanic/customer order tracking
- cancellation and expiry release

## Phase 9 - Reservation Fulfillment
Status: Completed
- ready reservation -> posted sale atomically
- held stock consumes both on_hand and reserved exactly once
- cash/card/credit and partial payment reuse sale accounting
- COGS, inventory, journal and outbox posting
- fulfilled state and buyer/store UI

## Phase 10 - Accounting Completion
Status: Completed
- operating expenses with automatic balanced journals
- default expense categories
- customer/supplier party-ledger statements
- store-level profit/loss report
- expense breakdown by category

## Phase 11 - Migration & Onboarding
- Excel import/mapping
- legacy software adapters
- guided store onboarding

## Phase 12 - Hardening & Scale
- observability
- rate limiting
- security review
- load tests
- backup/restore
- tenancy isolation tests
