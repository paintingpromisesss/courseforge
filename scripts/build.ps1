param(
  [switch]$SkipDeps
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$FrontendDir = Join-Path $RepoRoot 'frontend'
$BackendDir = Join-Path $RepoRoot 'backend'
$BinDir = Join-Path $RepoRoot 'bin'
$BinaryPath = Join-Path $BinDir 'courseforge.exe'
$GoCacheDir = Join-Path $RepoRoot '.cache\go-build'

New-Item -ItemType Directory -Force -Path $GoCacheDir | Out-Null
$env:GOCACHE = $GoCacheDir

if (-not $SkipDeps -or -not (Test-Path (Join-Path $FrontendDir 'node_modules'))) {
  Push-Location $FrontendDir
  try {
    npm.cmd ci
  } finally {
    Pop-Location
  }
}

Push-Location $FrontendDir
try {
  npm.cmd run build
} finally {
  Pop-Location
}

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

Push-Location $BackendDir
try {
  go build -o $BinaryPath ./cmd/courseforge
} finally {
  Pop-Location
}

Write-Host "Built $BinaryPath"
