param(
  [string]$Port = "8080",
  [string]$CoursesDir = "./backend/courses",
  [string]$DataDir = "./data"
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Bin = Join-Path $RepoRoot "bin\courseforge.exe"

if (!(Test-Path $Bin)) {
  throw "Binary not found at $Bin. Please build it first."
}

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "CourseForge Comprehensive Local Binary Verification" -ForegroundColor Cyan
Write-Host "Binary: $Bin" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

function Send-RPC {
  param(
    [System.Diagnostics.Process]$Proc,
    [string]$Json
  )
  $Proc.StandardInput.WriteLine($Json)
  $Proc.StandardInput.Flush()
  $resp = $Proc.StandardOutput.ReadLine()
  if ([string]::IsNullOrWhiteSpace($resp)) {
    throw "Empty response received from MCP process"
  }
  return ($resp | ConvertFrom-Json)
}

# =========================================================================
# STEP 1: Test MCP Standby Mode (No Flags, Server Offline)
# =========================================================================
Write-Host "`n[STEP 1] Testing MCP in Standby Mode (Server Offline)..." -ForegroundColor Yellow

$mcpPsi = New-Object System.Diagnostics.ProcessStartInfo
$mcpPsi.FileName = $Bin
$mcpPsi.Arguments = "mcp"
$mcpPsi.UseShellExecute = $false
$mcpPsi.RedirectStandardInput = $true
$mcpPsi.RedirectStandardOutput = $true
$mcpPsi.RedirectStandardError = $true
$mcpPsi.CreateNoWindow = $true

$mcpProc = [System.Diagnostics.Process]::Start($mcpPsi)
Start-Sleep -Milliseconds 300

if ($mcpProc.HasExited) {
  $stderr = $mcpProc.StandardError.ReadToEnd()
  throw "MCP process exited unexpectedly! Stderr: $stderr"
}
Write-Host "  -> MCP process running in standby mode (PID: $($mcpProc.Id))" -ForegroundColor Green

# 1.1 initialize
$initResp = Send-RPC $mcpProc '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}'
if ($null -eq $initResp.result -or $initResp.error) {
  throw "Initialize failed: $($initResp | ConvertTo-Json)"
}
Write-Host "  -> JSON-RPC initialize OK: $($initResp.result.serverInfo.name) v$($initResp.result.serverInfo.version)" -ForegroundColor Green

# 1.2 tools/list
$toolsResp = Send-RPC $mcpProc '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
$toolNames = $toolsResp.result.tools | ForEach-Object { $_.name }
if ($toolNames.Count -lt 9) {
  throw "Expected 9 tools, got $($toolNames.Count)"
}
Write-Host "  -> tools/list OK: $($toolNames.Count) tools registered" -ForegroundColor Green

# 1.3 Verify each tool returns graceful error when server is offline
$sampleTools = @(
  @{ Name = "list_courses"; Args = @{} },
  @{ Name = "get_current_task"; Args = @{} },
  @{ Name = "set_active_task_context"; Args = @{ course_slug = "go-interview"; task_slug = "golang-strings-1" } },
  @{ Name = "get_task_details"; Args = @{ course_slug = "go-interview"; task_slug = "golang-strings-1" } },
  @{ Name = "get_task_template"; Args = @{ course_slug = "go-interview"; task_slug = "golang-strings-1"; language = "go" } },
  @{ Name = "run_solution"; Args = @{ course_slug = "go-interview"; task_slug = "golang-strings-1"; language = "go"; code = "package main" } }
)

$rpcId = 10
foreach ($t in $sampleTools) {
  $rpcId++
  $argsJson = $t.Args | ConvertTo-Json -Compress
  $req = @"
{"jsonrpc":"2.0","id":$rpcId,"method":"tools/call","params":{"name":"$($t.Name)","arguments":$argsJson}}
"@
  $callResp = Send-RPC $mcpProc $req
  if (!$callResp.result.isError) {
    throw "Expected tool $($t.Name) to fail when server is offline, but it succeeded!"
  }
  $errText = $callResp.result.content[0].text
  if ($errText -notmatch "CourseForge.*127\.0\.0\.1:8080") {
    throw "Unexpected error message for $($t.Name): $errText"
  }
}
Write-Host "  -> All tools correctly returned 'Server not running' error when offline" -ForegroundColor Green

# =========================================================================
# STEP 2: Start CourseForge Server on Port 8080
# =========================================================================
Write-Host "`n[STEP 2] Starting Main CourseForge Server on Port $Port..." -ForegroundColor Yellow

$serverPsi = New-Object System.Diagnostics.ProcessStartInfo
$serverPsi.FileName = $Bin
$serverPsi.Arguments = "--port=$Port --courses-dir=$CoursesDir --data-dir=$DataDir -tray=false"
$serverPsi.UseShellExecute = $false
$serverPsi.RedirectStandardOutput = $true
$serverPsi.RedirectStandardError = $true
$serverPsi.CreateNoWindow = $true

$serverProc = [System.Diagnostics.Process]::Start($serverPsi)
Write-Host "  -> Server started (PID: $($serverProc.Id))" -ForegroundColor Green

# Wait for server to become responsive
$serverUrl = "http://127.0.0.1:$Port"
$ready = $false
for ($i = 0; $i -lt 30; $i++) {
  try {
    $infoResp = Invoke-RestMethod -Uri "$serverUrl/api/info" -Method Get -TimeoutSec 1
    if ($infoResp.courses_dir -and $infoResp.data_dir) {
      $ready = $true
      break
    }
  } catch {
    Start-Sleep -Milliseconds 200
  }
}

if (!$ready) {
  $errOut = $serverProc.StandardError.ReadToEnd()
  $stdOut = $serverProc.StandardOutput.ReadToEnd()
  try { $serverProc.Kill() } catch {}
  try { $mcpProc.Kill() } catch {}
  throw "Server failed to start on $serverUrl within 6s.`nStdout: $stdOut`nStderr: $errOut"
}

Write-Host "  -> Server is UP and healthy on $serverUrl" -ForegroundColor Green
Write-Host "     courses_dir: $($infoResp.courses_dir)" -ForegroundColor DarkGray
Write-Host "     data_dir:    $($infoResp.data_dir)" -ForegroundColor DarkGray

# =========================================================================
# STEP 3: Verify Dynamic MCP Pickup (Same MCP Process!)
# =========================================================================
Write-Host "`n[STEP 3] Testing Dynamic MCP Pickup while Server is Running..." -ForegroundColor Yellow

# 3.1 list_courses
$rpcId++
$listResp = Send-RPC $mcpProc @"
{"jsonrpc":"2.0","id":$rpcId,"method":"tools/call","params":{"name":"list_courses","arguments":{}}}
"@
if ($listResp.result.isError) {
  throw "list_courses failed after server started: $($listResp.result.content[0].text)"
}
Write-Host "  -> list_courses succeeded dynamically" -ForegroundColor Green

# 3.2 set_active_task_context
$rpcId++
$setResp = Send-RPC $mcpProc @"
{"jsonrpc":"2.0","id":$rpcId,"method":"tools/call","params":{"name":"set_active_task_context","arguments":{"course_slug":"go-interview","task_slug":"golang-strings-1","language":"go"}}}
"@
if ($setResp.result.isError) {
  throw "set_active_task_context failed: $($setResp.result.content[0].text)"
}
Write-Host "  -> set_active_task_context succeeded" -ForegroundColor Green

# 3.3 get_current_task
$rpcId++
$curResp = Send-RPC $mcpProc @"
{"jsonrpc":"2.0","id":$rpcId,"method":"tools/call","params":{"name":"get_current_task","arguments":{}}}
"@
if ($curResp.result.isError) {
  throw "get_current_task failed: $($curResp.result.content[0].text)"
}
$curTaskJson = $curResp.result.content[0].text | ConvertFrom-Json
if ($curTaskJson.task_slug -ne "golang-strings-1") {
  throw "Unexpected task slug: $($curTaskJson.task_slug)"
}
Write-Host "  -> get_current_task verified active task: $($curTaskJson.title)" -ForegroundColor Green

# 3.4 get_task_solution
$rpcId++
$solResp = Send-RPC $mcpProc @"
{"jsonrpc":"2.0","id":$rpcId,"method":"tools/call","params":{"name":"get_task_solution","arguments":{"course_slug":"go-interview","task_slug":"golang-strings-1","language":"go"}}}
"@
if ($solResp.result.isError) {
  throw "get_task_solution failed: $($solResp.result.content[0].text)"
}
$solJson = $solResp.result.content[0].text | ConvertFrom-Json
Write-Host "  -> get_task_solution retrieved: $($solJson.filename) ($($solJson.code.Length) bytes)" -ForegroundColor Green

# 3.5 run_solution with correct solution code
$rpcId++
$runPayload = @{
  name = "run_solution"
  arguments = @{
    course_slug = "go-interview"
    task_slug = "golang-strings-1"
    language = "go"
    code = $solJson.code
    save_submission = $false
  }
} | ConvertTo-Json -Compress

$runResp = Send-RPC $mcpProc @"
{"jsonrpc":"2.0","id":$rpcId,"method":"tools/call","params":$runPayload}
"@
if ($runResp.result.isError) {
  throw "run_solution tool error: $($runResp.result.content[0].text)"
}
$runResult = $runResp.result.content[0].text | ConvertFrom-Json
Write-Host "  -> run_solution result: exitCode=$($runResult.exit_code), passedTests=$($runResult.passed_tests)/$($runResult.total_tests)" -ForegroundColor Green

# 3.6 read resource
$rpcId++
$resResp = Send-RPC $mcpProc @"
{"jsonrpc":"2.0","id":$rpcId,"method":"resources/read","params":{"uri":"courseforge://active-task"}}
"@
if ($resResp.error) {
  throw "read resource failed: $($resResp.error.message)"
}
Write-Host "  -> resources/read courseforge://active-task OK" -ForegroundColor Green

# =========================================================================
# STEP 4: Test Dynamic Graceful Fallback when Server Stops
# =========================================================================
Write-Host "`n[STEP 4] Stopping Server and Testing Fallback..." -ForegroundColor Yellow

$serverProc.Kill()
$serverProc.WaitForExit(3000) | Out-Null
Write-Host "  -> Server stopped. Waiting 1.2s for ping cache expiration..." -ForegroundColor Yellow
Start-Sleep -Milliseconds 1200

$rpcId++
$fallbackResp = Send-RPC $mcpProc @"
{"jsonrpc":"2.0","id":$rpcId,"method":"tools/call","params":{"name":"list_courses","arguments":{}}}
"@
if (!$fallbackResp.result.isError) {
  throw "Expected tool call to fail after server stopped, but it succeeded!"
}
$fbErr = $fallbackResp.result.content[0].text
if ($fbErr -notmatch "CourseForge.*127\.0\.0\.1:8080") {
  throw "Unexpected fallback error: $fbErr"
}
Write-Host "  -> Fallback verified: tool returned 'Server not running' error without crashing" -ForegroundColor Green

# Cleanup
try { $mcpProc.Kill() } catch {}
try { $serverProc.Kill() } catch {}

Write-Host "`n==========================================================" -ForegroundColor Green
Write-Host "All Comprehensive Verification Steps PASSED Successfully!" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
