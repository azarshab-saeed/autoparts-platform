# Auto Parts Platform — Competitive Roadmap vs Holoo Auto Parts

> **Product north star:** build a system that an Iranian auto-parts store can use as its primary daily operating system, while gaining capabilities that a traditional accounting/POS product does not natively provide: a live parts network, cross-store inventory discovery, mechanic demand, reservations, procurement, OEM/alias/fitment search, and network-driven sales.

## Why this file exists

This document is the persistent product checkpoint for future development sessions. **Before defining or starting a new product phase, read this file first.** Do not restart the roadmap from an older phase or divert the main product sequence into infrastructure-only work unless a blocking technical issue prevents the next product phase.

The goal is not merely to reproduce Holoo accounting screens. The goal is to remove the practical reasons an auto-parts shop keeps Holoo, while preserving and expanding our network advantages.

---

## Current checkpoint

- **Latest completed product checkpoint:** Phase 15.14 — Tax, VAT & Official Invoicing Foundation
- **Next product phase:** **Phase 15.15 — Iranian Modian Integration**
- Phase 15.15 should remain the next phase unless a verified blocker makes it impossible to implement safely.
- Build/dependency/release hardening tasks are important, but they do **not** replace this product roadmap. They should be handled as blocking fixes, maintenance slices, or release gates unless they materially change product capability.

---

## Product strategy

A traditional accounting product is opened primarily to manage the store's own operations.

Our product should be opened to:

1. manage the store's own sales, purchases, inventory, accounting, checks, bank activity and hardware;
2. find parts by OEM, alias and vehicle fitment faster;
3. expose available stock to mechanics and other stores;
4. generate new demand through the parts network;
5. reserve inventory for mechanics/consumers;
6. source missing inventory from another store;
7. convert network activity into measurable sales and profit.

The strategic moat is therefore:

> **Specialized auto-parts ERP/POS + parts network + demand and supply marketplace**

We should not spend the roadmap trying to win only by copying a mature accounting package feature-for-feature. We close the operational gaps that block migration, then use the network to become materially more valuable.

---

## Competitive status after Phase 15.14

| Area | Current status | Strategic position |
|---|---|---|
| Daily sales | Strong | Near parity; modern UX advantage |
| Purchases and inventory | Strong | Near parity |
| Customer/supplier accounts | Strong | Near parity |
| Double-entry accounting / P&L / expenses | Strong | Near parity for store operations |
| Sales/purchase history and reports | Strong | Near parity |
| OEM / alias search | Strong | **Our advantage** |
| Vehicle fitment | Strong | **Our advantage** |
| Cross-store inventory search | Strong | **Core advantage** |
| Mechanic price + distance + availability workflow | Strong | **Core advantage** |
| Inventory reservation | Strong | **Core advantage** |
| Store-to-store procurement | Strong | **Core advantage** |
| Network-driven sales | Strong foundation | **Core moat** |
| Initial CSV/import migration | Implemented | Migration friction reduced |
| Offline POS continuity | Implemented | Strong operational advantage |
| Barcode scanning | Implemented | Competitive requirement closed |
| Receipt/A4/label printing and cash drawer | Implemented | Competitive requirement closed |
| POS provider bridge / Store Agent | Implemented | Competitive requirement closed |
| Received/payable checks | Implemented | Major Holoo gap substantially closed |
| Check spend/endorsement | Implemented | Major Holoo gap substantially closed |
| Bank accounts / due / bounced / cleared checks | Implemented | Major Holoo gap substantially closed |
| Checks UX and pagination | Implemented through 15.10 | Ready for larger real datasets |
| Advanced multi-price rules | **Implemented in 15.11** | Migration blocker closed |
| Wholesale/retail/mechanic pricing | **Implemented in 15.11** | Customer + quantity aware pricing |
| Multi-unit / pack / carton handling | **Implemented in 15.12** | Migration blocker closed; canonical base-stock model |
| New-product barcode/package setup | **Implemented in 15.12** | Faster setup than fragmented legacy workflows |
| Check maturity averaging (راس‌گیری) | **Implemented in 15.13** | Professional finance gap closed |
| Bank reconciliation | **Implemented in 15.13** | Full/partial matching + audited undo |
| Future check cash-flow / customer risk | **Implemented in 15.13** | Forward-looking management advantage |
| Tax/VAT and official invoicing | **Implemented foundation in 15.14** | Local calculation, accounting, snapshots and official document blocker substantially closed |
| Iranian Modian integration | **Missing** | Major migration blocker |
| Advanced invoice/document designer | **Missing** | Important operational gap |
| User performance / anomaly intelligence | Limited | Opportunity to exceed traditional software |

---

# Planned product phases

## Phase 15.11 — Advanced Pricing & Trade Terms

**Status:** DONE

