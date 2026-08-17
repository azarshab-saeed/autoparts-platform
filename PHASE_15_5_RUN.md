# Phase 15.5 — UI/UX Polish

این فاز فقط تجربه کاربری و ظاهر را ارتقا می‌دهد و قرارداد API یا منطق کسب‌وکار را تغییر نمی‌دهد.

## اعمال

```bash
unzip AutoParts_Phase15_5_UI_Polish_EasyApply.zip
cd AutoParts_Phase15_5_UI_Polish_EasyApply
./apply.sh --repo ../autoparts-core/
```

سپس:

```bash
cd ../autoparts-core
docker compose up -d --build web
```

برای تست کامل RC بعد از تأیید ظاهر:

```bash
make rc-static-check
make rc-smoke
make rc-db-check
make rc-load
```

## Visual QA

رزولوشن‌های پیشنهادی:

- Desktop: 1440x900
- Laptop: 1280x800
- Tablet: 820x1180
- Mobile: 390x844

صفحه‌های اصلی:

- `/store`
- `/store/sales`
- `/store/purchases`
- `/store/inventory`
- `/store/procurement`
- `/store/orders`
- `/mechanic`
- `/mechanic/orders`

در موبایل پنل فروشگاه، sidebar با bottom navigation پنج‌گزینه‌ای جایگزین می‌شود.
