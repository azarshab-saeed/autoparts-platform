param(
  [Parameter(Mandatory=$true)][string]$ManagerExecutable,
  [string]$ServiceName = "AutoPartsStoreEdgeManager",
  [string]$LegacyServiceName = "AutoPartsStoreEdge"
)
$ErrorActionPreference = "Stop"

function Assert-Administrator {
  $id = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = New-Object Security.Principal.WindowsPrincipal($id)
  if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Administrator privileges are required to install AutoParts Store Agent."
  }
}
function Invoke-Sc([string[]]$Args, [switch]$AllowFailure) {
  & "$env:SystemRoot\System32\sc.exe" @Args | Out-Host
  if ($LASTEXITCODE -ne 0 -and -not $AllowFailure) { throw "sc.exe $($Args -join ' ') failed with exit code $LASTEXITCODE" }
}
function Remove-LegacyService([string]$Name) {
  $legacy = Get-Service -Name $Name -ErrorAction SilentlyContinue
  if ($legacy) {
    if ($legacy.Status -ne "Stopped") { Stop-Service -Name $Name -Force -ErrorAction SilentlyContinue; try { $legacy.WaitForStatus("Stopped",[TimeSpan]::FromSeconds(12)) } catch {} }
    Invoke-Sc @("delete",$Name) -AllowFailure
  }
}

Assert-Administrator
$ManagerExecutable = [IO.Path]::GetFullPath($ManagerExecutable)
if (-not (Test-Path $ManagerExecutable)) { throw "Store Agent Manager executable not found: $ManagerExecutable" }
$installDir = Split-Path $ManagerExecutable -Parent
foreach ($required in @("AutoPartsStoreEdge.exe","AutoPartsStoreEdgeUpdater.exe")) {
  if (-not (Test-Path (Join-Path $installDir $required))) { throw "Required Store Agent file is missing: $required" }
}

$dataRoot = Join-Path $env:ProgramData "AutoParts\StoreEdge"
New-Item -ItemType Directory -Force -Path (Join-Path $dataRoot "data") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $dataRoot "logs") | Out-Null
Remove-LegacyService $LegacyServiceName

$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($service) {
  if ($service.Status -ne "Stopped") { Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue; try { $service.WaitForStatus("Stopped",[TimeSpan]::FromSeconds(15)) } catch {} }
  Invoke-Sc @("config", $ServiceName, "binPath=", ('"' + $ManagerExecutable + '" service'), "start=", "delayed-auto", "DisplayName=", "AutoParts Store Agent Manager")
} else {
  Invoke-Sc @("create", $ServiceName, "binPath=", ('"' + $ManagerExecutable + '" service'), "start=", "delayed-auto", "DisplayName=", "AutoParts Store Agent Manager")
}
Invoke-Sc @("description", $ServiceName, "AutoParts local lifecycle manager for offline sales and store hardware agent")
Invoke-Sc @("failure", $ServiceName, "reset=", "86400", "actions=", "restart/5000/restart/15000/restart/60000")
Invoke-Sc @("failureflag", $ServiceName, "1") -AllowFailure

Start-Service -Name $ServiceName
(Get-Service -Name $ServiceName).WaitForStatus("Running", [TimeSpan]::FromSeconds(15))
Start-Sleep -Milliseconds 1200
try {
  $managerHealth = Invoke-RestMethod -Uri "http://127.0.0.1:17623/healthz" -TimeoutSec 5
  Write-Host "Store Agent Manager is running. Version=$($managerHealth.version)"
  $workerHealth = Invoke-RestMethod -Uri "http://127.0.0.1:17624/healthz" -TimeoutSec 8
  Write-Host "Store Agent worker is running. Version=$($workerHealth.version)"
} catch {
  throw "Manager service started, but local lifecycle health check failed: $($_.Exception.Message)"
}
Write-Host "Data: $(Join-Path $dataRoot 'data')"
Write-Host "Logs: $(Join-Path $dataRoot 'logs')"
