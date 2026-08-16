# UI API Contract V1

این سند قرارداد سطح UI است. implementation ممکن است تغییر کند، اما semantics و error behavior نباید بدون versioning تغییر کند.

## Conventions
Base URL:
`/v1`

Headers:
- Authorization: Bearer <token>
- X-Tenant-ID: فقط برای internal/admin contexts؛ tenant عادی از token resolve شود.
- Idempotency-Key: برای عملیات مالی/رزرو قابل تکرار.

Money:
- integer minor unit یا integer تومان؛ در V1 پروژه باید یک convention واحد انتخاب و در DB/API enforce شود.
- فعلاً API examples از integer `amount` استفاده می‌کنند.

Dates:
- RFC3339 UTC در API
- تبدیل timezone در UI

Pagination:
```json
{
  "items": [],
  "next_cursor": "opaque-or-null"
}
```

Error envelope:
```json
{
  "error": {
    "code": "INSUFFICIENT_STOCK",
    "message": "موجودی کافی نیست",
    "field": "items[0].quantity",
    "details": {}
  }
}
```

Core error codes:
- VALIDATION_ERROR
- UNAUTHORIZED
- FORBIDDEN
- NOT_FOUND
- CONFLICT
- INSUFFICIENT_STOCK
- DUPLICATE_REQUEST
- CUSTOMER_REQUIRED_FOR_CREDIT
- RESERVATION_EXPIRED
- TENANT_SCOPE_VIOLATION

## Store Dashboard
GET /store/dashboard

Response:
```json
{
  "sales_today": 87000000,
  "gross_profit_today": 14200000,
  "purchases_today": 38000000,
  "received_today": 72000000,
  "credit_sales_today": 15000000,
  "receivables": 124000000,
  "payables": 82000000,
  "low_stock_count": 7,
  "network_new_orders": 3
}
```

## Product Search
GET /products/search?q=lent%20206&warehouse_id=...

Response item:
```json
{
  "product_id": "uuid",
  "name": "لنت جلو پژو 206 تیپ 5",
  "brand": "Textar",
  "sku": "TXT-206-F",
  "available_qty": 6,
  "sale_price": 1800000
}
```

## Customers Search
GET /customers/search?q=ahmadi

## Suppliers Search
GET /suppliers/search?q=...

## Create Sale
POST /sales
Header: Idempotency-Key required

Request:
```json
{
  "customer_id": "uuid-or-null",
  "warehouse_id": "uuid",
  "items": [
    {
      "product_id": "uuid",
      "quantity": 2,
      "unit_price": 1800000,
      "discount": 0
    }
  ],
  "payments": [
    {
      "method": "card",
      "account_id": "uuid",
      "amount": 3600000
    }
  ]
}
```

Response 201:
```json
{
  "sale_id": "uuid",
  "sale_no": "S-1405-000123",
  "total": 3600000,
  "paid": 3600000,
  "balance": 0,
  "status": "posted"
}
```

Rules:
- credit amount > 0 => customer_id required
- transaction must atomically persist sale + items + inventory movements + balance/accounting events
- duplicate Idempotency-Key returns same logical result

## Sales List
GET /sales?from=&to=&customer_id=&status=&cursor=

## Create Purchase
POST /purchases
Header: Idempotency-Key required

Request mirrors sale but supplier + purchase prices.

## Inventory List
GET /inventory?q=&warehouse_id=&stock_state=&cursor=

stock_state:
- all
- low
- out
- reserved

## Inventory Adjustment
POST /inventory/adjustments

Request:
```json
{
  "warehouse_id": "uuid",
  "product_id": "uuid",
  "new_quantity": 12,
  "reason": "physical_count"
}
```

## Customer Receipt
POST /customers/{id}/receipts

## Supplier Payment
POST /suppliers/{id}/payments

## Network Search
GET /network/search?q=&vehicle_id=&brand_id=&lat=&lng=&sort=&in_stock=true

sort:
- relevance
- nearest
- cheapest
- freshest

Response:
```json
{
  "query": "لنت جلو 206",
  "items": [
    {
      "listing_id": "uuid",
      "product": {
        "canonical_name": "لنت جلو پژو 206",
        "brand": "Textar"
      },
      "store": {
        "id": "uuid",
        "name": "یدکی رضایی",
        "distance_meters": 850,
        "rating": 4.8
      },
      "price": 1780000,
      "available_qty": 4,
      "inventory_updated_at": "2026-08-15T06:00:00Z",
      "availability_confidence": "high"
    }
  ]
}
```

## Create Reservation
POST /network/reservations
Header: Idempotency-Key required

Request:
```json
{
  "listing_id": "uuid",
  "quantity": 1
}
```

Response:
```json
{
  "reservation_id": "uuid",
  "status": "pending",
  "expires_at": "2026-08-15T07:00:00Z"
}
```

## API Contract Rule for Frontend
Frontend نباید business rule حیاتی را فقط در client enforce کند. Client validation برای UX است؛ backend source of truth است.
