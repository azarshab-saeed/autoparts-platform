# Phase 15.7.1 — Production Store Agent Installer

این Hotfix/Hardening قبل از Phase 15.8، Store Edge را از ابزار توسعه‌ای به Agent قابل نصب برای فروشگاه تبدیل می‌کند.

## Windows customer experience

کاربر نهایی فقط فایل زیر را اجرا می‌کند:

```text
AutoParts-Store-Agent-Setup-0.15.7.1.exe
```

Setup با دسترسی Administrator یک‌بار اجرا می‌شود و سپس:

- Agent را در `C:\Program Files\AutoParts\StoreAgent` نصب می‌کند.
- Windows Service واقعی `AutoPartsStoreEdge` می‌سازد.
- Service را `Delayed Auto Start` می‌کند.
- در failureهای متوالی Service را خودکار restart می‌کند.
- داده آفلاین را در `C:\ProgramData\AutoParts\StoreEdge\data` نگه می‌دارد.
- log را در `C:\ProgramData\AutoParts\StoreEdge\logs\agent.log` می‌نویسد.
- shortcut «AutoParts Offline POS» می‌سازد.
- نصب نسخه جدید را به‌صورت upgrade روی همان AppId انجام می‌دهد و ProgramData را حفظ می‌کند.

کاربر هیچ `go run`، `make`، Docker یا PowerShell دستی اجرا نمی‌کند.

## ساخت Setup برای Release

روی Windows x64 با Go و Inno Setup 6:

```powershell
powershell -ExecutionPolicy Bypass -File edge/windows/build-installer.ps1 -Version 0.15.7.1
```

خروجی:

```text
dist/AutoParts-Store-Agent-Setup-0.15.7.1.exe
dist/AutoParts-Store-Agent-Setup-0.15.7.1.exe.sha256
```

برای Release مشتری باید Setup code-sign شود. build script در صورت تعریف `AUTOPARTS_SIGN_CERT_SHA1` از `signtool.exe` استفاده می‌کند.

## Build خودکار در GitHub Actions

Workflow `Store Agent Windows Installer` به‌صورت manual dispatch یا tag با الگوی `store-agent-v*` Setup ویندوز را روی runner ویندوز می‌سازد و به‌عنوان artifact تحویل می‌دهد. Secrets اختیاری `WINDOWS_CODE_SIGN_PFX_BASE64` و `WINDOWS_CODE_SIGN_PFX_PASSWORD` برای Authenticode signing پشتیبانی می‌شوند.

## Upgrade behavior

Setup قبل از جایگزینی binary، Service قبلی را stop می‌کند؛ سپس همان Service را با binary جدید config/start می‌کند. اطلاعات Pairing، snapshot، صف فروش آفلاین و conflictها در ProgramData باقی می‌مانند.

## Uninstall

Uninstall معمولی Service و binary را حذف می‌کند ولی data آفلاین را برای جلوگیری از data loss نگه می‌دارد. پاک‌کردن data باید آگاهانه و جداگانه انجام شود.

## Linux development/pilot

برای اینکه روی Linux هم `make edge-run` دائمی لازم نباشد:

```bash
./edge/linux/install-user-service.sh
```

و حذف:

```bash
./edge/linux/uninstall-user-service.sh
```

## Validation

```bash
make edge-test
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -o /tmp/AutoPartsStoreEdge.exe ./cmd/store-edge
```
