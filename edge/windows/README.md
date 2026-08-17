# AutoParts Store Agent - Windows release

The customer-facing distribution is `AutoParts-Store-Agent-Setup-<version>.exe`.
Customers do not run `go`, `make`, Docker, or PowerShell manually.

## Release build

On a Windows x64 release machine with Go and Inno Setup 6:

```powershell
powershell -ExecutionPolicy Bypass -File edge/windows/build-installer.ps1 -Version 0.15.8
```

For a signed customer release, install the signing certificate in the Windows certificate store and set its SHA-1 thumbprint:

```powershell
$env:AUTOPARTS_SIGN_CERT_SHA1="CERTIFICATE_THUMBPRINT"
powershell -ExecutionPolicy Bypass -File edge/windows/build-installer.ps1 -Version 0.15.8
```

The script writes the Setup executable and a `.sha256` file to `dist/`.

## Installed layout

- Binary: `C:\Program Files\AutoParts\StoreAgent\AutoPartsStoreEdge.exe`
- Windows Service: `AutoPartsStoreEdge` / `AutoParts Store Agent`
- Offline data: `C:\ProgramData\AutoParts\StoreEdge\data`
- Agent log: `C:\ProgramData\AutoParts\StoreEdge\logs\agent.log`
- Local UI: `http://127.0.0.1:17624/`

The service is delayed-auto-start and configured to restart after failures. The same Inno Setup AppId is kept across releases, so installing a newer Setup upgrades the existing installation while preserving ProgramData.

## Developer fallback

`install.ps1` and `uninstall.ps1` are retained for development/support. They also use the native Windows Service, not Startup-folder execution.
