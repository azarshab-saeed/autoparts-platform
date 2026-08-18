# AutoParts Store Agent - Windows release

Customer distribution is `AutoParts-Store-Agent-Setup-<version>.exe`. Customers do not run Go, Make, Docker, PowerShell, or Windows Services manually.

## Runtime layout since 15.8.1

- Manager: `C:\Program Files\AutoParts\StoreAgent\AutoPartsStoreEdgeManager.exe`
- Worker: `C:\Program Files\AutoParts\StoreAgent\AutoPartsStoreEdge.exe`
- Updater: `C:\Program Files\AutoParts\StoreAgent\AutoPartsStoreEdgeUpdater.exe`
- Windows Service: `AutoPartsStoreEdgeManager`
- Manager API: `http://127.0.0.1:17623/`
- Worker/local POS: `http://127.0.0.1:17624/`
- Offline data: `C:\ProgramData\AutoParts\StoreEdge\data`
- Logs: `C:\ProgramData\AutoParts\StoreEdge\logs`

The service is delayed-auto-start. Manager supervises the Worker, so Start/Stop/Restart from `/store/edge` never requires the user to open Services.msc. Upgrade removes the legacy `AutoPartsStoreEdge` direct service while preserving ProgramData.

## Release build

On Windows x64 with Go and Inno Setup 6:

```powershell
$env:AUTOPARTS_EDGE_UPDATE_MANIFEST_URL="https://downloads.example.com/store-agent/windows-amd64.json"
$env:AUTOPARTS_EDGE_UPDATE_PUBLIC_KEY="BASE64_ED25519_PUBLIC_KEY"
powershell -ExecutionPolicy Bypass -File edge/windows/build-installer.ps1 -Version 0.15.8.1
```

For Authenticode signing, set `AUTOPARTS_SIGN_CERT_SHA1` as before. Ed25519 update signing is separate: only the public key is embedded in the Manager; the private release key stays outside the installer/build output.

The same Inno Setup AppId is retained, so a newer Setup upgrades the existing installation and preserves local offline data.
