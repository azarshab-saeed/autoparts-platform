# UI State Matrix

هر Screen باید حداقل این stateها را طراحی و تست کند.

| State | رفتار استاندارد |
|---|---|
| Loading | Skeleton نزدیک به layout واقعی؛ spinner تمام‌صفحه فقط برای bootstrap |
| Empty | توضیح کوتاه + CTA مرتبط |
| Error retryable | پیام قابل فهم + Retry |
| Validation | خطا کنار field و summary در صورت چند خطا |
| Forbidden | توضیح سطح دسترسی بدون نمایش کنترل حساس |
| Offline | banner واضح؛ عملیات مالی بدون queue مبهم انجام نشود |
| Stale network inventory | زمان بروزرسانی + کاهش confidence |
| Partial data | بخش‌های سالم نمایش داده شوند؛ failure یک widget کل صفحه را نخواباند |

## Financial actions
برای create sale/purchase/payment:
- دکمه submit بعد از کلیک disabled شود.
- Idempotency-Key قبل از request ساخته شود و retry همان key را استفاده کند.
- timeout به معنی «ناموفق بودن قطعی» تلقی نشود؛ UI باید وضعیت را با lookup/retry امن resolve کند.

## Inventory actions
- quantity conflict باید مقدار تازه را نمایش دهد.
- اگر موجودی در فاصله انتخاب کالا تا submit تغییر کرد، backend conflict/insufficient stock برگرداند و UI cart را refresh کند.
