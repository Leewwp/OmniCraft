$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$scriptPath = Join-Path $PSScriptRoot "readiness.ps1"
if (-not (Test-Path -LiteralPath $scriptPath)) {
    throw "readiness.ps1 must exist"
}

. $scriptPath

function Assert-True([bool] $Condition, [string] $Message) {
    if (-not $Condition) { throw "ASSERTION FAILED: $Message" }
}

$request = {
    param([string] $Uri)
    if ($Uri -like "*/healthz") { return [pscustomobject]@{ StatusCode = 200; Content = '{"status":"ok"}' } }
    if ($Uri -like "*/config/public") { return [pscustomobject]@{ StatusCode = 200; Content = '{"features":{},"captcha":{}}' } }
    return [pscustomobject]@{ StatusCode = 200; Content = '<html><body>OmniCraft</body></html>' }
}

$ready = Invoke-Readiness -Request $request -ProcessProbe { param($Process) $true }
Assert-True ($ready.Status -eq "PASS") "healthz, frontend, and real public config must be required for readiness"

$blocked = Invoke-Readiness -Request {
    param([string] $Uri)
    if ($Uri -like "*/healthz") { throw "connection refused" }
    return [pscustomobject]@{ StatusCode = 200; Content = '<html></html>' }
} -ProcessProbe { param($Process) $true }
Assert-True ($blocked.Status -eq "BLOCKED") "unreachable backend healthz must block cross-stack execution"
Assert-True ($blocked.Detail -match "healthz") "blocked readiness must identify its failed probe"

Write-Host "PASS: readiness contract tests"
