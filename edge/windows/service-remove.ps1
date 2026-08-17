param(
  [string]$ServiceName = "AutoPartsStoreEdge",
  [switch]$RemoveData
)
$ErrorActionPreference = "Stop"

$id = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($id)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw "Administrator privileges are required to remove AutoParts Store Agent."
}

$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($service) {
  if ($service.Status -ne "Stopped") {
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    try { $service.WaitForStatus("Stopped", [TimeSpan]::FromSeconds(15)) } catch {}
  }
  & "$env:SystemRoot\System32\sc.exe" delete $ServiceName | Out-Host
}
if ($RemoveData) {
  Remove-Item (Join-Path $env:ProgramData "AutoParts\StoreEdge") -Recurse -Force -ErrorAction SilentlyContinue
  Write-Host "Store Agent service and local offline data removed."
} else {
  Write-Host "Store Agent service removed. Offline data was preserved under $env:ProgramData\AutoParts\StoreEdge."
}
