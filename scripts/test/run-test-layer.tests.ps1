$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$scriptPath = Join-Path $PSScriptRoot "run-test-layer.ps1"
if (-not (Test-Path -LiteralPath $scriptPath)) {
    throw "run-test-layer.ps1 must exist"
}

. $scriptPath

function Assert-True([bool] $Condition, [string] $Message) {
    if (-not $Condition) { throw "ASSERTION FAILED: $Message" }
}

$mocked = Invoke-TestLayer -Layer MockedBrowser -Action { [pscustomobject]@{ ExitCode = 0; Detail = "contracts passed" } }
Assert-True ($mocked.Status -eq "PASS") "successful mocked browser contract must report PASS"
Assert-True ($mocked.Label -eq "mocked browser contract") "mocked browser layer must never be labeled cross-stack"

$failed = Invoke-TestLayer -Layer GoUnit -Action { [pscustomobject]@{ ExitCode = 17; Detail = "go test failed" } }
Assert-True ($failed.Status -eq "FAIL") "executed failing test command must report FAIL"

$blocked = Invoke-TestLayer -Layer CrossStack -Blocker "PostgreSQL host port 5432 is unavailable"
Assert-True ($blocked.Status -eq "BLOCKED") "missing preflight must report BLOCKED rather than PASS"
Assert-True ($blocked.ExitCode -ne 0) "every non-pass layer result must have a non-zero exit code"

$name = New-TemporaryDatabaseName
Assert-True ($name -match '^omnicraft_test_[a-z0-9_]+$') "temporary database name must stay inside the test-only namespace"

$cache = Get-RunnerGoCache
Assert-True ($cache -match 'omnicraft-go-cache') "runner must use an isolated writable Go cache instead of inheriting an inaccessible default"

$previousErrorAction = $ErrorActionPreference
$ErrorActionPreference = "Continue"
try { $directOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $scriptPath 2>&1 } finally { $ErrorActionPreference = $previousErrorAction }
Assert-True ($LASTEXITCODE -ne 0) "direct runner invocation without a layer must fail instead of silently returning success"
Assert-True (($directOutput -join "`n") -match "-Layer is required") "direct runner invocation must execute its own command entry point after loading helpers"
Assert-True ((Get-Command Stop-OwnedProcessTree -ErrorAction SilentlyContinue) -ne $null) "runner must stop complete owned process trees during cleanup"

$setupFailure = Resolve-RealLayerFailure -Layer PostgresIntegration -Detail "migration setup failed" -ExecutionStarted $false
Assert-True ($setupFailure.Status -eq "BLOCKED") "unmet real-layer setup must report BLOCKED"

$executedFailure = Resolve-RealLayerFailure -Layer CrossStack -Detail "Playwright assertion failed" -ExecutionStarted $true
Assert-True ($executedFailure.Status -eq "FAIL") "a failure after real test execution starts must report FAIL"
Assert-True ($executedFailure.ExitCode -ne 0) "executed failures must produce a non-zero exit code"

$readinessFailure = Resolve-RealLayerFailure -Layer CrossStack -Detail "backend healthz: connection refused" -ExecutionStarted $true
Assert-True ($readinessFailure.Status -eq "FAIL") "readiness failure after owned services start must report FAIL rather than BLOCKED"

$successfulLayer = [pscustomobject]@{ Layer = "CrossStack"; Label = "CrossStack"; Status = "PASS"; ExitCode = 0; Detail = "cross-stack passed" }
$cleanupFailure = Resolve-CleanupOutcome -Result $successfulLayer -CleanupErrors @("Redis DB 15 cleanup failed", "owned frontend process cleanup failed")
Assert-True ($cleanupFailure.Status -eq "FAIL") "cleanup failure must convert an otherwise passing real layer to FAIL"
Assert-True ($cleanupFailure.ExitCode -ne 0) "cleanup failure after PASS must produce a non-zero exit code"
Assert-True ($cleanupFailure.Detail -match "Redis DB 15 cleanup failed") "cleanup failure detail must be retained in the layer report"

$preservedFailure = Resolve-CleanupOutcome -Result $executedFailure -CleanupErrors @("temporary database cleanup failed")
Assert-True ($preservedFailure.Status -eq "FAIL") "cleanup must not relabel an existing executed failure"

Write-Host "PASS: test layer runner contract tests"
