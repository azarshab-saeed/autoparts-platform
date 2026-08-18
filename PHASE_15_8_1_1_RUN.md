# Phase 15.8.1.1 — One-click Store Agent Bootstrap

این فاز نصب اولیه Store Agent را از workflow دستی توسعه جدا می‌کند. کاربر از صفحه `/store/edge` سیستم‌عامل و معماری تشخیص‌داده‌شده را می‌بیند و Installer رسمی همان platform را دانلود می‌کند. بعد از تأیید Installer توسط سیستم‌عامل، Manager خودکار Start می‌شود و همان صفحه حضور آن را poll می‌کند.

## تجربه کاربر

### Windows x64

1. `/store/edge` باز می‌شود.
2. `Windows x64` تشخیص داده می‌شود.
3. کاربر روی `دانلود Installer` می‌زند.
4. `AutoParts-Store-Agent-Setup-windows-x64.exe` دانلود می‌شود.
5. کاربر UAC/Installer را یک بار تأیید می‌کند.
6. `AutoPartsStoreEdgeManager` به‌صورت Windows Service با auto-start اجرا می‌شود.
7. صفحه Manager را خودکار پیدا می‌کند.
8. از اینجا Start/Stop/Restart/Update فقط از UI انجام می‌شود.

### Linux Debian/Ubuntu

برای amd64 و arm64 فایل DEB رسمی تولید می‌شود:

```text
autoparts-store-agent-linux-amd64.deb
autoparts-store-agent-linux-arm64.deb
```

کاربر فایل را با Software Center باز می‌کند و Install را تأیید می‌کند. package یک global user unit نصب می‌کند و برای sessionهای فعال Manager را خودکار enable/start می‌کند. در loginهای بعدی نیز user service خودکار اجرا می‌شود.

Linux package فایل‌های package-owned را زیر `/usr/lib/autoparts-store-edge` نگه می‌دارد، اما runtime قابل بروزرسانی را در home کاربر زیر `~/.local/share/autoparts-store-edge/bin` اجرا می‌کند. بنابراین signed self-update به root دسترسی نیاز ندارد.

## Release assets

Workflow `.github/workflows/store-agent-installer.yml` روی tagهای زیر اجرا می‌شود:

```text
store-agent-v0.15.8.2
```

و این stable aliases را روی GitHub Release منتشر می‌کند:

```text
AutoParts-Store-Agent-Setup-windows-x64.exe
autoparts-store-agent-linux-amd64.deb
autoparts-store-agent-linux-arm64.deb
```

Frontend به‌صورت پیش‌فرض `latest/download` همین assetها را استفاده می‌کند. در محیط‌هایی که mirror/CDN دارند می‌توان URLها را override کرد:

```text
NEXT_PUBLIC_EDGE_INSTALLER_WINDOWS_AMD64_URL
NEXT_PUBLIC_EDGE_INSTALLER_LINUX_AMD64_URL
NEXT_PUBLIC_EDGE_INSTALLER_LINUX_ARM64_URL
```

متغیر قدیمی `NEXT_PUBLIC_EDGE_INSTALLER_URL` برای backward compatibility به‌عنوان override عمومی باقی مانده است.

## Production browser origin

Manager قبل از Pair شدن هم باید origin اصلی Web App را بشناسد تا صفحه بتواند نصب موفق را تشخیص دهد. Release builder باید GitHub Actions variable زیر را تنظیم کند:

```text
STORE_AGENT_ALLOWED_ORIGINS=https://store.example.com
```

چند origin با comma جدا می‌شوند. این مقدار در build رسمی Manager embed می‌شود. originهای Pair شده نیز مانند قبل از config محلی خوانده می‌شوند.

## QA

کد:

```bash
go test ./cmd/store-edge-manager ./internal/storeedgemanager ./cmd/store-edge ./internal/storeedge
sh -n edge/linux/build-deb.sh edge/linux/package/*
VERSION=0.15.8.2 ARCH=amd64 ./edge/linux/build-deb.sh
dpkg-deb --info dist/autoparts-store-agent_0.15.8.2_amd64.deb
```

Frontend acceptance:

```text
Manager absent
  -> OS/arch detected
  -> download installer
  -> waiting state
  -> install OS package
  -> Manager detected automatically
  -> Start/Stop/Restart available
  -> Pair
  -> signed Update available from same page
```

در توسعه سورس، `./edge/linux/install-user-service.sh` همچنان برای developer shortcut وجود دارد؛ مشتری نهایی نباید از آن استفاده کند.
