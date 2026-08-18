param(
  [string]$SourceDir = (Join-Path $PSScriptRoot "..\..\dist")
)
$ErrorActionPreference = "Stop"
$id = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($id)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw "Run PowerShell as Administrator, or use AutoParts-Store-Agent-Setup.exe." }
$SourceDir = [IO.Path]::GetFullPath($SourceDir)
$files = @("AutoPartsStoreEdge.exe","AutoPartsStoreEdgeManager.exe","AutoPartsStoreEdgeUpdater.exe")
foreach ($f in $files) { if (-not (Test-Path (Join-Path $SourceDir $f))) { throw "$f not found. Run edge/windows/build.ps1 first." } }
$installDir = Join-Path $env:ProgramFiles "AutoParts\StoreAgent"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
foreach ($svcName in @("AutoPartsStoreEdgeManager","AutoPartsStoreEdge")) {
  $svc=Get-Service -Name $svcName -ErrorAction SilentlyContinue
  if($svc -and $svc.Status -ne "Stopped"){Stop-Service -Name $svcName -Force -ErrorAction SilentlyContinue;try{$svc.WaitForStatus("Stopped",[TimeSpan]::FromSeconds(15))}catch{}}
}
foreach ($f in $files) { Copy-Item (Join-Path $SourceDir $f) (Join-Path $installDir $f) -Force }
foreach ($f in @("service-install.ps1","service-remove.ps1","uninstall.ps1")) { Copy-Item (Join-Path $PSScriptRoot $f) (Join-Path $installDir $f) -Force }
$manager = Join-Path $installDir "AutoPartsStoreEdgeManager.exe"
$worker = Join-Path $installDir "AutoPartsStoreEdge.exe"
& (Join-Path $installDir "service-install.ps1") -ManagerExecutable $manager
$ws = New-Object -ComObject WScript.Shell
$desktop = [Environment]::GetFolderPath("CommonDesktopDirectory")
$shortcut = $ws.CreateShortcut((Join-Path $desktop "AutoParts Offline POS.lnk"));$shortcut.TargetPath=$worker;$shortcut.Arguments="open";$shortcut.WorkingDirectory=$installDir;$shortcut.Description="Open AutoParts offline sales";$shortcut.Save()
$uninstallKey="HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\AutoPartsStoreAgent";New-Item -Path $uninstallKey -Force|Out-Null
$installedVersion=(& $manager version).Trim();Set-ItemProperty $uninstallKey DisplayName "AutoParts Store Agent";Set-ItemProperty $uninstallKey DisplayVersion $installedVersion;Set-ItemProperty $uninstallKey Publisher "AutoParts";Set-ItemProperty $uninstallKey InstallLocation $installDir;Set-ItemProperty $uninstallKey UninstallString ('powershell.exe -NoProfile -ExecutionPolicy Bypass -File "'+(Join-Path $installDir "uninstall.ps1")+'"')
Write-Host "AutoParts Store Agent $installedVersion installed with Lifecycle Manager."
