param([switch]$RemoveData)
$ErrorActionPreference = "Stop"

$id = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($id)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw "Run PowerShell as Administrator to uninstall AutoParts Store Agent."
}
$installDir = Join-Path $env:ProgramFiles "AutoParts\StoreAgent"
$removeScript = Join-Path $installDir "service-remove.ps1"
if (Test-Path $removeScript) { & $removeScript -RemoveData:$RemoveData }
Remove-Item (Join-Path ([Environment]::GetFolderPath("CommonDesktopDirectory")) "AutoParts Offline POS.lnk") -Force -ErrorAction SilentlyContinue
Remove-Item "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\AutoPartsStoreAgent" -Recurse -Force -ErrorAction SilentlyContinue

# A running PowerShell process cannot safely remove its own script directory immediately.
$cleanup = 'Start-Sleep -Seconds 2; Remove-Item -LiteralPath "' + $installDir.Replace('"','`"') + '" -Recurse -Force -ErrorAction SilentlyContinue'
Start-Process powershell.exe -WindowStyle Hidden -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", $cleanup)
Write-Host $(if ($RemoveData) { "Store Agent uninstalled and data removal scheduled." } else { "Store Agent uninstalled. Offline data is preserved under ProgramData." })
