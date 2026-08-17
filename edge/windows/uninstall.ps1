param([switch]$RemoveData)
$ErrorActionPreference = "Stop"
$installDir = Join-Path $env:LOCALAPPDATA "AutoParts\StoreEdge"
Get-Process AutoPartsStoreEdge -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path ([Environment]::GetFolderPath("Startup")) "AutoParts Store Edge.lnk") -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path ([Environment]::GetFolderPath("Desktop")) "AutoParts Offline POS.url") -Force -ErrorAction SilentlyContinue
Remove-Item $installDir -Recurse -Force -ErrorAction SilentlyContinue
if ($RemoveData) {
  Remove-Item (Join-Path $env:USERPROFILE ".autoparts-store-edge") -Recurse -Force -ErrorAction SilentlyContinue
  Write-Host "Store Edge and local data removed."
} else {
  Write-Host "Store Edge removed. Local offline data was preserved at $env:USERPROFILE\.autoparts-store-edge"
}
