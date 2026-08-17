# Release Notes — Phase 15.7

- Store Edge Agent با Go و بدون dependency خارجی جدید.
- Durable local offline sale queue با atomic file persistence.
- local offline POS مستقل از Cloud UI برای زمان قطع کامل اینترنت.
- device pairing یک‌بارمصرف، secret hash، revoke و last-seen.
- tenant/store/warehouse scoped catalog snapshot.
- device-authenticated idempotent offline-sale replay.
- فروش آفلاین فقط cash/card؛ credit/mixed عمداً رد می‌شود.
- conflict queue به‌جای silent data loss.
- `/store/edge` برای Pairing، device management و وضعیت Agent.
- fallback صفحه فروش: روی network/5xx و فقط در صورت Edge آماده، cash/card روی Agent ذخیره می‌شود.
- Windows user-level build/install/uninstall scripts و Desktop Offline POS shortcut.
- migration `014_store_edge_offline.sql` و RC invariants جدید.