### Objective

Remove the "I need different prices for retail, mechanics, colleagues and wholesale" migration blocker, while making pricing safer and more intelligent than a traditional POS.

### Scope

#### 1. Configurable price lists

Support tenant-defined price lists rather than hard-coded fields. Examples:

- Retail
- Mechanic
- Colleague / trade
- Wholesale
- VIP
- Dealer

Each product can have prices for one or more lists.

#### 2. Customer default price list

A customer/party may have a default price list. Selecting the customer in POS automatically selects the correct pricing context.

#### 3. Quantity-based pricing

Support volume breaks, for example:

```text
1–4        850,000
5–9        820,000
10+        790,000
```

The rule engine must resolve the applicable price deterministically.

#### 4. Line and invoice discounts

Support both percentage and fixed-amount discounts at:

- sale-line level;
- invoice level.

Accounting and reports must preserve gross value, discount and net value correctly.

#### 5. Margin guard

Use inventory cost / weighted-average cost to calculate margin before finalizing a sale.

Capabilities:

- configurable minimum margin policy;
- warning below threshold;
- permission-based override;
- optional override reason;
- owner/admin visibility into below-margin sales.

#### 6. Price override audit

Persist enough data to explain a manual price change later, including concepts such as:

- selected price list;
- original/resolved unit price;
- final unit price;
- applied discount;
- override actor;
- override reason;
- timestamp / audit event.

#### 7. Separate local and network commercial prices

Do not assume store POS price equals network selling/supply price.

A store may expose, for example:

```text
Retail in store       850,000
Mechanic              820,000
Wholesale             780,000
Network mechanic      830,000
Network trade supply  770,000
```

This separation is strategically important because our network business model is not equivalent to a classic local price list.

### Acceptance direction

- Price resolution is tenant-safe and deterministic.
- Existing sales remain backward compatible.
- Historical invoices never change when a price list changes later.
- Cashier cannot silently bypass owner pricing policy.
- Network prices cannot accidentally leak a private local wholesale tier.
- Discount/margin effects are reflected correctly in accounting and reports.

### Delivered in Phase 15.11

- tenant/store-scoped configurable price lists with a canonical default retail list;
- customer default price-list assignment;
- deterministic quantity-break resolution with default-list fallback;
- POS automatic repricing by customer and quantity;
- line and invoice discount UX using persisted final unit prices;
- immutable gross, discount and net sale facts for history/reporting;
- minimum-margin policy with server-side enforcement, mandatory reason and actor audit for Owner/Admin exceptions;
- persisted list price, final price, source, override reason, override actor and historical margin-guard decision;
- local price lists remain separate from `store_product_offers`, so private wholesale/mechanic tiers are not published to the parts network;
- imported selling price seeds the local default retail tier without coupling later local-tier edits to network offers;
- Store Edge receives default-retail quantity breaks and cashier override policy, so offline quantity changes keep the same pricing behavior and manual overrides cannot bypass store policy.
- Main web POS preserves an already-resolved mechanic/wholesale/customer-tier price plus `customer_id` through the offline queue; Cloud revalidates it against current server pricing on sync.

---

## Phase 15.12 — Multi-Unit & Packaging

**Status:** DONE

### Objective

Support how auto parts are actually purchased, stocked and sold: piece, pair, set, pack, box/carton and other tenant-defined units.

### Scope

- base stock unit per product;
- alternate sale/purchase units;
- conversion factors;
- piece / pair / set / pack / carton-style workflows;
- barcode per packaging unit where required;
- purchase in carton, sale in piece;
- inventory always reconciled to a canonical base quantity;
- price per unit/packaging;
- printable documents showing the commercial unit used;
- import/export support for units and conversions;
- validation preventing impossible/fractional conversions when product policy disallows them.

### Delivered in Phase 15.12

- canonical base-stock quantity per product plus commercial sale/purchase units;
- piece, pair, set, pack, box, carton and tenant/product-defined unit codes/names;
- alternate-unit conversion factors with integer/fractional-stock policy validation;
- tenant-safe barcode per active packaging unit;
- **New Product Wizard** that creates product + base unit + package units + barcodes + initial retail prices in one workflow;
- USB barcode-scanner friendly inputs, manual barcode entry and internally generated EAN-13-style barcodes;
- Store Edge label-print action directly after product/unit setup;
- dedicated unit/barcode management page after product creation;
- purchase in carton/pack while weighted-average inventory cost remains canonical per base unit;
- sale in piece/pair/set/carton with commercial quantity and immutable conversion facts on sale lines;
- sales and purchase returns preserve the original commercial unit while inventory movement remains base-unit correct;
- package-specific price breaks, with deterministic fallback to base-unit price × conversion factor when a package has no dedicated price;
- printable sale/purchase details show the commercial unit used on the document;
- inventory screens explicitly display and adjust the base stock unit;
- CSV import/export supports package code, name, conversion factor, barcode and sale/purchase permissions; imported products receive canonical base units;
- existing products and existing Phase 15.11 prices are migrated to their canonical base unit without changing historical meaning;
- Store Edge snapshot/search/offline queue is packaging-aware: alternate barcodes resolve locally, package sales reduce base inventory, package pricing works offline, and `product_unit_id` survives cloud sync;
- pending offline package sales are replayed against fresh snapshots using base quantity, preventing double availability;
- backward compatibility for pre-15.12 Store Edge snapshots/queued sales is preserved.

