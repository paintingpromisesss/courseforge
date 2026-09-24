param(
  [switch]$SkipDeps,
  [string]$GoArch = ''
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

$WebDistDir = Join-Path $BackendDir 'internal\web\dist'
New-Item -ItemType Directory -Force -Path $WebDistDir | Out-Null
Copy-Item -Recurse -Force (Join-Path $FrontendDir 'dist\*') $WebDistDir

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

Push-Location $BackendDir
try {
  # swag runs as a tool on THIS machine (via `go run`) — it must build for the
  # host arch, not whatever $GoArch cross-compiles the final binary for.
  Invoke-CheckedNative go @('run', 'github.com/swaggo/swag/cmd/swag', 'init', '-g', 'main.go', '-d', './cmd/server,./internal/api/handlers,./internal/api/dto', '-o', './docs', '--exclude', './courses')

  $Version = (git -C $RepoRoot describe --tags --always --dirty 2>$null)
  # Built with -H=windowsgui: no console window is ever allocated when double-clicked
  # from Explorer. When run from a terminal (CLI/MCP), attachConsoleIfAvailable()
  # attaches to the parent console at startup so stdout/stderr work properly.
  $LdFlags = "-X main.version=$Version -H=windowsgui"
  if ($GoArch) { $env:GOARCH = $GoArch }
  Invoke-CheckedNative go @('build', '-tags', 'swagger', '-ldflags', $LdFlags, '-o', $BinaryPath, './cmd/courseforge')
} finally {
  Pop-Location
}

Write-Host "Built $BinaryPath"
