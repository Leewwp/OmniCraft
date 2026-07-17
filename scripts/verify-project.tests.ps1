$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$verifier = Join-Path $PSScriptRoot "verify-project.ps1"
if (-not (Test-Path -LiteralPath $verifier)) {
    throw "verify-project.ps1 does not exist"
}

function Assert-Equal {
    param(
        [Parameter(Mandatory = $true)] $Actual,
        [Parameter(Mandatory = $true)] $Expected,
        [Parameter(Mandatory = $true)] [string] $Message
    )

    if ($Actual -ne $Expected) {
        throw "$Message`nActual: $Actual`nExpected: $Expected"
    }
}

function Assert-CommandLog {
    param(
        [Parameter(Mandatory = $true)] [string] $LogPath,
        [Parameter(Mandatory = $true)] [string[]] $Expected,
        [Parameter(Mandatory = $true)] [string] $Message
    )

    $actual = if (Test-Path -LiteralPath $LogPath) { @(Get-Content -LiteralPath $LogPath) } else { @() }
    Assert-Equal -Actual ($actual -join "`n") -Expected ($Expected -join "`n") -Message $Message
}

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("omnicraft-verify-tests-" + [guid]::NewGuid().ToString("N"))
$repoRoot = Join-Path $tempRoot "repo"
$toolsRoot = Join-Path $tempRoot "tools"
$logPath = Join-Path $tempRoot "commands.log"
$originalPath = $env:PATH
$originalLog = $env:OMNICRAFT_VERIFY_TEST_LOG
$originalFailure = $env:OMNICRAFT_VERIFY_TEST_FAIL

