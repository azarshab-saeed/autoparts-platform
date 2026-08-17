# Phase 15.6 Acceptance

- [ ] Dashboard به‌وضوح نشان دهد شبکه چه مقدار فروش/درخواست ایجاد کرده است.
- [ ] اگر network خاموش یا بدون offer باشد، Dashboard CTA مناسب برای فعال‌سازی نشان دهد.
- [ ] Action Center شمارنده‌های رزرو، procurement و low stock را درست نشان دهد.
- [ ] `/store/import` فایل CSV UTF-8 را parse و preview کند.
- [ ] duplicate SKU/barcode و opening stock بدون cost قبل از commit رد شود.
- [ ] import با Idempotency-Key replay امن داشته باشد.
- [ ] import کالا/offer را update کند اما inventory دارای movement/reservation را overwrite نکند.
- [ ] journal مربوط به opening inventory debit/credit متوازن باشد.
- [ ] Saved Vehicle بعد از reload باقی بماند.
- [ ] Recent searches بعد از reload باقی بماند.
- [ ] reservation و procurement قبلی regression نداشته باشند.
- [ ] RC DB invariants سبز بمانند.
