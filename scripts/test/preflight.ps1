[CmdletBinding()]
param(
    [string] $RepoRoot = "",
    [string] $ComposeRoot = "",
    [string] $DatabaseAdminDsn = $env:OMNICRAFT_TEST_ADMIN_DB_DSN,
    [string] $RedisAddress = $(if ($env:REDIS_ADDR) { $env:REDIS_ADDR } else { "127.0.0.1:6379" }),
    [int] $RedisDatabase = $(if ($env:OMNICRAFT_TEST_REDIS_DB) { [int]$env:OMNICRAFT_TEST_REDIS_DB } else { 15 }),
    [string] $GoCache = $(if ($env:GOCACHE) { $env:GOCACHE } else { Join-Path ([IO.Path]::GetTempPath()) "omnicraft-go-cache" })
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$script:RecoveryCommand = "docker compose up -d --force-recreate postgres redis"

function Invoke-LocalNative {
    param([string] $File, [string[]] $Arguments, [string] $WorkingDirectory)
    Push-Location $WorkingDirectory
    try {
        $previousErrorAction = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        try { $output = @(& $File @Arguments 2>&1) } finally { $ErrorActionPreference = $previousErrorAction }
        return [pscustomobject]@{ ExitCode = $LASTEXITCODE; Output = ($output -join "`n") }
    } finally {
        Pop-Location
    }
}

function Test-LoopbackHost {
    param([string] $Address)
    $value = $Address.Trim().Trim('[', ']')
    return $value -ieq "localhost" -or $value -eq "127.0.0.1" -or $value -eq "::1"
}

function Get-ConnectionSetting {
    param([string] $Dsn, [string] $Key)
    if ($Dsn -match '^(postgres|postgresql)://') {
        $uri = [Uri]$Dsn
        if ($Key -eq "host") { return $uri.Host }
        if ($Key -eq "dbname") { return $uri.AbsolutePath.TrimStart('/') }
    }
    $pattern = '(?i)(?:^|\s){0}=(''[^'']*''|"[^"]*"|[^\s]+)' -f [regex]::Escape($Key)
    $match = [regex]::Match($Dsn, $pattern)
    if ($match.Success) { return $match.Groups[1].Value.Trim([char]39, [char]34) }
    return ""
}

function Test-LocalTestSettings {
    param([string] $DatabaseAdminDsn, [string] $RedisAddress, [int] $RedisDatabase)
    $issues = [System.Collections.Generic.List[string]]::new()
    if ([string]::IsNullOrWhiteSpace($DatabaseAdminDsn)) {
        $issues.Add("OMNICRAFT_TEST_ADMIN_DB_DSN is required for real database layers")
    } else {
        $dbHost = Get-ConnectionSetting -Dsn $DatabaseAdminDsn -Key "host"
        $dbName = Get-ConnectionSetting -Dsn $DatabaseAdminDsn -Key "dbname"
        if (-not (Test-LoopbackHost $dbHost)) { $issues.Add("PostgreSQL admin DSN host must be loopback") }
        if ($dbName -ne "postgres") { $issues.Add("PostgreSQL admin DSN must target postgres, never a development database") }
    }
    $redisParts = $RedisAddress -split ':'
    if ($redisParts.Count -ne 2 -or -not (Test-LoopbackHost $redisParts[0])) { $issues.Add("Redis address must be loopback host:port") }
    if ($RedisDatabase -le 0) { $issues.Add("Redis test database must be non-zero") }
    return [pscustomobject]@{ IsValid = $issues.Count -eq 0; Issues = @($issues) }
}

function Test-TcpPort {
    param([string] $Address, [int] $Port)
    $client = [Net.Sockets.TcpClient]::new()
    try {
        $task = $client.ConnectAsync($Address, $Port)
        return $task.Wait(1500) -and $client.Connected
    } catch { return $false } finally { $client.Dispose() }
}

function Test-LocalListener {
    param([int] $Port)
    return $null -ne (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -First 1)
}

function Invoke-Preflight {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)] [string] $RepoRoot,
        [string] $ComposeRoot = $RepoRoot,
        [Parameter(Mandatory = $true)] [string] $DatabaseAdminDsn,
        [Parameter(Mandatory = $true)] [string] $RedisAddress,
        [Parameter(Mandatory = $true)] [int] $RedisDatabase,
        [Parameter(Mandatory = $true)] [string] $GoCache,
        [scriptblock] $PortProbe = ${function:Test-TcpPort},
        [scriptblock] $ListenerProbe = ${function:Test-LocalListener}
    )
    $blockers = [System.Collections.Generic.List[string]]::new()
    $settings = Test-LocalTestSettings -DatabaseAdminDsn $DatabaseAdminDsn -RedisAddress $RedisAddress -RedisDatabase $RedisDatabase
    foreach ($issue in $settings.Issues) { $blockers.Add($issue) }
    $composePath = Join-Path $ComposeRoot "docker-compose.yml"
    if (-not (Test-Path -LiteralPath $composePath)) { $blockers.Add("docker-compose.yml is missing") }
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        $blockers.Add("Docker CLI is unavailable")
    } else {
        foreach ($args in @(@("--version"), @("compose", "version"))) {
            $probe = Invoke-LocalNative -File "docker" -Arguments $args -WorkingDirectory $ComposeRoot
            if ($probe.ExitCode -ne 0) { $blockers.Add("Docker Compose is unavailable") }
        }
        $services = Invoke-LocalNative -File "docker" -Arguments @("compose", "ps", "--format", "json") -WorkingDirectory $ComposeRoot
        if ($services.ExitCode -ne 0) {
            $blockers.Add("Docker Compose service status could not be read")
        } else {
            try {
                # Compose emits either one JSON array or newline-delimited JSON objects,
                # depending on the installed Compose release.
                try {
                    $parsed = @($services.Output | ConvertFrom-Json)
                } catch {
                    $parsed = @($services.Output -split "`r?`n" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | ForEach-Object { $_ | ConvertFrom-Json })
                }
                foreach ($name in @("postgres", "redis")) {
                    $service = @($parsed | Where-Object { $_.Service -eq $name } | Select-Object -First 1)
                    if ($service.Count -eq 0 -or $service[0].State -ne "running" -or $service[0].Health -ne "healthy") {
                        $blockers.Add("Compose service $name is not running and healthy")
                    }
                }
            } catch { $blockers.Add("Docker Compose returned invalid service status") }
        }
    }
    foreach ($port in @(5432, 6379)) {
        if (-not (& $PortProbe "127.0.0.1" $port)) { $blockers.Add("host port $port is unavailable") }
    }
    foreach ($port in @(3000, 8080)) {
        if (& $ListenerProbe $port) { $blockers.Add("test port $port is already owned; cross-stack runner refuses to reuse it") }
    }
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        $blockers.Add("Go toolchain is unavailable")
    } else {
        try {
            New-Item -ItemType Directory -Force -Path $GoCache | Out-Null
            $marker = Join-Path $GoCache ".omnicraft-write-probe"
            [IO.File]::WriteAllText($marker, "ok")
            Remove-Item -LiteralPath $marker -Force
            $oldCache = $env:GOCACHE
            $env:GOCACHE = $GoCache
            try {
                $goRoot = Invoke-LocalNative -File "go" -Arguments @("env", "GOROOT") -WorkingDirectory (Join-Path $RepoRoot "backend")
                $context = Invoke-LocalNative -File "go" -Arguments @("list", "context") -WorkingDirectory (Join-Path $RepoRoot "backend")
                if ($goRoot.ExitCode -ne 0 -or [string]::IsNullOrWhiteSpace($goRoot.Output) -or $context.ExitCode -ne 0) { $blockers.Add("Go GOROOT/context validation failed") }
            } finally { $env:GOCACHE = $oldCache }
        } catch { $blockers.Add("GOCACHE is not writable: $GoCache") }
    }
    if ($blockers.Count -gt 0) {
        return [pscustomobject]@{ Status = "BLOCKED"; ExitCode = 2; Detail = ($blockers -join "; "); Remediation = $script:RecoveryCommand }
    }
    return [pscustomobject]@{ Status = "PASS"; ExitCode = 0; Detail = "Docker, localhost ports, Go context, and isolated cache are ready"; Remediation = "" }
}

if ($MyInvocation.InvocationName -ne ".") {
    if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot) }
    if ([string]::IsNullOrWhiteSpace($ComposeRoot)) { $ComposeRoot = $RepoRoot }
    if ([string]::IsNullOrWhiteSpace($DatabaseAdminDsn)) { $DatabaseAdminDsn = "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=postgres sslmode=disable" }
    $result = Invoke-Preflight -RepoRoot $RepoRoot -ComposeRoot $ComposeRoot -DatabaseAdminDsn $DatabaseAdminDsn -RedisAddress $RedisAddress -RedisDatabase $RedisDatabase -GoCache $GoCache
    Write-Host "PREFLIGHT $($result.Status): $($result.Detail)"
    if ($result.Status -ne "PASS") { Write-Host "Remediation: $($result.Remediation)" }
    exit $result.ExitCode
}
