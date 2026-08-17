# Phase 11 Acceptance Criteria

- [ ] Sales history cannot return another tenant/store's documents.
- [ ] Purchase history cannot return another tenant/store's documents.
- [ ] `payment_state=paid|due` returns the expected subset.
- [ ] Pagination returns `total` and a usable `next_cursor` when more rows exist.
- [ ] Dashboard metrics are computed from persisted data in real API mode.
- [ ] Inventory report rejects a warehouse outside the authenticated store.
- [ ] Inventory value equals `on_hand * avg_unit_cost` per row and aggregated summary.
- [ ] Low-stock uses available stock (`on_hand-reserved`) and configured reorder point.
- [ ] Dead stock flags positive on-hand inventory with no sale in 90 days.
- [ ] Cash report includes cash/card sale/purchase payments, settlements, expenses and cash/card returns.
- [ ] A daily close stores a snapshot and variance exactly once per business date.
- [ ] Reusing an idempotency key returns the same close.
- [ ] Late same-day activity after close sets `changed_after_close=true`.
- [ ] Sale and purchase documents are printable and link to return workflow.
- [ ] CSV exports open correctly as UTF-8 in spreadsheet tools.
- [ ] Docker build passes for API and web on the user's local environment.
