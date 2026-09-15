param(
  [string]$CoursesDir = "./backend/courses",
  [string]$DataDir = "./data"
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$McpBin = Join-Path $RepoRoot "bin\courseforge-mcp.exe"

Write-Host "==> [Smoke Test] Building courseforge-mcp..." -ForegroundColor Cyan
Push-Location (Join-Path $RepoRoot "backend")
try {
  & go build -o $McpBin ./cmd/mcp
  if ($LASTEXITCODE -ne 0) { throw "go build failed" }
} finally {
  Pop-Location
}

Write-Host "==> [Smoke Test] Launching MCP Server via STDIO..." -ForegroundColor Cyan
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $McpBin
$psi.Arguments = "--courses-dir=$CoursesDir --data-dir=$DataDir --transport=stdio"
$psi.UseShellExecute = $false
$psi.RedirectStandardInput = $true
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.CreateNoWindow = $true

$proc = [System.Diagnostics.Process]::Start($psi)

function Send-RPC {
  param([string]$json)
  $proc.StandardInput.WriteLine($json)
  $proc.StandardInput.Flush()
  $resp = $proc.StandardOutput.ReadLine()
  return ($resp | ConvertFrom-Json)
}

try {
  # 1. initialize
  Write-Host "-> Testing JSON-RPC initialize..." -ForegroundColor Yellow
  $initResp = Send-RPC '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke-tester","version":"1.0"}}}'
  if ($null -eq $initResp.result -or $initResp.error) {
    throw "initialize failed: $($initResp | ConvertTo-Json)"
  }
  Write-Host "   [OK] Server name: $($initResp.result.serverInfo.name) v$($initResp.result.serverInfo.version)" -ForegroundColor Green

  # 2. tools/list
  Write-Host "-> Testing tools/list..." -ForegroundColor Yellow
  $toolsResp = Send-RPC '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
  $toolCount = $toolsResp.result.tools.Count
  if ($toolCount -lt 9) {
    throw "Expected at least 9 tools, got $toolCount"
  }
  Write-Host "   [OK] Registered tools count: $toolCount" -ForegroundColor Green

  # 3. tools/call list_courses
  Write-Host "-> Testing tools/call list_courses..." -ForegroundColor Yellow
  $listResp = Send-RPC '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_courses","arguments":{}}}'
  if ($listResp.result.isError) {
    throw "list_courses error: $($listResp.result.content[0].text)"
  }
  Write-Host "   [OK] Courses listed successfully" -ForegroundColor Green

  # 4. tools/call set_active_task_context
  Write-Host "-> Testing tools/call set_active_task_context..." -ForegroundColor Yellow
  $setActiveResp = Send-RPC '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"set_active_task_context","arguments":{"course_slug":"go-interview","task_slug":"golang-strings-1","language":"go"}}}'
  if ($setActiveResp.result.isError) {
    throw "set_active_task_context error: $($setActiveResp.result.content[0].text)"
  }
  Write-Host "   [OK] Active task context updated" -ForegroundColor Green

  # 5. tools/call get_current_task
  Write-Host "-> Testing tools/call get_current_task..." -ForegroundColor Yellow
  $getTaskResp = Send-RPC '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_current_task","arguments":{}}}'
  if ($getTaskResp.result.isError -or -not ($getTaskResp.result.content[0].text -match "golang-strings-1")) {
    throw "get_current_task failed: $($getTaskResp.result.content[0].text)"
  }
  Write-Host "   [OK] Current task context retrieved" -ForegroundColor Green

  # 6. tools/call get_task_solution
  Write-Host "-> Testing tools/call get_task_solution..." -ForegroundColor Yellow
  $solResp = Send-RPC '{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"get_task_solution","arguments":{"course_slug":"go-interview","task_slug":"golang-strings-1","language":"go"}}}'
  if ($solResp.result.isError) {
    throw "get_task_solution error: $($solResp.result.content[0].text)"
  }
  $solData = ($solResp.result.content[0].text | ConvertFrom-Json)
  Write-Host "   [OK] Solution retrieved (filename: $($solData.filename))" -ForegroundColor Green

  # 7. tools/call run_solution
  Write-Host "-> Testing tools/call run_solution with runner execution..." -ForegroundColor Yellow
  $runArg = @{
    name = "run_solution"
    arguments = @{
      course_slug = "go-interview"
      task_slug = "golang-strings-1"
      language = "go"
      code = $solData.code
      save_submission = $true
    }
  }
  $runRpc = @{
    jsonrpc = "2.0"
    id = 7
    method = "tools/call"
    params = $runArg
  } | ConvertTo-Json -Depth 5 -Compress
  $runResp = Send-RPC $runRpc
  if ($runResp.result.isError -or -not ($runResp.result.content[0].text -match "PASS")) {
    throw "run_solution failed: $($runResp.result.content[0].text)"
  }
  Write-Host "   [OK] Solution verified by runner, tests PASS" -ForegroundColor Green

  # 8. resources/read courseforge://active-task
  Write-Host "-> Testing resources/read courseforge://active-task..." -ForegroundColor Yellow
  $resResp = Send-RPC '{"jsonrpc":"2.0","id":8,"method":"resources/read","params":{"uri":"courseforge://active-task"}}'
  if ($null -eq $resResp.result) {
    throw "resources/read failed: $($resResp | ConvertTo-Json)"
  }
  Write-Host "   [OK] Resource read successfully" -ForegroundColor Green

  Write-Host "`nAll MCP Smoke Tests Passed Successfully!" -ForegroundColor Green
} finally {
  if (-not $proc.HasExited) {
    $proc.Kill()
  }
  $proc.Dispose()
}
