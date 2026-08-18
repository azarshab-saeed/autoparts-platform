param(
  [string]$OutDir = "dist",
  [string]$Version = "0.15.8.1",
  [ValidateSet("amd64", "arm64")][string]$Arch = "amd64"
)
$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$out = Join-Path $root $OutDir
New-Item -ItemType Directory -Force -Path $out | Out-Null
$worker = Join-Path $out "AutoPartsStoreEdge.exe"
$manager = Join-Path $out "AutoPartsStoreEdgeManager.exe"
$updater = Join-Path $out "AutoPartsStoreEdgeUpdater.exe"

$manifestUrl = [string]$env:AUTOPARTS_EDGE_UPDATE_MANIFEST_URL
$publicKey = [string]$env:AUTOPARTS_EDGE_UPDATE_PUBLIC_KEY

Push-Location $root
try {
  $env:CGO_ENABLED = "0"
  $env:GOOS = "windows"
  $env:GOARCH = $Arch
  go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$Version" -o $worker ./cmd/store-edge
  if ($LASTEXITCODE -ne 0) { throw "Store Edge worker build failed with exit code $LASTEXITCODE" }

  $managerLdflags = "-s -w -X main.version=$Version"
  if ($manifestUrl.Trim()) { $managerLdflags += " -X main.defaultUpdateManifestURL=$($manifestUrl.Trim())" }
  if ($publicKey.Trim()) { $managerLdflags += " -X main.defaultUpdatePublicKey=$($publicKey.Trim())" }
  go build -buildvcs=false -trimpath -ldflags $managerLdflags -o $manager ./cmd/store-edge-manager
  if ($LASTEXITCODE -ne 0) { throw "Store Edge Manager build failed with exit code $LASTEXITCODE" }

  go build -buildvcs=false -trimpath -ldflags "-s -w" -o $updater ./cmd/store-edge-updater
  if ($LASTEXITCODE -ne 0) { throw "Store Edge Updater build failed with exit code $LASTEXITCODE" }
} finally { Pop-Location }

foreach ($file in @($worker,$manager,$updater)) {
  $hash = (Get-FileHash -Algorithm SHA256 $file).Hash.ToLowerInvariant()
  "$hash  $(Split-Path $file -Leaf)" | Set-Content -Encoding ASCII "$file.sha256"
  Write-Host "Built: $file"
  Write-Host "SHA256: $hash"
}
if (-not $manifestUrl.Trim() -or -not $publicKey.Trim()) {
  Write-Warning "Signed auto-update is disabled in this build. Set AUTOPARTS_EDGE_UPDATE_MANIFEST_URL and AUTOPARTS_EDGE_UPDATE_PUBLIC_KEY on the release builder."
}
