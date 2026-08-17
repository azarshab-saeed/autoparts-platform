# Phase 15.7 — Store Edge & Offline Sales Continuity

هدف این فاز این است که قطع اینترنت صندوق فروشگاه را متوقف نکند. Store Edge یک Agent کوچک Go است که فقط روی loopback (`127.0.0.1:17624`) گوش می‌دهد، کاتالوگ و موجودی فروشگاه را محلی نگه می‌دارد و فروش کامل نقد/کارت را روی دیسک به‌صورت durable ثبت می‌کند.

## معماری

```text
Cloud API/PostgreSQL
       ↑ ↓ device-auth sync
AutoParts Store Edge (Go)
       ↑ ↓ localhost
Web UI / Offline POS
```

- Pairing با کد یک‌بارمصرف ۱۰ دقیقه‌ای انجام می‌شود.
- secret دستگاه فقط یک‌بار به Agent تحویل می‌شود و در سرور فقط hash نگهداری می‌شود.
- Agent به‌صورت پیش‌فرض روی loopback است و از LAN قابل دسترسی نیست.
- فایل‌های `config.json` و `state.json` با atomic rename + fsync نوشته می‌شوند.
- هر فروش محلی `LOCAL-YYYYMMDD-NNNNNN` و `local_operation_id` مستقل دارد.
- Sync سرور با idempotency key مشتق از device/local id انجام می‌شود؛ replay دوباره فروش نمی‌سازد.
- فروش نسیه/ترکیبی آفلاین عمداً بسته است.
- conflict سرور باعث حذف فروش محلی نمی‌شود؛ رکورد در حالت `conflict` باقی می‌ماند تا اپراتور بررسی کند.

## Migration

`014_store_edge_offline.sql`

## اجرای توسعه

```bash
make edge-test
make edge-run
```

سپس باز کن:

```text
http://127.0.0.1:17624/
```

## Pair کردن

1. با owner/admin وارد `/store/edge` شو.
2. «ساخت کد اتصال» را بزن.
3. صفحه محلی Store Edge را باز کن و Cloud URL، نام دستگاه و Pair Code را وارد کن.
4. بعد از Pair، Agent snapshot کاتالوگ را می‌گیرد.

## Smoke test قطع اینترنت

1. Agent و Cloud را در حالت آنلاین Sync کن.
2. صفحه محلی `http://127.0.0.1:17624/` را باز بگذار.
3. شبکه/Cloud API را موقتاً قطع کن (Agent را نبند).
4. یک فروش نقدی ثبت کن؛ باید فوراً شماره `LOCAL-*` بگیرد و `pending` شود.
5. Agent را restart کن؛ فروش باید هنوز در صف باشد و موجودی محلی کاهش‌یافته حفظ شود.
6. Cloud را برگردان و «همگام‌سازی» را بزن.
7. فروش باید `synced` شود و در `/store/sales/history` دیده شود.
8. `make rc-db-check` باید edge provenance/sync reconciliation را PASS کند.

## Windows build/install

```powershell
powershell -ExecutionPolicy Bypass -File edge/windows/build.ps1
powershell -ExecutionPolicy Bypass -File edge/windows/install.ps1
```

Installer فعلی user-level است و admin نمی‌خواهد. Agent در Startup کاربر قرار می‌گیرد و shortcut «AutoParts Offline POS» روی Desktop ساخته می‌شود. حذف معمولی local data را نگه می‌دارد؛ برای پاک‌کردن کامل:

```powershell
powershell -ExecutionPolicy Bypass -File edge/windows/uninstall.ps1 -RemoveData
```

## محدودیت آگاهانه

این فاز distributed stock conflict را پنهان نمی‌کند. اگر فروشگاه در زمان outage فروش فیزیکی داشته باشد ولی Cloud همزمان همان موجودی را مصرف کرده باشد، Sync ممکن است 409 بدهد. Store Edge فروش فیزیکی را نگه می‌دارد و آن را `conflict` می‌کند؛ Phaseهای بعدی می‌توانند workflow reconciliation خودکار/نیمه‌خودکار اضافه کنند.
