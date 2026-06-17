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

function Invoke-CheckedNative {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Command,

    [string[]]$Arguments
  )

  & $Command @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "$Command $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
  }
}

New-Item -ItemType Directory -Force -Path $GoCacheDir | Out-Null
$env:GOCACHE = $GoCacheDir

if (-not $SkipDeps -or -not (Test-Path (Join-Path $FrontendDir 'node_modules'))) {
  Push-Location $FrontendDir
  try {
    Invoke-CheckedNative npm.cmd @('ci')
  } finally {
    Pop-Location
  }
}

Push-Location $FrontendDir
try {
  Invoke-CheckedNative npm.cmd @('run', 'build')
} finally {
  Pop-Location
}

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

Push-Location $BackendDir
try {
  Invoke-CheckedNative go @('run', 'github.com/swaggo/swag/cmd/swag', 'init', '-g', 'main.go', '-d', './cmd/server,./internal/api/handlers,./internal/api/dto', '-o', './docs', '--exclude', './courses')
  Invoke-CheckedNative go @('build', '-tags', 'swagger', '-o', $BinaryPath, './cmd/courseforge')
} finally {
  Pop-Location
}

Write-Host "Built $BinaryPath"
