param(
  [string]$InstallDir = '',
  [switch]$SkipDeps
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$BinaryPath = Join-Path $RepoRoot 'bin\courseforge.exe'

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
  $InstallDir = Join-Path $HOME '.courseforge\bin'
}

$InstallDir = [System.IO.Path]::GetFullPath($InstallDir)

if ($SkipDeps) {
  & (Join-Path $PSScriptRoot 'build.ps1') -SkipDeps
} else {
  & (Join-Path $PSScriptRoot 'build.ps1')
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Force $BinaryPath (Join-Path $InstallDir 'courseforge.exe')

Write-Host "Installed to $(Join-Path $InstallDir 'courseforge.exe')"
Write-Host "If needed, add $InstallDir to PATH."
