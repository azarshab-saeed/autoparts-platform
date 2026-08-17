param(
  [string]$SourceExe = (Join-Path $PSScriptRoot "..\..\dist\AutoPartsStoreEdge.exe")
)
$ErrorActionPreference = "Stop"

$id = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($id)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw "Run PowerShell as Administrator, or use AutoParts-Store-Agent-Setup.exe."
}
if (-not (Test-Path $SourceExe)) { throw "AutoPartsStoreEdge.exe not found. Run edge/windows/build.ps1 first." }

$installDir = Join-Path $env:ProgramFiles "AutoParts\StoreAgent"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
$target = Join-Path $installDir "AutoPartsStoreEdge.exe"

$existing = Get-Service -Name "AutoPartsStoreEdge" -ErrorAction SilentlyContinue
if ($existing -and $existing.Status -ne "Stopped") {
  Stop-Service -Name "AutoPartsStoreEdge" -Force -ErrorAction SilentlyContinue
  try { $existing.WaitForStatus("Stopped", [TimeSpan]::FromSeconds(15)) } catch {}
}
Copy-Item $SourceExe $target -Force
Copy-Item (Join-Path $PSScriptRoot "service-install.ps1") (Join-Path $installDir "service-install.ps1") -Force
Copy-Item (Join-Path $PSScriptRoot "service-remove.ps1") (Join-Path $installDir "service-remove.ps1") -Force
Copy-Item (Join-Path $PSScriptRoot "uninstall.ps1") (Join-Path $installDir "uninstall.ps1") -Force

& (Join-Path $installDir "service-install.ps1") -Executable $target

$ws = New-Object -ComObject WScript.Shell
$desktop = [Environment]::GetFolderPath("CommonDesktopDirectory")
$shortcut = $ws.CreateShortcut((Join-Path $desktop "AutoParts Offline POS.lnk"))
$shortcut.TargetPath = $target
$shortcut.Arguments = "open"
$shortcut.WorkingDirectory = $installDir
$shortcut.Description = "Open AutoParts offline sales"
$shortcut.Save()

$uninstallKey = "HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\AutoPartsStoreAgent"
New-Item -Path $uninstallKey -Force | Out-Null
$installedVersion = (& $target version).Trim()
Set-ItemProperty $uninstallKey DisplayName "AutoParts Store Agent"
Set-ItemProperty $uninstallKey DisplayVersion $installedVersion
Set-ItemProperty $uninstallKey Publisher "AutoParts"
Set-ItemProperty $uninstallKey InstallLocation $installDir
Set-ItemProperty $uninstallKey UninstallString ('powershell.exe -NoProfile -ExecutionPolicy Bypass -File "' + (Join-Path $installDir "uninstall.ps1") + '"')
Write-Host "AutoParts Store Agent $installedVersion installed as Windows Service."
