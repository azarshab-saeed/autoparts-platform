param(
  [string]$OutDir = "dist",
  [string]$Version = "0.15.7.1",
  [ValidateSet("amd64", "arm64")][string]$Arch = "amd64"
)
$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$out = Join-Path $root $OutDir
New-Item -ItemType Directory -Force -Path $out | Out-Null
$exe = Join-Path $out "AutoPartsStoreEdge.exe"

Push-Location $root
try {
  $env:CGO_ENABLED = "0"
  $env:GOOS = "windows"
  $env:GOARCH = $Arch
  go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$Version" -o $exe ./cmd/store-edge
  if ($LASTEXITCODE -ne 0) { throw "Go build failed with exit code $LASTEXITCODE" }
} finally { Pop-Location }

$hash = (Get-FileHash -Algorithm SHA256 $exe).Hash.ToLowerInvariant()
"$hash  AutoPartsStoreEdge.exe" | Set-Content -Encoding ASCII (Join-Path $out "AutoPartsStoreEdge.exe.sha256")
Write-Host "Built Store Agent $Version ($Arch): $exe"
Write-Host "SHA256: $hash"
