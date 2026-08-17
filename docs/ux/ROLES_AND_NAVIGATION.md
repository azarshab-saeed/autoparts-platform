# Roles & Navigation

## 1. Store Admin / Owner
هدف: اداره روزانه فروشگاه بدون مواجهه با اصطلاحات پیچیده حسابداری.

Navigation اصلی:
1. داشبورد
2. فروش
3. خرید
4. انبار
5. مشتریان
6. تامین‌کنندگان
7. حساب‌ها
8. جست‌وجوی شبکه
9. گزارش‌ها
10. تنظیمات

Quick Actions:
- فروش جدید
- خرید جدید
- دریافت پول
- پرداخت پول
- جست‌وجوی قطعه
- اصلاح موجودی

## 2. Store Cashier
دسترسی:
- فروش
- مشتریان
- دریافت پول
- جست‌وجوی کالا

محدودیت:
- بدون مشاهده سود کل فروشگاه
- بدون تنظیمات مالی حساس
- بدون حذف/اصلاح سند نهایی‌شده مگر با مجوز

## 3. Store Warehouse
دسترسی:
- انبار
- رسید خرید
- انتقال/اصلاح موجودی
- موجودی کم

## 4. Mechanic
Navigation موبایل:
1. خانه / جست‌وجو
2. استعلام‌ها
3. سفارش‌ها
4. فروشگاه‌های منتخب
5. حساب من

اصل UX:
مکانیک نباید داشبورد ERP ببیند. باید در چند ثانیه به «چه کسی دارد، چند است، چقدر دور است، رزرو می‌شود یا نه» برسد.

## 5. Consumer
Navigation موبایل:
1. جست‌وجو
2. خودروهای من
3. سفارش‌ها
4. پروفایل

اصل UX:
اطلاعات فنی پیچیده فقط در جزئیات نمایش داده شود. UI اولیه بر سازگاری، قیمت، اعتبار، فاصله و موجودی متمرکز است.

## Phase 8 additions

### Mechanic / consumer
- `/mechanic` — public network search; login is required only when reserving.
- `/mechanic/orders` — authenticated reservation tracker, status and cancellation.

### Store roles
- `/store/orders` — incoming network reservations.
- owner/admin/cashier/warehouse can view and transition reservations.
- accounting data remains outside the reservation workflow until fulfillment becomes a posted sale.

## Phase 10 additions

### Owner / Admin / Accountant
- `/store/expenses` — ثبت و مشاهده هزینه‌های عملیاتی
- `/store/reports` — سود و زیان فروشگاه
- `/store/accounts` — مشاهده گردش حساب مشتری و تأمین‌کننده

Expense/report navigation is hidden for cashier and warehouse roles. Party settlement/statement permissions remain role-checked in the API.
