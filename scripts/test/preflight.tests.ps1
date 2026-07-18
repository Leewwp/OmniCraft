$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$scriptPath = Join-Path $PSScriptRoot "preflight.ps1"
if (-not (Test-Path -LiteralPath $scriptPath)) {
    throw "preflight.ps1 must exist"
}

. $scriptPath

function Assert-True([bool] $Condition, [string] $Message) {
    if (-not $Condition) { throw "ASSERTION FAILED: $Message" }
}

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Assert-True ((Get-Command Invoke-Preflight).Parameters.Keys -contains "ComposeRoot") "preflight must permit a worktree's code to validate against the owning local Compose project"
$originalPath = $env:PATH
$tools = Join-Path ([System.IO.Path]::GetTempPath()) ("omnicraft-preflight-tools-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tools | Out-Null
try {
    Set-Content -LiteralPath (Join-Path $tools "docker.cmd") -Encoding ascii -Value @'
@echo off
if "%1"=="--version" echo Docker version fixture
if "%1 %2 %3"=="compose version" echo Docker Compose fixture
if "%1 %2 %3 %4"=="compose ps --format json" echo [{"Service":"postgres","State":"running","Health":"healthy"},{"Service":"redis","State":"running","Health":"healthy"}]
exit /b 0
'@
    $env:PATH = $tools + [IO.Path]::PathSeparator + $originalPath
    $cache = Join-Path $tools "go-cache"
    $safe = Test-LocalTestSettings -DatabaseAdminDsn "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=postgres sslmode=disable" -RedisAddress "127.0.0.1:6379" -RedisDatabase 15
    Assert-True $safe.IsValid "loopback postgres admin DSN and non-zero Redis DB must be accepted"

    $unsafe = Test-LocalTestSettings -DatabaseAdminDsn "host=db.example.com dbname=postgres" -RedisAddress "127.0.0.1:6379" -RedisDatabase 15
    Assert-True (-not $unsafe.IsValid) "remote database endpoints must be rejected"

    $result = Invoke-Preflight -RepoRoot $repoRoot -DatabaseAdminDsn "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=postgres sslmode=disable" -RedisAddress "127.0.0.1:6379" -RedisDatabase 15 -PortProbe { param($Address, $Port) $true } -ListenerProbe { param($Port) $false } -GoCache $cache
    Assert-True ($result.Status -eq "PASS") "fixture Docker, loopback ports, Go context, and writable GOCACHE must pass preflight"

    $blocked = Invoke-Preflight -RepoRoot $repoRoot -DatabaseAdminDsn "host=127.0.0.1 dbname=postgres" -RedisAddress "127.0.0.1:6379" -RedisDatabase 15 -PortProbe { param($Address, $Port) $Port -ne 5432 } -ListenerProbe { param($Port) $false } -GoCache $cache
    Assert-True ($blocked.Status -eq "BLOCKED") "missing Postgres host port must block real test layers"
    Assert-True ($blocked.Detail -match "5432") "blocked result must name the unavailable port"
    Assert-True ($blocked.Remediation -match "docker compose up -d --force-recreate postgres redis") "blocked result must provide non-destructive recovery command"
} finally {
    $env:PATH = $originalPath
    Remove-Item -LiteralPath $tools -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "PASS: preflight contract tests"
