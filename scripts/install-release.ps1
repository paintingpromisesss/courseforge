<#
  Installs a prebuilt courseforge.exe release into %LOCALAPPDATA%\CourseForge
  and adds it to the user PATH. No Go/Node toolchain required.

  Usage:
    irm https://raw.githubusercontent.com/paintingpromisesss/courseforge/main/scripts/install-release.ps1 | iex
    .\scripts\install-release.ps1 -Tag v1.2.3
#>
param(
  [string]$Tag = 'latest',
  [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'CourseForge')
)

$ErrorActionPreference = 'Stop'
$Repo = 'paintingpromisesss/courseforge'

$ReleaseUrl = if ($Tag -eq 'latest') {
  "https://api.github.com/repos/$Repo/releases/latest"
} else {
  "https://api.github.com/repos/$Repo/releases/tags/$Tag"
}

Write-Host "Fetching release info ($Tag)..."
$Release = Invoke-RestMethod -Uri $ReleaseUrl -Headers @{ 'User-Agent' = 'courseforge-installer' }

$Arch = if ([Environment]::Is64BitOperatingSystem -and $env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$AssetName = "courseforge-windows-$Arch.exe"

$Asset = $Release.assets | Where-Object { $_.name -eq $AssetName }
if (-not $Asset) {
  throw "No $AssetName asset found in release $($Release.tag_name)"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$BinaryPath = Join-Path $InstallDir 'courseforge.exe'

Write-Host "Downloading $($Asset.name) ($($Release.tag_name))..."
Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $BinaryPath

$UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$PathEntries = $UserPath -split ';' | Where-Object { $_ -ne '' }
if ($PathEntries -notcontains $InstallDir) {
  $NewPath = if ($UserPath) { "$UserPath;$InstallDir" } else { $InstallDir }
  [Environment]::SetEnvironmentVariable('Path', $NewPath, 'User')
  Write-Host "Added $InstallDir to user PATH."
} else {
  Write-Host "$InstallDir already on PATH."
}

# Broadcast the change so new shells pick it up without a reboot.
$signature = @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
$type = Add-Type -MemberDefinition $signature -Name Win32SendMessageTimeout -Namespace Win32Functions -PassThru
$HWND_BROADCAST = [IntPtr]0xffff
$WM_SETTINGCHANGE = 0x1a
$result = [UIntPtr]::Zero
$type::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result) | Out-Null

Write-Host ""
Write-Host "Installed to $BinaryPath"
Write-Host "Open a new terminal and run: courseforge"