### Competitive outcome

A wholesaler or mature parts shop should not need a second system merely because their supplier invoices cartons while counter sales happen by piece/set.

---

## Phase 15.13 — Bank Reconciliation & Check Intelligence

**Status:** DONE

### Objective

Move checks/banking from "recording transactions" to a professional financial control surface.

### Scope

- check maturity averaging / **راس‌گیری چک**;
- expected future cash-in/cash-out calendar;
- check maturity buckets and risk warnings;
- bank statement/manual transaction reconciliation;
- matched / unmatched / partially matched flows where applicable;
- duplicate detection assistance;
- reconciliation audit trail;
- bank/check dashboard for owner/accountant;
- overdue and bounced exposure by customer;
- optional customer credit/risk indicators based on actual history.

### Delivered in Phase 15.13

- weighted check maturity averaging (راس‌گیری) for selected receivable or payable checks with Jalali/Gregorian reference-date input;
- future check cash-in/cash-out calendar with projected bank balance and maturity buckets;
- owner/accountant dashboard for open/overdue receivable and payable exposure;
- customer check-risk indicators using overdue amount, bounced history, bounce rate and maximum overdue age;
- tenant/store-scoped bank statement import with exact duplicate fingerprint protection;
- reconciliation candidate assistance from real bank GL journal entries, same direction and ±14-day proximity;
- full and partial reconciliation without mutating or recreating accounting journals;
- duplicate-suspected statement-line flagging to assist manual review;
- reconciliation history plus audited undo/reopen for mistaken matches;
- immutable reconciliation event audit for import, match and unmatch actions;
- dedicated **Finance Control** UI for dashboard, maturity averaging and bank reconciliation.

### Competitive outcome

Close one of the strongest reasons check-heavy wholesalers stay on legacy accounting software, then add forward-looking cash intelligence on top.

---

## Phase 15.14 — Tax, VAT & Official Invoicing Foundation

**Status:** DONE

### Objective

Make the transaction model legally and operationally ready for official Iranian invoicing and tax workflows before coupling the core sale engine to Modian transport/API concerns.

### Scope

- tenant legal/economic identity fields;
- customer legal identity fields needed for official invoices;
- tax/VAT configuration with effective dates;
- tax-exempt/non-taxable handling where applicable;
- official vs normal/non-official sales document mode as permitted by the business rules;
- invoice numbering/series policy;
- tax totals persisted as immutable invoice facts;
- correction/cancellation prerequisites;
- printable official invoice dataset;
- reporting/export foundation for accountant workflows.

### Important implementation rule

Tax behavior must be configuration/version based. Historical invoices must not recalculate because a future tax rate or tenant setting changes.

### Delivered in Phase 15.14

- versioned tax codes/rates and effective-date resolution;
- tenant seller identity + customer tax identity;
- normal/official sale mode and atomic local official numbering;
- immutable seller/buyer/tax snapshots;
- separate VAT payable accounting;
- tax-aware sale return reversal;
- official invoice register, CSV foundation and printable snapshot;
- local correction/cancellation request prerequisites;
- POS tax-aware totals with online-only enforcement for official/tax-applied sales.

---

## Phase 15.15 — Iranian Modian Integration

**Status:** PLANNED

### Objective

Remove the "I still need Holoo to send my official invoices" blocker.

### Scope direction

Implementation must be verified against the **current official Iranian tax/Modian specification at implementation time**. Do not implement this phase from old assumptions or stale protocol documents.

Expected product capabilities:

- invoice eligibility and validation before submission;
- canonical Modian payload generation;
- credentials/signing/configuration handling as required by the current protocol;
- submit/send workflow;
- durable submission state machine;
- retry and idempotency;
- reference/tracking identifiers;
- accepted/rejected/pending/error visibility;
- user-readable error mapping;
- correction/cancellation flows supported by the official model;
- complete audit trail;
- operational dashboard for unsent/rejected invoices;
- no loss of local sale/accounting correctness when the external service is unavailable.

### Architecture rule

The sale posting transaction must not depend on synchronous external Modian availability. Use a durable outbox/submission workflow so the store can continue operating during external outages.

