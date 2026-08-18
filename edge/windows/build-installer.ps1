param(
  [string]$Version = "0.15.8.2",
  [ValidateSet("amd64")][string]$Arch = "amd64"
)
$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$dist = Join-Path $root "dist"

& (Join-Path $PSScriptRoot "build.ps1") -OutDir "dist" -Version $Version -Arch $Arch

$candidates = @(
  (Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe"),
  (Join-Path $env:ProgramFiles "Inno Setup 6\ISCC.exe")
) | Where-Object { $_ -and (Test-Path $_) }
if (-not $candidates -or $candidates.Count -eq 0) {
  throw "Inno Setup 6 (ISCC.exe) was not found. Install Inno Setup 6 on the Windows release machine, then rerun this script."
}
$iscc = $candidates[0]
$iss = Join-Path $PSScriptRoot "installer\AutoPartsStoreAgent.iss"
& $iscc "/DMyAppVersion=$Version" "/DRepoRoot=$root" $iss
if ($LASTEXITCODE -ne 0) { throw "Inno Setup build failed with exit code $LASTEXITCODE" }

$setup = Join-Path $dist "AutoParts-Store-Agent-Setup-$Version.exe"
if (-not (Test-Path $setup)) { throw "Expected installer was not produced: $setup" }

$thumbprint = [string]$env:AUTOPARTS_SIGN_CERT_SHA1
if ($thumbprint.Trim()) {
  $signtool = (Get-Command signtool.exe -ErrorAction SilentlyContinue).Source
  if (-not $signtool) { throw "AUTOPARTS_SIGN_CERT_SHA1 is set, but signtool.exe is not available." }
  & $signtool sign /sha1 $thumbprint.Trim() /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 $setup
  if ($LASTEXITCODE -ne 0) { throw "Code signing failed." }
}

$hash = (Get-FileHash -Algorithm SHA256 $setup).Hash.ToLowerInvariant()
"$hash  $(Split-Path $setup -Leaf)" | Set-Content -Encoding ASCII "$setup.sha256"
Write-Host "Release installer: $setup"
Write-Host "SHA256: $hash"
if (-not $thumbprint.Trim()) { Write-Warning "Installer is unsigned. Sign customer releases by setting AUTOPARTS_SIGN_CERT_SHA1 on the release machine." }

$stableSetup = Join-Path $dist "AutoParts-Store-Agent-Setup-windows-x64.exe"
Copy-Item $setup $stableSetup -Force
$stableHash = (Get-FileHash -Algorithm SHA256 $stableSetup).Hash.ToLowerInvariant()
"$stableHash  $(Split-Path $stableSetup -Leaf)" | Set-Content -Encoding ASCII "$stableSetup.sha256"
Write-Host "Stable installer alias: $stableSetup"
