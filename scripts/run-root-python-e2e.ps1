param(
    [ValidateSet("all", "search-download", "admin-journey")]
    [string]$Suite = "all",
    [string]$Python = "python"
)

$ErrorActionPreference = "Stop"

function Invoke-Suite {
    param(
        [string]$Label,
        [string]$ScriptPath
    )

    Write-Host ""
    Write-Host "=== Running $Label ==="
    & $Python $ScriptPath
    if ($LASTEXITCODE -ne 0) {
        throw "$Label failed with exit code $LASTEXITCODE"
    }
}

$root = Split-Path -Parent $PSScriptRoot
$searchScript = Join-Path $root "e2e\test_search_download.py"
$adminScript = Join-Path $root "e2e\test_admin_journey.py"

switch ($Suite) {
    "search-download" {
        Invoke-Suite -Label "search/download manual root E2E" -ScriptPath $searchScript
    }
    "admin-journey" {
        Invoke-Suite -Label "admin journey manual root E2E" -ScriptPath $adminScript
    }
    default {
        Invoke-Suite -Label "search/download manual root E2E" -ScriptPath $searchScript
        Invoke-Suite -Label "admin journey manual root E2E" -ScriptPath $adminScript
    }
}