---

## Phase 15.16 — Invoice Designer & Store Documents

**Status:** PLANNED

### Objective

Give stores enough document flexibility that printer/layout habits are no longer a migration blocker.

### Scope

- multiple document templates;
- A4 and receipt-oriented layouts;
- logo/store identity;
- configurable visible columns;
- optional OEM/vehicle/barcode/discount/tax fields;
- header/footer notes;
- stamp/signature areas;
- official vs normal invoice templates;
- sales, purchase, return and settlement document coverage where relevant;
- preview before print;
- printer-friendly deterministic output;
- permissioned template management.

### Constraint

Avoid turning the application into a free-form desktop publishing tool. Prefer safe configurable templates/blocks with deterministic print output.

---

## Phase 15.17 — Management Intelligence & User Performance

**Status:** PLANNED

### Objective

Go beyond legacy accounting reports and help the owner make better decisions using operational and network data.

### Scope

- sales and gross-profit performance by user/cashier;
- discount/override analysis;
- below-margin sale report;
- suspicious or unusual transaction signals;
- returns/cancellations by user;
- top/declining customers;
- customer inactivity / churn-risk signals;
- dead/slow inventory actions;
- reorder suggestions using sales velocity and network demand;
- missed-sale / unavailable-search signals where data exists;
- network-driven revenue attribution;
- mechanic/customer acquisition from network;
- store-to-store procurement savings/opportunity metrics;
- actionable owner dashboard, not only passive charts.

### Competitive outcome

At this point the product should answer not only "what happened?" but increasingly "what should I do next?" using information a standalone accounting package cannot easily observe.

---

# Product gates for "better than Holoo"

The roadmap is successful only if we can satisfy all three layers below.

## Gate A — Safe replacement

A mature store can run daily operations without keeping another system open for critical workflows:

- sales/purchases/inventory;
- receivables/payables;
- accounting/expenses/P&L;
- checks and bank workflows;
- multi-price and wholesale terms;
- packaging/multi-unit inventory;
- barcode/POS/printing;
- tax/official invoices/Modian where required;
- reliable import/migration of starting data.

## Gate B — Lower migration risk

For an established Holoo user, switching must feel operationally safe:

- import tools are repeatable and validated;
- historical/opening balances are preserved correctly;
- hardware works;
- offline sales continue during connectivity loss;
- permissions and audit are strong;
- accounting can explain every posted operational document;
- backups/restore and release procedures are proven.

## Gate C — A reason to switch, not merely parity

The owner can measure value that did not exist in the old workflow:

- new mechanic demand;
- cross-store availability search;
- reservations converted to sales;
- store-to-store sourcing;
- network revenue;
- faster part identification through OEM/alias/fitment;
- smarter pricing/margin control;
- demand-aware inventory decisions.

**Parity gets permission to replace Holoo. Network value creates the reason to replace it.**

---

# Prioritization rules for future phases

When choosing work, use this order:

1. **Migration blockers:** something that forces a target store to keep Holoo open every day.
2. **Money-path correctness:** sales, purchasing, inventory, accounting, checks, tax and settlement correctness.
3. **Network moat:** capabilities that generate or fulfill demand across stores/mechanics.
4. **Adoption friction:** import, hardware, offline mode, UX and training reduction.
5. **Management intelligence:** capabilities that help the owner outperform a traditional accounting workflow.
6. **Infrastructure hardening:** do it immediately when it blocks or endangers the above; otherwise keep it as a release/maintenance track rather than replacing the product sequence.

---

# Phase sequencing rule

Unless this document is intentionally revised, continue in this order:

```text
15.9.1  Checks UX Hotfix                         DONE
15.10   Checks Pagination & Acceptance           DONE
15.11   Advanced Pricing & Trade Terms            DONE
15.12   Multi-Unit & Packaging + Product Wizard   DONE
15.13   Bank Reconciliation & Check Intelligence  DONE
15.14   Tax, VAT & Official Invoicing Foundation  DONE
15.15   Iranian Modian Integration                NEXT
15.16   Invoice Designer & Store Documents        PLANNED
15.17   Management Intelligence & User Performance PLANNED
```

If a future session proposes a different next product phase, it should first explain which verified blocker or changed product priority justifies changing this sequence, then update this file in the same change.

---

# Maintenance of this roadmap

At the end of each completed phase:

1. mark the phase `DONE`;
2. move `NEXT` to the following phase;
3. update the competitive status table if a gap was closed;
4. add or revise acceptance outcomes based on what was actually implemented;
5. keep release notes/runbooks phase-specific; do not use them as a substitute for this roadmap;
6. do not silently renumber or reorder planned phases without documenting why.

This file is the canonical product-direction checkpoint for continuing development across ChatGPT/Codex sessions.
