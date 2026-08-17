param(
  [Parameter(Mandatory=$true)][string]$Executable,
  [string]$ServiceName = "AutoPartsStoreEdge"
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

Assert-Administrator
$Executable = [IO.Path]::GetFullPath($Executable)
if (-not (Test-Path $Executable)) { throw "Store Agent executable not found: $Executable" }

$dataRoot = Join-Path $env:ProgramData "AutoParts\StoreEdge"
New-Item -ItemType Directory -Force -Path (Join-Path $dataRoot "data") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $dataRoot "logs") | Out-Null

$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($service) {
  if ($service.Status -ne "Stopped") {
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    $service.WaitForStatus("Stopped", [TimeSpan]::FromSeconds(15))
  }
  Invoke-Sc @("config", $ServiceName, "binPath=", ('"' + $Executable + '" service'), "start=", "delayed-auto", "DisplayName=", "AutoParts Store Agent")
} else {
  Invoke-Sc @("create", $ServiceName, "binPath=", ('"' + $Executable + '" service'), "start=", "delayed-auto", "DisplayName=", "AutoParts Store Agent")
}
Invoke-Sc @("description", $ServiceName, "AutoParts local offline sales and store hardware agent")
Invoke-Sc @("failure", $ServiceName, "reset=", "86400", "actions=", "restart/5000/restart/15000/restart/60000")
Invoke-Sc @("failureflag", $ServiceName, "1") -AllowFailure

Start-Service -Name $ServiceName
(Get-Service -Name $ServiceName).WaitForStatus("Running", [TimeSpan]::FromSeconds(15))
Start-Sleep -Milliseconds 800
try {
  $health = Invoke-RestMethod -Uri "http://127.0.0.1:17624/healthz" -TimeoutSec 5
  Write-Host "AutoParts Store Agent service is running. Version=$($health.version) Mode=$($health.mode)"
} catch {
  throw "Windows Service started, but local health check failed: $($_.Exception.Message)"
}
Write-Host "Data: $(Join-Path $dataRoot 'data')"
Write-Host "Logs: $(Join-Path $dataRoot 'logs\agent.log')"
