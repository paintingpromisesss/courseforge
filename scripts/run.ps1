param(
  [string]$Host = '127.0.0.1',
  [int]$Port = 8080,
  [string]$CoursesDir = '',
  [string]$DataDir = '',
  [switch]$Build
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$BinaryPath = Join-Path $RepoRoot 'bin\courseforge.exe'
$FrontendDir = Join-Path $RepoRoot 'frontend\dist'

if ([string]::IsNullOrWhiteSpace($CoursesDir)) {
  $CoursesDir = Join-Path $RepoRoot 'courses'
}
if ([string]::IsNullOrWhiteSpace($DataDir)) {
  $DataDir = Join-Path $RepoRoot 'data'
}

if ($Build -or -not (Test-Path $BinaryPath) -or -not (Test-Path (Join-Path $FrontendDir 'index.html'))) {
  & (Join-Path $PSScriptRoot 'build.ps1')
}

& $BinaryPath `
  "--host=$Host" `
  "--port=$Port" `
  "--courses-dir=$CoursesDir" `
  "--data-dir=$DataDir" `
  "--frontend-dir=$FrontendDir"
