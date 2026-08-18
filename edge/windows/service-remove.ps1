param(
  [string]$ServiceName = "AutoPartsStoreEdgeManager",
  [string]$LegacyServiceName = "AutoPartsStoreEdge",
  [switch]$RemoveData
)
$ErrorActionPreference = "Stop"
$id = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($id)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw "Administrator privileges are required to remove AutoParts Store Agent." }
foreach ($name in @($ServiceName,$LegacyServiceName)) {
  $service = Get-Service -Name $name -ErrorAction SilentlyContinue
  if ($service) {
    if ($service.Status -ne "Stopped") { Stop-Service -Name $name -Force -ErrorAction SilentlyContinue; try { $service.WaitForStatus("Stopped",[TimeSpan]::FromSeconds(15)) } catch {} }
    & "$env:SystemRoot\System32\sc.exe" delete $name | Out-Host
  }
}
if ($RemoveData) { Remove-Item (Join-Path $env:ProgramData "AutoParts\StoreEdge") -Recurse -Force -ErrorAction SilentlyContinue; Write-Host "Store Agent Manager, worker and local data removed." }
else { Write-Host "Store Agent services removed. Offline data was preserved under $env:ProgramData\AutoParts\StoreEdge." }
