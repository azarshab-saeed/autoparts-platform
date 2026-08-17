param([string]$OutDir = "dist")
$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$out = Join-Path $root $OutDir
New-Item -ItemType Directory -Force -Path $out | Out-Null
Push-Location $root
try {
  $env:CGO_ENABLED = "0"
  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  go build -trimpath -ldflags "-s -w -X main.version=0.15.7" -o (Join-Path $out "AutoPartsStoreEdge.exe") ./cmd/store-edge
  Write-Host "Built: $(Join-Path $out 'AutoPartsStoreEdge.exe')"
} finally { Pop-Location }
