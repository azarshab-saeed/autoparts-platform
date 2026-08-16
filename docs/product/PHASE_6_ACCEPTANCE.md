# Phase 6 Acceptance

- [x] Sales support split / partial payment contract.
- [x] Purchases support split / partial payment contract.
- [x] Unpaid sale balance posts to customer ledger.
- [x] Unpaid purchase balance posts to supplier ledger.
- [x] Customer receipt reduces AR and party balance atomically.
- [x] Supplier payment reduces AP and party balance atomically.
- [x] Settlement overpayment rejected in V1.
- [x] Sale return quantity capped by original quantity minus prior returns.
- [x] Sale return restores inventory and reverses revenue/COGS.
- [x] Purchase return quantity capped and available-stock checked.
- [x] Purchase return protects weighted-average valuation invariant.
- [x] All writes use tenant/store from verified Keycloak context.
- [x] New financial writes require idempotency keys.
- [x] Existing DB volumes receive versioned migrations without reset.
- [x] Store UI exposes accounts, settlements and returns.
- [x] Toman is the explicit V1 monetary business unit (`IRT` application code).
