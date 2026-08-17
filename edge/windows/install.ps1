param(
  [string]$SourceExe = (Join-Path $PSScriptRoot "..\..\dist\AutoPartsStoreEdge.exe")
)
$ErrorActionPreference = "Stop"
if (-not (Test-Path $SourceExe)) { throw "AutoPartsStoreEdge.exe not found. Run edge/windows/build.ps1 first." }
$installDir = Join-Path $env:LOCALAPPDATA "AutoParts\StoreEdge"
$target = Join-Path $installDir "AutoPartsStoreEdge.exe"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Get-Process AutoPartsStoreEdge -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Copy-Item $SourceExe $target -Force

$ws = New-Object -ComObject WScript.Shell
$startup = [Environment]::GetFolderPath("Startup")
$shortcut = $ws.CreateShortcut((Join-Path $startup "AutoParts Store Edge.lnk"))
$shortcut.TargetPath = $target
$shortcut.WorkingDirectory = $installDir
$shortcut.WindowStyle = 7
$shortcut.Description = "AutoParts Store Edge background agent"
$shortcut.Save()

$desktop = [Environment]::GetFolderPath("Desktop")
$url = Join-Path $desktop "AutoParts Offline POS.url"
@"
[InternetShortcut]
URL=http://127.0.0.1:17624/
IconFile=$target
IconIndex=0
"@ | Set-Content -Encoding ASCII $url

Start-Process -FilePath $target -WorkingDirectory $installDir -WindowStyle Hidden
Start-Sleep -Milliseconds 900
try {
  $health = Invoke-RestMethod -Uri "http://127.0.0.1:17624/healthz" -TimeoutSec 3
  Write-Host "Store Edge installed and running: $($health.version)"
} catch {
  Write-Warning "Agent was installed but health check did not answer yet. Open http://127.0.0.1:17624/ manually."
}
Write-Host "Install dir: $installDir"
Write-Host "Data is kept under $env:USERPROFILE\.autoparts-store-edge"
