# Phase 15.8 — Store Hardware Bridge

Phase 15.8 روی Store Edge فاز 15.7 سوار می‌شود و Agent محلی را به gateway سخت‌افزار صندوق تبدیل می‌کند. Cloud API مستقیم به USB/Printer/POS دست نمی‌زند؛ تمام integration محلی از loopback Agent عبور می‌کند.

## قابلیت‌ها

### فیش‌پرینتر

Transportهای اولیه:

- `windows_spool_text`: چاپ Unicode از طریق Windows printer driver؛ انتخاب پیشنهادی برای متن فارسی.
- `tcp9100`: ارسال raw به چاپگر شبکه.
- `windows_share`: ارسال raw به printer share ویندوز.
- `file`: QA/debug output.

Language:

- `text`
- `escpos`

**نکته:** ESC/POS خام الزاماً روی همه مدل‌ها فارسی Unicode را درست چاپ نمی‌کند. برای فروشگاه ویندوزی، driver mode نقطه شروع امن‌تر است. Raster/Persian-specific adapters می‌توانند برای مدل‌های تاییدشده بعداً اضافه شوند.

### بارکدخوان

Keyboard-Wedge scanner نیاز به driver اختصاصی ندارد: در `/store/sales` کد اسکن‌شده وارد search می‌شود و Enter اسکنر، تطبیق دقیق barcode/SKU/OEM را به فاکتور اضافه می‌کند.

### لیبل

- ZPL با `^CI28`
- TCP 9100 / Windows raw share / file QA

برای چاپ فارسی ZPL، مدل چاپگر و font capability باید در pilot تایید شود.

### Cash Drawer

پالس ESC/POS از receipt printer خام ارسال می‌شود. اگر `auto_open` روشن باشد، بعد از چاپ موفق فروش نقدی drawer pulse ارسال می‌شود.

### A4

Endpoint محلی PDF را به spooler سیستم می‌دهد:

- Windows: برنامه PDF پیش‌فرض با Print verb.
- Linux/macOS-like environments: `lp` و در صورت مشخص‌شدن printer name از `-d`.

### POS

Providerهای فاز 15.8:

- `manual`: هیچ ادعای اتصال خودکار ندارد؛ اپراتور مبلغ را روی terminal وارد می‌کند.
- `mock`: فقط وقتی `AUTOPARTS_EDGE_ALLOW_MOCK_POS=true` باشد؛ برای QA.
- `tcp_json`: bridge عمومی به daemon/SDK شرکت PSP که JSON روی TCP می‌پذیرد.

فرمت درخواست generic adapter:

```json
{"amount": 1250000, "reference": "sale-ref", "currency": "IRT"}
```

پاسخ مورد انتظار:

```json
{"approved": true, "rrn": "...", "trace": "...", "message": "..."}
```

این به معنی پشتیبانی خودکار همه کارتخوان‌های ایران نیست. برای هر PSP/مدل واقعی، adapter/SDK همان vendor باید پیاده و در pilot تایید شود.

## صفحه تنظیمات

```text
/store/hardware
```

تنظیمات فقط روی همان Store Edge محلی ذخیره می‌شوند (`hardware.json`) و به Cloud منتقل نمی‌شوند.

## Smoke test بدون سخت‌افزار واقعی

transport نوع `file` فقط برای QA است و در حالت عادی غیرفعال است. Agent تست را با `AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT=true` اجرا کن، سپس برای فیش‌پرینتر transport را `file` بگذار و Address را یک مسیر writable مثل زیر بده:

```text
C:\Users\<user>\AppData\Local\AutoParts\receipt-test.txt
```

یا روی Linux:

```text
/tmp/autoparts-receipt.txt
```

سپس «چاپ تست» را بزن و فایل را بررسی کن.

برای POS QA:

```powershell
$env:AUTOPARTS_EDGE_ALLOW_MOCK_POS="true"
AutoPartsStoreEdge.exe
```

و provider را `mock` انتخاب کن.

## Build/Install ویندوز

```powershell
powershell -ExecutionPolicy Bypass -File edge/windows/build-installer.ps1 -Version 0.15.8
# خروجی: dist/AutoParts-Store-Agent-Setup-0.15.8.exe
```

Installer تولیدی Phase 15.7.1 حفظ شده و حالا executable نسخه 0.15.8 شامل Offline + Hardware Bridge هر دو است. Pairing origin hotfix فاز 15.7.1.1 نیز داخل این baseline ادغام شده است.
