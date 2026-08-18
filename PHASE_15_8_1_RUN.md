# Phase 15.8.1 — Store Agent Lifecycle Manager

این فاز وابستگی کاربر به `systemctl`, Windows Services، Terminal و اجرای دستی `make edge-run` را حذف می‌کند.

## معماری

دو process محلی داریم:

```text
Browser / Store UI
       |
       | http://127.0.0.1:17623
       v
Store Agent Manager   <-- همیشه سبک و فعال
       |
       | child process
       v
Store Edge Worker     <-- offline sales + hardware bridge
       |
       +-- http://127.0.0.1:17624
```

Manager کنترل‌پلین پایدار است. وقتی کاربر از UI دکمه **Stop Agent** را می‌زند فقط Worker متوقف می‌شود؛ Manager روشن می‌ماند تا Start/Restart/Update همچنان از همان UI ممکن باشد.

## Linux development / pilot

بعد از apply فقط یک بار installer سرویس را اجرا کن:

```bash
./edge/linux/install-user-service.sh
```

این script سرویس قدیمی `autoparts-store-edge.service` را حذف و سرویس جدید زیر را فعال می‌کند:

```text
autoparts-store-edge-manager.service
```

از آن لحظه به بعد Start/Stop/Restart از `/store/edge` انجام می‌شود و کاربر نباید `systemctl stop/start` بزند.

Status فنی در صورت نیاز پشتیبانی:

```bash
systemctl --user status autoparts-store-edge-manager.service
curl http://127.0.0.1:17623/healthz
curl http://127.0.0.1:17624/healthz
```

اگر داده pairing قبلی در `~/.autoparts-store-edge` باشد و data directory جدید خالی باشد، installer آن را migrate می‌کند.

## Windows customer install

Installer جدید سه binary نصب می‌کند:

```text
AutoPartsStoreEdgeManager.exe
AutoPartsStoreEdge.exe
AutoPartsStoreEdgeUpdater.exe
```

Windows Service فقط Manager است:

```text
AutoPartsStoreEdgeManager
```

Manager با delayed auto-start بالا می‌آید و Worker را خودش اجرا/monitor می‌کند. سرویس legacy با نام `AutoPartsStoreEdge` هنگام upgrade حذف می‌شود.

## Frontend lifecycle

صفحه:

```text
/store/edge
```

امکانات:

- مشاهده Windows/Linux و نسخه Manager/Agent
- Start Agent
- Stop Agent
- Restart Agent
- Check update
- Apply signed update
- نمایش crash/last-exit state
- هشدار قبل از Stop در صورت وجود فروش در صف Sync

## Signed update

Auto-update فقط وقتی فعال است که release build هر دو مقدار را داشته باشد:

```text
AUTOPARTS_EDGE_UPDATE_MANIFEST_URL
AUTOPARTS_EDGE_UPDATE_PUBLIC_KEY
```

Public key از نوع Ed25519 و Base64 است. Private key **هرگز** داخل Agent/Installer قرار نمی‌گیرد.

هر asset علاوه بر SHA-256 یک signature دارد که این message را امضا می‌کند:

```text
<version>\n<platform>\n<component>\n<sha256>
```

بنابراین تغییر URL/manifest نمی‌تواند یک binary بدون امضای release key را نصب کند و signature به version/platform/component هم bind شده است.

ساخت manifest امضاشده:

```bash
export AUTOPARTS_EDGE_UPDATE_PRIVATE_KEY='<base64-ed25519-private-key>'

go run ./cmd/store-edge-manifest \
  --version 0.15.9 \
  --platform linux-amd64 \
  --worker dist/autoparts-store-edge \
  --worker-url https://downloads.example.com/0.15.9/autoparts-store-edge \
  --manager dist/autoparts-store-edge-manager \
  --manager-url https://downloads.example.com/0.15.9/autoparts-store-edge-manager \
  --notes 'Store Agent 0.15.9' \
  --out dist/update-manifest.json
```

برای Windows platform از `windows-amd64` استفاده شود.

### Worker-only update

اگر manifest فقط Worker داشته باشد، Manager بدون restart خودش Worker را graceful stop می‌کند، binary جدید را جایگزین می‌کند و Worker را دوباره Start می‌کند.

### Manager + Worker update

اگر Manager هم در manifest باشد، updater helper وارد عمل می‌شود:

- Windows: Manager service را stop می‌کند، binaryها را جایگزین می‌کند و service را start می‌کند.
- Linux systemd-user: updater در transient systemd unit جدا اجرا می‌شود، Manager را stop می‌کند، binaryها را replace و سرویس را دوباره start می‌کند.

## CORS / local security

Manager فقط loopback است. Browser origin مجاز شامل localhost development و origin ذخیره‌شده در pairing config است. Mutating lifecycle endpoints علاوه بر Origin guard به header محلی `X-AutoParts-Edge: 1` نیاز دارند. Update binary بدون SHA-256 + Ed25519 معتبر نصب نمی‌شود.

## QA

```bash
make edge-manager-test
make edge-manager-run
```

در `/store/edge` این سناریو را تست کن:

```text
Running -> Stop Agent -> Stopped -> Start Agent -> Running -> Restart -> Running
```

Pairing و offline queue در Stop/Restart حذف نمی‌شوند.
