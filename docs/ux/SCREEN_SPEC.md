# UX / Screen Specification V1

## قواعد مشترک UI
- RTL و فارسی به‌صورت native.
- Desktop-first برای Store Admin و Mobile-first برای Mechanic/Consumer.
- هر عملیات اصلی حداکثر 2 تا 4 گام.
- هیچ عملیات مالی با دکمه مبهم مثل «ثبت» تنها نباشد؛ متن باید نتیجه را روشن کند، مثل «ثبت فروش و دریافت».
- مبالغ با جداکننده هزارگان و واحد مشخص.
- وضعیت موجودی با متن + رنگ؛ رنگ تنها نشانه نباشد.
- زمان آخرین بروزرسانی موجودی شبکه همیشه قابل مشاهده باشد.
- حذف سند قطعی ممنوع؛ از cancel/reverse استفاده شود.

---

# Store Admin

## S-01 Dashboard
هدف: صاحب فروشگاه در کمتر از 10 ثانیه وضعیت روز را بفهمد.

نمایش:
- فروش امروز
- سود تقریبی امروز
- خرید امروز
- دریافتی امروز
- فروش نسیه
- طلب مشتریان
- بدهی تامین‌کنندگان
- تعداد کالاهای کم‌موجود
- سفارش/رزرو جدید از شبکه

Widgets:
- فروش 7 روز اخیر
- کالاهای رو به اتمام
- آخرین فروش‌ها
- پرفروش‌ها
- فعالیت شبکه: نمایش/استعلام/رزرو

Actions:
- فروش جدید
- خرید جدید
- دریافت پول
- پیدا کردن قطعه

API:
- GET /v1/store/dashboard
- GET /v1/inventory/low-stock
- GET /v1/sales/recent

States:
- first day: Empty dashboard + CTA برای وارد کردن کالا یا فروش جدید
- partial data: کارت‌هایی که داده ندارند صفر یا «هنوز داده‌ای نیست» نشان دهند

---

## S-02 Sales List
نمایش:
- شماره فروش
- تاریخ/ساعت
- مشتری
- تعداد اقلام
- مبلغ کل
- پرداخت شده
- مانده
- وضعیت
- کاربر ثبت‌کننده

Filters:
- تاریخ
- مشتری
- وضعیت
- روش پرداخت
- کاربر

Actions:
- فروش جدید
- مشاهده
- چاپ/اشتراک فاکتور
- مرجوعی
- لغو با مجوز

API:
- GET /v1/sales
- POST /v1/sales
- GET /v1/sales/{id}

---

## S-03 New Sale
UX Flow:
1. مشتری اختیاری/الزامی برای نسیه
2. جست‌وجوی کالا با نام، کد، بارکد
3. انتخاب تعداد و قیمت
4. تخفیف اختیاری
5. انتخاب روش پرداخت
6. تایید نهایی

Screen layout:
- نوار جست‌وجوی بزرگ در بالا
- cart در سمت مقابل
- موجودی فعلی کنار هر item
- total sticky در پایین

Payment modes:
- cash
- card
- credit
- mixed

Validation:
- تعداد > 0
- موجودی کافی مگر فروش منفی در tenant policy مجاز باشد
- فروش نسیه بدون مشتری ثبت‌شده ممنوع
- mixed باید جمع پرداخت‌ها = amount paid

API:
- GET /v1/products/search?q=
- GET /v1/customers/search?q=
- POST /v1/sales

Success:
- نمایش شماره فاکتور
- مبلغ پرداختی/مانده
- دکمه چاپ/ارسال/فروش بعدی

---

## S-04 Purchases List
نمایش:
- شماره خرید
- تامین‌کننده
- تاریخ
- مبلغ
- پرداخت شده
- مانده
- وضعیت رسید انبار

API:
- GET /v1/purchases
- POST /v1/purchases
- GET /v1/purchases/{id}

---

## S-05 New Purchase
Flow:
1. تامین‌کننده
2. کالا
3. تعداد و قیمت خرید
4. هزینه جانبی اختیاری
5. روش پرداخت
6. ثبت خرید و ورود به انبار

Rules:
- خرید finalized موجودی را افزایش می‌دهد.
- تغییر قیمت خرید، تاریخچه قیمت را ثبت می‌کند.
- اگر payable وجود داشته باشد، حساب تامین‌کننده تغییر می‌کند.

API:
- GET /v1/suppliers/search?q=
- GET /v1/products/search?q=
- POST /v1/purchases

---

## S-06 Inventory
Tabs:
- همه کالاها
- کم‌موجود
- ناموجود
- رزروشده
- گردش موجودی

Columns:
- کالا
- SKU / کد فنی
- برند
- انبار
- موجودی فیزیکی
- رزرو
- قابل فروش
- قیمت فروش
- آخرین خرید
- محل قفسه

Actions:
- اصلاح موجودی
- انتقال بین انبارها
- مشاهده گردش
- تغییر حداقل موجودی
- انتشار/عدم انتشار در شبکه

API:
- GET /v1/inventory
- GET /v1/inventory/{productId}/movements
- POST /v1/inventory/adjustments

---

## S-07 Customers
نمایش:
- نام
- تلفن
- نوع مشتری
- بدهی فعلی
- خرید ماه
- آخرین خرید

Customer detail:
- مانده واضح: «به شما بدهکار است» یا «از شما طلب دارد»
- گردش حساب
- فروش‌ها
- دریافت‌ها

Actions:
- فروش جدید
- دریافت پول
- افزودن یادداشت

API:
- GET /v1/customers
- POST /v1/customers
- GET /v1/customers/{id}
- POST /v1/customers/{id}/receipts
- GET /v1/customers/{id}/statement

---

