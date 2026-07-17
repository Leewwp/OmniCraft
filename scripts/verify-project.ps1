[CmdletBinding()]
param(
    [string] $RepoRoot,
    [switch] $Full,
    [switch] $Release,
    [switch] $Tauri
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if (-not $RepoRoot) {
    $RepoRoot = Split-Path -Parent $PSScriptRoot
}

if ($Full -and $Release) {
    throw "-Full and -Release are mutually exclusive verification tiers"
}

function Invoke-ExternalCommand {
    param(
        [Parameter(Mandatory = $true)] [string] $Command,
        [string[]] $Arguments = @()
    )

    Write-Host ("> " + $Command + " " + ($Arguments -join " "))
    & $Command @Arguments
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) {
        throw "Command failed with exit code $exitCode`: $Command $($Arguments -join ' ')"
    }
}

function Invoke-InLocation {
    param(
        [Parameter(Mandatory = $true)] [string] $Path,
        [Parameter(Mandatory = $true)] [scriptblock] $Action
    )

    Push-Location -LiteralPath $Path
    try {
        & $Action
    } finally {
        Pop-Location
    }
}

$resolvedRepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$npmCommand = if ([System.Environment]::OSVersion.Platform -eq [System.PlatformID]::Win32NT) { "npm.cmd" } else { "npm" }
$tier = if ($Release) { "release" } elseif ($Full) { "full" } else { "default" }

Write-Host "OmniCraft verification tier: $tier"

Invoke-InLocation -Path (Join-Path $resolvedRepoRoot "backend") -Action {
    Invoke-ExternalCommand -Command "go" -Arguments @("test", "./...")
    Invoke-ExternalCommand -Command "go" -Arguments @("vet", "./...")
    Invoke-ExternalCommand -Command "go" -Arguments @("build", "./...")
}

Invoke-InLocation -Path (Join-Path $resolvedRepoRoot "frontend") -Action {
    Invoke-ExternalCommand -Command $npmCommand -Arguments @("run", "test:unit")
    Invoke-ExternalCommand -Command $npmCommand -Arguments @("run", "lint")
    Invoke-ExternalCommand -Command $npmCommand -Arguments @("run", "build")
}

Invoke-InLocation -Path (Join-Path $resolvedRepoRoot "tools\doc-validator") -Action {
    Invoke-ExternalCommand -Command "go" -Arguments @("test", "./...")
    Invoke-ExternalCommand -Command "go" -Arguments @("run", ".", "--check", "--profile", "release")
}

if ($Full) {
    Invoke-InLocation -Path (Join-Path $resolvedRepoRoot "frontend") -Action {
        Invoke-ExternalCommand -Command $npmCommand -Arguments @("run", "test:contracts")
    }
}

if ($Release) {
    Invoke-InLocation -Path (Join-Path $resolvedRepoRoot "frontend") -Action {
        Invoke-ExternalCommand -Command $npmCommand -Arguments @("run", "test:e2e")
    }
}

if ($Tauri) {
    Invoke-InLocation -Path (Join-Path $resolvedRepoRoot "tauri-client") -Action {
        Invoke-ExternalCommand -Command $npmCommand -Arguments @("run", "build")
        Invoke-ExternalCommand -Command "cargo" -Arguments @("test", "--manifest-path", "src-tauri/Cargo.toml")
    }
}

Write-Host "OmniCraft $tier verification passed"
