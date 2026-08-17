# Phase 12 — Network Procurement / تأمین از شبکه

Phase 12 lets one store procure stock from another network-enabled store without leaving the platform.

## Start

```bash
docker compose up --build
```

Do not remove the PostgreSQL volume. `010_network_procurement.sql` upgrades the existing database in place.

## Recommended dev smoke test

All dev users use the password `ChangeMe123!`.

1. Sign in as `owner@example.com` (یدکی رضایی).
2. Open `/store/procurement`.
3. Search `لنت ۲۰۶` and choose the local `BRK-206-TXT` product.
4. Request one unit from `پارس یدک` or another procurement-enabled store.
5. Sign out and sign in as the owner of the selected seller (`pars@example.com` for پارس یدک).
6. Open `/store/procurement`, find the incoming request, press **تأیید درخواست**, then **آماده تحویل شد**.
7. Sign back in as `owner@example.com`.
8. Open `/store/procurement` and press **دریافت + ورود به انبار**.

Expected result:

- seller `reserved` increases immediately when the procurement request is created;
- reject/cancel/expiry releases exactly that hold;
- receive consumes seller `on_hand` and `reserved` exactly once;
- seller gets a posted credit sale, AR, revenue, COGS, inventory journal and customer ledger entry;
- buyer gets a posted credit purchase, AP, inventory journal, supplier ledger entry and weighted-average inventory update;
- buyer and seller counterparties are created automatically and linked to the opposite network store;
- the procurement row becomes `received` and links both the seller sale and buyer purchase;
- repeating receive returns the already-linked documents instead of posting twice.

## Procurement lifecycle

```text
requested -> accepted -> ready -> received
    |           |
    +-> rejected+
    |
    +-> cancelled (buyer)

requested / accepted / ready -> expired when TTL is reached
```

TTL policy in this phase:

- requested: 12 hours
- accepted: 24 hours
- ready: 48 hours

## Network offer control

`/store/network` now has a separate **تأمین همکار** switch. A visible offer can therefore be shown to mechanics while store-to-store procurement is independently disabled.

## Accounting model

The first procurement version settles the inter-store trade as credit on both sides. This deliberately produces matching operational documents without pretending money moved at receive time. Existing customer/supplier settlement flows can be used later to settle the resulting AR/AP balances.

## Dev restart invariant

`dev/postgres/repair_network_demo_reservations.sql` now rebuilds demo `reserved` quantities from both active mechanic reservations and active procurement holds, so `docker compose up` cannot silently erase a procurement hold.