## S-08 Suppliers
مشابه Customers ولی با:
- خریدها
- پرداخت‌ها
- بدهی ما به تامین‌کننده

API:
- GET /v1/suppliers
- POST /v1/suppliers
- GET /v1/suppliers/{id}
- POST /v1/suppliers/{id}/payments
- GET /v1/suppliers/{id}/statement

---

## S-09 Accounts
هدف: UI ساده، موتور حسابداری دقیق.

نمایش ساده:
- صندوق
- بانک‌ها
- کارت‌خوان‌ها
- طلب از مشتریان
- بدهی به تامین‌کنندگان
- هزینه‌های امروز

Advanced section با permission:
- دفتر روزنامه
- دفتر کل
- chart of accounts

API:
- GET /v1/accounts/summary
- GET /v1/accounts
- GET /v1/journals

---

## S-10 Network Search for Store
Search:
- نام بازاری
- کد فنی/OEM
- خودرو

Result card:
- فروشگاه
- فاصله
- موجودی قابل فروش
- قیمت
- برند
- freshness
- trust score

Actions:
- درخواست/رزرو
- تماس
- مسیریابی

API:
- GET /v1/network/search
- POST /v1/network/reservations

---

## S-11 Reports
V1:
- فروش روزانه/ماهانه
- سود تقریبی
- پرفروش‌ها
- کم‌گردش‌ها
- مانده مشتریان
- مانده تامین‌کنندگان
- موجودی به ارزش خرید

API:
- GET /v1/reports/sales
- GET /v1/reports/inventory
- GET /v1/reports/receivables
- GET /v1/reports/payables

---

## S-12 Settings
Sections:
- اطلاعات فروشگاه
- کاربران و نقش‌ها
- انبارها
- صندوق/بانک/کارت‌خوان
- سیاست موجودی منفی
- سیاست انتشار در شبکه
- شماره‌گذاری اسناد
- import data

API:
- GET /v1/store/settings
- PATCH /v1/store/settings
- GET /v1/users
- POST /v1/users

---

# Mechanic

## M-01 Home / Search
هدف: پاسخ سریع به نیاز قطعه.

UI:
- Search بزرگ: «چه قطعه‌ای می‌خوای؟»
- خودرو انتخابی اختیاری
- Recent searches
- سفارش‌های باز

API:
- GET /v1/network/search
- GET /v1/me/orders?status=open

---

## M-02 Search Results
Filters:
- خودرو
- برند
- موجود الان
- نزدیک‌ترین
- ارزان‌ترین

Card:
- نام استاندارد + نام بازاری
- برند
- قیمت
- فروشگاه
- فاصله
- موجودی
- آخرین بروزرسانی
- امتیاز/اعتماد

Actions:
- رزرو
- تماس
- مسیریابی
- جزئیات

---

## M-03 Part Detail
نمایش:
- تصاویر
- سازگاری خودرو
- OEM/کد فنی در بخش پیشرفته
- فروشگاه‌های موجود
- جایگزین‌ها

---

## M-04 Reservation
Flow:
1. انتخاب فروشگاه
2. تعداد
3. تایید تلفن/هویت در صورت نیاز
4. زمان انقضا
5. ثبت

Status:
- pending
- confirmed
- ready
- picked_up
- expired
- cancelled

API:
- POST /v1/network/reservations
- GET /v1/me/reservations
- DELETE /v1/me/reservations/{id}

---

## M-05 Multi-item Inquiry
هدف: مکانیک لیست چند قلم را یکجا بفرستد.

Input:
- چند قطعه + تعداد

Output:
- فروشگاه‌هایی که بیشترین coverage را دارند
- کمترین total price
- نزدیک‌ترین full-fill option

API future:
- POST /v1/network/quotes

---

# Consumer

## C-01 Search
تفاوت با مکانیک:
- خودروهای من برجسته‌تر
- jargon کمتر
- پیشنهاد برند و سازگاری برجسته‌تر

## C-02 Result / Detail
اولویت اطلاعات:
1. سازگار با خودروی من؟
2. قیمت
3. اعتبار فروشگاه
4. فاصله/ارسال
5. موجودی

Technical info collapsed by default.

## Phase 8 — Mechanic reservation

### Search result card
Additional action when `allow_reservation=true`:
- `رزرو قطعه`
- opens quantity confirmation
- displays current availability, unit price, estimated total, and 30-minute hold rule
- unauthenticated users are redirected to Keycloak and return to the mechanic route

### `/mechanic/orders`
Data:
- product / store
- qty / price
- status
- reservation expiry / pickup expiry
- store phone
Actions:
- cancel while pending, accepted, or ready

### `/store/orders`
Data:
- mechanic/customer identity snapshot
- product / qty / amount
- created time / expiry
- status
Actions:
- pending -> accepted
- pending/accepted -> rejected
- accepted -> ready

No `complete` action exists in Phase 8 because completion must post/attach an actual sale atomically.

## Phase 10 — Expenses

Route: `/store/expenses`

- Date range and category filter.
- Total expense KPI for the selected period.
- Expense table: date, category, payment channel, note, amount.
- `ثبت هزینه` modal: category, amount, date, cash/card, note.
- User records the business action only; the debit/credit journal is automatic.

## Phase 10 — Profit & Loss

Route: `/store/reports`

- Date range selector.
- KPIs: net sales, gross profit, operating expenses, net operating profit.
- Statement rows: gross sales, sales returns, net sales, COGS, reversed COGS, net COGS, gross profit, expenses, net profit.
- Expense breakdown by category.

## Phase 10 — Party Statement

Route: `/store/accounts`

- `گردش حساب` is available per customer/supplier.
- Statement shows chronological ledger movements and running balance.
- Positive customer balance means customer owes the store.
- Positive supplier balance means store owes the supplier.