try {
    foreach ($path in @(
        $repoRoot,
        $toolsRoot,
        (Join-Path $repoRoot "backend"),
        (Join-Path $repoRoot "frontend"),
        (Join-Path $repoRoot "scripts"),
        (Join-Path $repoRoot "tools\doc-validator"),
        (Join-Path $repoRoot "tauri-client\src-tauri")
    )) {
        New-Item -ItemType Directory -Force -Path $path | Out-Null
    }

    $fakeCommand = @'
param(
    [Parameter(Mandatory = $true, Position = 0)] [string] $Tool,
    [Parameter(Position = 1, ValueFromRemainingArguments = $true)] [string[]] $RemainingArgs
)

$location = Split-Path -Leaf (Get-Location).Path
$line = "$location|$Tool"
if ($RemainingArgs.Count -gt 0) {
    $line += " " + ($RemainingArgs -join " ")
}
[System.IO.File]::AppendAllText($env:OMNICRAFT_VERIFY_TEST_LOG, $line + [Environment]::NewLine)
if ($env:OMNICRAFT_VERIFY_TEST_FAIL -and $line -eq $env:OMNICRAFT_VERIFY_TEST_FAIL) {
    exit 23
}
exit 0
'@
    Set-Content -LiteralPath (Join-Path $toolsRoot "fake-command.ps1") -Value $fakeCommand -Encoding UTF8

    $powershell = Join-Path $PSHOME "powershell.exe"
    foreach ($tool in @("go", "npm.cmd", "cargo")) {
        $wrapperName = if ($tool -eq "npm.cmd") { "npm.cmd" } else { "$tool.cmd" }
        $wrapper = @"
@echo off
"$powershell" -NoProfile -ExecutionPolicy Bypass -File "%~dp0fake-command.ps1" "$tool" %*
exit /b %ERRORLEVEL%
"@
        Set-Content -LiteralPath (Join-Path $toolsRoot $wrapperName) -Value $wrapper -Encoding ASCII
    }

    $env:PATH = $toolsRoot + [System.IO.Path]::PathSeparator + $originalPath
    $env:OMNICRAFT_VERIFY_TEST_LOG = $logPath
    Remove-Item Env:OMNICRAFT_VERIFY_TEST_FAIL -ErrorAction SilentlyContinue
	$fixtureVerifier = Join-Path $repoRoot "scripts\verify-project.ps1"
	Copy-Item -LiteralPath $verifier -Destination $fixtureVerifier

    $defaultCommands = @(
        "backend|go test ./...",
        "backend|go vet ./...",
        "backend|go build ./...",
        "frontend|npm.cmd run test:unit",
        "frontend|npm.cmd run lint",
        "frontend|npm.cmd run build",
        "doc-validator|go test ./...",
        "doc-validator|go run . --check --profile release"
    )

    Remove-Item -LiteralPath $logPath -ErrorAction SilentlyContinue
    & $verifier -RepoRoot $repoRoot | Out-Null
    Assert-CommandLog -LogPath $logPath -Expected $defaultCommands -Message "default tier command contract changed"

    Remove-Item -LiteralPath $logPath -ErrorAction SilentlyContinue
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & $powershell -NoProfile -ExecutionPolicy Bypass -File $fixtureVerifier *> $null
        $defaultRootExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    Assert-Equal -Actual $defaultRootExitCode -Expected 0 -Message "standalone verifier must resolve RepoRoot from its own scripts directory when omitted"
    Assert-CommandLog -LogPath $logPath -Expected $defaultCommands -Message "default RepoRoot command contract changed"

    Remove-Item -LiteralPath $logPath -ErrorAction SilentlyContinue
    & $verifier -RepoRoot $repoRoot -Full | Out-Null
    Assert-CommandLog -LogPath $logPath -Expected ($defaultCommands + "frontend|npm.cmd run test:contracts") -Message "full tier must add mocked browser contracts"

    Remove-Item -LiteralPath $logPath -ErrorAction SilentlyContinue
    & $verifier -RepoRoot $repoRoot -Release | Out-Null
    Assert-CommandLog -LogPath $logPath -Expected ($defaultCommands + "frontend|npm.cmd run test:e2e") -Message "release tier must add the complete Playwright suite"

    Remove-Item -LiteralPath $logPath -ErrorAction SilentlyContinue
    & $verifier -RepoRoot $repoRoot -Tauri | Out-Null
    Assert-CommandLog -LogPath $logPath -Expected ($defaultCommands + @(
        "tauri-client|npm.cmd run build",
        "tauri-client|cargo test --manifest-path src-tauri/Cargo.toml"
    )) -Message "Tauri option must add frontend and Rust gates"

    Remove-Item -LiteralPath $logPath -ErrorAction SilentlyContinue
    $env:OMNICRAFT_VERIFY_TEST_FAIL = "backend|go vet ./..."
    $beforeFailure = (Get-Location).Path
    $caught = $null
    try {
        & $verifier -RepoRoot $repoRoot *> $null
    } catch {
        $caught = $_
    }
    if ($null -eq $caught) {
        throw "verifier must throw when an external command fails"
    }
    Assert-Equal -Actual (Get-Location).Path -Expected $beforeFailure -Message "verifier must restore the caller location after failure"
    Assert-CommandLog -LogPath $logPath -Expected @(
        "backend|go test ./...",
        "backend|go vet ./..."
    ) -Message "verifier must stop immediately after the first failed command"

    Remove-Item -LiteralPath $logPath -ErrorAction SilentlyContinue
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & $powershell -NoProfile -ExecutionPolicy Bypass -File $verifier -RepoRoot $repoRoot *> $null
        $standaloneExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    if ($standaloneExitCode -eq 0) {
        throw "standalone verifier process must return a non-zero exit code on child-command failure"
    }

    Write-Host "verify-project contract tests passed"
} finally {
    $env:PATH = $originalPath
    if ($null -eq $originalLog) {
        Remove-Item Env:OMNICRAFT_VERIFY_TEST_LOG -ErrorAction SilentlyContinue
    } else {
        $env:OMNICRAFT_VERIFY_TEST_LOG = $originalLog
    }
    if ($null -eq $originalFailure) {
        Remove-Item Env:OMNICRAFT_VERIFY_TEST_FAIL -ErrorAction SilentlyContinue
    } else {
        $env:OMNICRAFT_VERIFY_TEST_FAIL = $originalFailure
    }
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
