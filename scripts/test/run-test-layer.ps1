[CmdletBinding()]
param(
    [ValidateSet("MockedBrowser", "GoUnit", "PostgresIntegration", "CrossStack")]
    [string] $Layer = "",
    [string] $RepoRoot = "",
    [string] $ComposeRoot = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
# Dot-sourced helpers have their own parameter blocks. Preserve this script's
# caller input before they are loaded so a helper cannot reset RepoRoot.
$RequestedRepoRoot = $RepoRoot
$RequestedComposeRoot = $ComposeRoot
. (Join-Path $PSScriptRoot "preflight.ps1")
. (Join-Path $PSScriptRoot "readiness.ps1")

function New-TemporaryDatabaseName {
    return "omnicraft_test_" + [guid]::NewGuid().ToString("N").Substring(0, 16)
}

function Get-RunnerGoCache {
    return Join-Path ([IO.Path]::GetTempPath()) "omnicraft-go-cache"
}

function Stop-OwnedProcessTree {
    param($Process)
    if ($null -eq $Process) { return [pscustomobject]@{ Success = $true; Detail = "" } }
    # `go run` and `next dev` spawn children that can outlive their launcher.
    # /T is intentionally limited to the known PID tree created by this runner.
    try {
        if ($Process.HasExited) { return [pscustomobject]@{ Success = $true; Detail = "" } }
    } catch { }
    $previousErrorAction = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try { $output = @(& taskkill.exe /PID $Process.Id /T /F 2>&1) } finally { $ErrorActionPreference = $previousErrorAction }
    if ($LASTEXITCODE -eq 0) { return [pscustomobject]@{ Success = $true; Detail = "" } }
    try {
        if ($Process.HasExited) { return [pscustomobject]@{ Success = $true; Detail = "" } }
    } catch { }
    return [pscustomobject]@{ Success = $false; Detail = "owned process $($Process.Id) cleanup failed: $($output -join '; ')" }
}

function Invoke-TestLayer {
    param([ValidateSet("MockedBrowser", "GoUnit", "PostgresIntegration", "CrossStack")] [string] $Layer, [scriptblock] $Action, [string] $Blocker)
    $label = if ($Layer -eq "MockedBrowser") { "mocked browser contract" } else { $Layer }
    if (-not [string]::IsNullOrWhiteSpace($Blocker)) { return [pscustomobject]@{ Layer = $Layer; Label = $label; Status = "BLOCKED"; ExitCode = 2; Detail = $Blocker } }
    $outcome = & $Action
    if ($outcome.ExitCode -eq 0) { return [pscustomobject]@{ Layer = $Layer; Label = $label; Status = "PASS"; ExitCode = 0; Detail = $outcome.Detail } }
    return [pscustomobject]@{ Layer = $Layer; Label = $label; Status = "FAIL"; ExitCode = [int]$outcome.ExitCode; Detail = $outcome.Detail }
}

function Invoke-RunnerCommand {
    param([string] $WorkingDirectory, [string] $File, [string[]] $Arguments)
    $result = Invoke-LocalNative -File $File -Arguments $Arguments -WorkingDirectory $WorkingDirectory
    return [pscustomobject]@{ ExitCode = $result.ExitCode; Detail = ($result.Output.Trim()) }
}

function Invoke-ComposeSql {
    param([string] $ComposeRoot, [string] $Database, [string] $Sql)
    $composeFile = Join-Path $ComposeRoot "docker-compose.yml"
    if (-not (Test-Path -LiteralPath $composeFile)) { throw "Compose file is missing: $composeFile" }
    try {
        $previousErrorAction = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        try { $result = $Sql | & docker compose -f $composeFile --project-directory $ComposeRoot exec -T postgres psql -v ON_ERROR_STOP=1 -U omnicraft -d $Database 2>&1 } finally { $ErrorActionPreference = $previousErrorAction }
        if ($LASTEXITCODE -ne 0) { throw "PostgreSQL command failed: $($result -join '; ')" }
    } finally { }
}

function Initialize-TemporaryDatabase {
    param([string] $RepoRoot, [string] $ComposeRoot, [string] $Database)
    if ($Database -notmatch '^omnicraft_test_[a-z0-9_]+$') { throw "refusing unsafe test database name" }
    Invoke-ComposeSql -ComposeRoot $ComposeRoot -Database "postgres" -Sql "CREATE DATABASE $Database;"
    try {
        Get-ChildItem -LiteralPath (Join-Path $RepoRoot "backend/migrations") -Filter "*.sql" | Sort-Object Name | ForEach-Object {
            Invoke-ComposeSql -ComposeRoot $ComposeRoot -Database $Database -Sql (Get-Content -Raw -LiteralPath $_.FullName)
        }
    } catch {
        Invoke-ComposeSql -ComposeRoot $ComposeRoot -Database "postgres" -Sql "DROP DATABASE IF EXISTS $Database WITH (FORCE);"
        throw
    }
}

function Remove-TemporaryDatabase {
    param([string] $ComposeRoot, [string] $Database)
    if ($Database -match '^omnicraft_test_[a-z0-9_]+$') { Invoke-ComposeSql -ComposeRoot $ComposeRoot -Database "postgres" -Sql "DROP DATABASE IF EXISTS $Database WITH (FORCE);" }
}

function Clear-TestRedisDatabase {
    param([string] $ComposeRoot, [int] $RedisDatabase)
    if ($RedisDatabase -le 0) { throw "refusing to clear Redis database 0" }
    $composeFile = Join-Path $ComposeRoot "docker-compose.yml"
    if (-not (Test-Path -LiteralPath $composeFile)) { throw "Compose file is missing: $composeFile" }
    try {
        $previousErrorAction = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        try { $output = & docker compose -f $composeFile --project-directory $ComposeRoot exec -T redis redis-cli -n $RedisDatabase FLUSHDB 2>&1 } finally { $ErrorActionPreference = $previousErrorAction }
        if ($LASTEXITCODE -ne 0) { throw "Redis test cleanup failed: $($output -join '; ')" }
    } finally { }
}

function Write-LayerResult { param($Result) Write-Host "LAYER=$($Result.Layer) STATUS=$($Result.Status) LABEL=$($Result.Label) DETAIL=$($Result.Detail)" }

function Resolve-RealLayerFailure {
    param(
        [ValidateSet("PostgresIntegration", "CrossStack")] [string] $Layer,
        [string] $Detail,
        [bool] $ExecutionStarted
    )
    if (-not $ExecutionStarted) { return Invoke-TestLayer -Layer $Layer -Blocker $Detail }
    return [pscustomobject]@{ Layer = $Layer; Label = $Layer; Status = "FAIL"; ExitCode = 1; Detail = $Detail }
}

function Resolve-CleanupOutcome {
    param($Result, [string[]] $CleanupErrors)
    $errors = @($CleanupErrors | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($errors.Count -eq 0) { return $Result }
    $detail = "$($Result.Detail); cleanup failed: $($errors -join '; ')"
    if ($Result.Status -eq "PASS") {
        return [pscustomobject]@{ Layer = $Result.Layer; Label = $Result.Label; Status = "FAIL"; ExitCode = 1; Detail = $detail }
    }
    return [pscustomobject]@{ Layer = $Result.Layer; Label = $Result.Label; Status = $Result.Status; ExitCode = $Result.ExitCode; Detail = $detail }
}

function Invoke-RealDatabaseLayer {
    param([string] $Layer, [string] $RepoRoot, [string] $ComposeRoot = $RepoRoot)
    $adminDsn = if ($env:OMNICRAFT_TEST_ADMIN_DB_DSN) { $env:OMNICRAFT_TEST_ADMIN_DB_DSN } else { "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=postgres sslmode=disable" }
    $redisAddress = if ($env:REDIS_ADDR) { $env:REDIS_ADDR } else { "127.0.0.1:6379" }
    $redisDatabase = if ($env:OMNICRAFT_TEST_REDIS_DB) { [int]$env:OMNICRAFT_TEST_REDIS_DB } else { 15 }
    $goCache = Get-RunnerGoCache
    $preflight = Invoke-Preflight -RepoRoot $RepoRoot -ComposeRoot $ComposeRoot -DatabaseAdminDsn $adminDsn -RedisAddress $redisAddress -RedisDatabase $redisDatabase -GoCache $goCache
    if ($preflight.Status -ne "PASS") { return Invoke-TestLayer -Layer $Layer -Blocker "$($preflight.Detail) Remediation: $($preflight.Remediation)" }
    $database = New-TemporaryDatabaseName
    $executionStarted = $false
    $backendProcess = $null; $frontendProcess = $null
    $cleanupErrors = [System.Collections.Generic.List[string]]::new()
    $result = $null
    $old = @{ OMNICRAFT_TEST_MODE = $env:OMNICRAFT_TEST_MODE; OMNICRAFT_TEST_DB_DSN = $env:OMNICRAFT_TEST_DB_DSN; OMNICRAFT_TEST_REDIS_DB = $env:OMNICRAFT_TEST_REDIS_DB; REDIS_ADDR = $env:REDIS_ADDR; REDIS_DB = $env:REDIS_DB; NEXT_PUBLIC_API_URL = $env:NEXT_PUBLIC_API_URL; GOCACHE = $env:GOCACHE }
    try {
        # A disposable database/migration run is part of the real test layer,
        # not an unmet host prerequisite. Failures from here must be visible as FAIL.
        $executionStarted = $true
        Initialize-TemporaryDatabase -RepoRoot $RepoRoot -ComposeRoot $ComposeRoot -Database $database
        Clear-TestRedisDatabase -ComposeRoot $ComposeRoot -RedisDatabase $redisDatabase
        $env:OMNICRAFT_TEST_MODE = "1"
        $env:OMNICRAFT_TEST_DB_DSN = ($adminDsn -replace '(?i)dbname=postgres', "dbname=$database")
        $env:OMNICRAFT_TEST_REDIS_DB = "$redisDatabase"
        $env:REDIS_ADDR = $redisAddress; $env:REDIS_DB = "$redisDatabase"; $env:GOCACHE = $goCache; $env:NEXT_PUBLIC_API_URL = "http://127.0.0.1:8080"
        if ($Layer -eq "PostgresIntegration") {
            $result = Invoke-TestLayer -Layer $Layer -Action { Invoke-RunnerCommand -WorkingDirectory (Join-Path $RepoRoot "backend") -File "go" -Arguments @("test", "./...") }
        } else {
            $backendProcess = Start-Process -FilePath "go" -ArgumentList @("run", "cmd/server/main.go") -WorkingDirectory (Join-Path $RepoRoot "backend") -PassThru -NoNewWindow
            $frontendProcess = Start-Process -FilePath "npm.cmd" -ArgumentList @("run", "dev", "--", "--hostname", "127.0.0.1", "--port", "3000") -WorkingDirectory (Join-Path $RepoRoot "frontend") -PassThru -NoNewWindow
            $readiness = $null
            foreach ($attempt in 1..30) { $readiness = Invoke-Readiness -BackendProcess $backendProcess -FrontendProcess $frontendProcess; if ($readiness.Status -eq "PASS") { break }; Start-Sleep -Seconds 2 }
            if ($readiness.Status -ne "PASS") {
                $result = Resolve-RealLayerFailure -Layer $Layer -Detail $readiness.Detail -ExecutionStarted $executionStarted
            } else {
                $result = Invoke-TestLayer -Layer $Layer -Action { Invoke-RunnerCommand -WorkingDirectory (Join-Path $RepoRoot "frontend") -File "npm.cmd" -Arguments @("run", "test:cross-stack") }
            }
        }
    } catch {
        $result = Resolve-RealLayerFailure -Layer $Layer -Detail $_.Exception.Message -ExecutionStarted $executionStarted
    } finally {
        foreach ($process in @($frontendProcess, $backendProcess)) {
            $processCleanup = Stop-OwnedProcessTree $process
            if (-not $processCleanup.Success) { $cleanupErrors.Add($processCleanup.Detail) }
        }
        try { Clear-TestRedisDatabase -ComposeRoot $ComposeRoot -RedisDatabase $redisDatabase } catch { $cleanupErrors.Add("Redis DB $redisDatabase cleanup failed: $($_.Exception.Message)") }
        try { Remove-TemporaryDatabase -ComposeRoot $ComposeRoot -Database $database } catch { $cleanupErrors.Add("temporary database $database cleanup failed: $($_.Exception.Message)") }
        foreach ($name in $old.Keys) { Set-Item -Path "Env:$name" -Value $old[$name] }
    }
    return Resolve-CleanupOutcome -Result $result -CleanupErrors $cleanupErrors.ToArray()
}

if ($MyInvocation.InvocationName -ne ".") {
    if ([string]::IsNullOrWhiteSpace($Layer)) { throw "-Layer is required" }
    if ([string]::IsNullOrWhiteSpace($RequestedRepoRoot)) { $RequestedRepoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot) }
    if ([string]::IsNullOrWhiteSpace($RequestedComposeRoot)) { $RequestedComposeRoot = $RequestedRepoRoot }
    if ($Layer -eq "MockedBrowser") {
        $result = Invoke-TestLayer -Layer $Layer -Action { Invoke-RunnerCommand -WorkingDirectory (Join-Path $RequestedRepoRoot "frontend") -File "npm.cmd" -Arguments @("run", "test:contracts") }
    } elseif ($Layer -eq "GoUnit") {
        $previousCache = $env:GOCACHE
        $env:GOCACHE = Get-RunnerGoCache
        try {
            New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
            $result = Invoke-TestLayer -Layer $Layer -Action { Invoke-RunnerCommand -WorkingDirectory (Join-Path $RequestedRepoRoot "backend") -File "go" -Arguments @("test", "./...") }
        } finally {
            $env:GOCACHE = $previousCache
        }
    } else {
        $result = Invoke-RealDatabaseLayer -Layer $Layer -RepoRoot $RequestedRepoRoot -ComposeRoot $RequestedComposeRoot
    }
    Write-LayerResult $result
    exit $result.ExitCode
}
