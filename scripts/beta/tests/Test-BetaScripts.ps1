$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$betaRoot = Split-Path -Parent $PSScriptRoot
$repoRoot = Split-Path -Parent (Split-Path -Parent $betaRoot)

function Assert-True {
    param(
        [bool]$Condition,
        [string]$Message
    )

    if (-not $Condition) {
        throw "ASSERTION FAILED: $Message"
    }
}

function Assert-Contains {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Message
    )

    Assert-True ($Text -match $Pattern) $Message
}

function Assert-NotContains {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Message
    )

    Assert-True ($Text -notmatch $Pattern) $Message
}

$requiredFiles = @(
    "Common.ps1",
    "beta-tasks.psd1",
    "bootstrap.ps1",
    "run-task.ps1",
    "integrate.ps1",
    "README.md"
)

foreach ($relativePath in $requiredFiles) {
    $fullPath = Join-Path $betaRoot $relativePath
    Assert-True (Test-Path -LiteralPath $fullPath) "Missing scripts/beta/$relativePath"
}

$powerShellScripts = Get-ChildItem -LiteralPath $betaRoot -Filter "*.ps1" -File
foreach ($script in $powerShellScripts) {
    $tokens = $null
    $errors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile(
        $script.FullName,
        [ref]$tokens,
        [ref]$errors
    )
    Assert-True ($errors.Count -eq 0) "$($script.Name) has PowerShell parse errors"
}

$allScriptText = ($powerShellScripts | ForEach-Object {
    Get-Content -Raw -LiteralPath $_.FullName
}) -join "`n"

# DESTRUCTIVE-GUARD registry: structured data array for discoverable, extensible safety checks.
# Each entry defines a regex pattern that must NOT appear in any beta script,
# along with a category label for static-lint tooling and a human-readable message.
$destructiveGuards = @(
    @{
        Pattern  = "(?m)^\s*(?:&\s*)?git\s+add\s+\.\s*$"
        Category = "git-safety"
        Message  = "Scripts must not stage the entire worktree"
    },
    @{
        Pattern  = "reset\s+--hard"
        Category = "branch-protection"
        Message  = "Scripts must not hard reset branches"
    },
    @{
        Pattern  = "push\s+(-f|--force(?:-with-lease)?)"
        Category = "force-push"
        Message  = "Scripts must not force push"
    },
    @{
        Pattern  = "dangerously-skip-permissions"
        Category = "permission-bypass"
        Message  = "Scripts must not bypass Claude permissions"
    }
)

foreach ($guard in $destructiveGuards) {
    # DESTRUCTIVE-GUARD: <$($guard.Category)>
    Assert-NotContains $allScriptText $guard.Pattern "[$($guard.Category)] $($guard.Message)"
}

# --- Executable path coverage: Common.ps1 exported functions ---
$commonPath = Join-Path $betaRoot "Common.ps1"
$commonTokens = $null
$commonParseErrors = $null
[void][System.Management.Automation.Language.Parser]::ParseFile(
    $commonPath,
    [ref]$commonTokens,
    [ref]$commonParseErrors
)
Assert-True ($commonParseErrors.Count -eq 0) "Common.ps1 has PowerShell parse errors"

$commonAst = [System.Management.Automation.Language.Parser]::ParseFile(
    $commonPath,
    [ref]$null,
    [ref]$null
)
$exportedFunctions = @(
    "Resolve-AbsolutePath",
    "Get-DefaultRepoRoot",
    "Get-DefaultIntegrationPath",
    "Get-DefaultTaskPath",
    "Get-OrchestrationRoot",
    "Invoke-Git",
    "Invoke-CheckedProcess",
    "Assert-CommandAvailable",
    "Assert-GitRepository",
    "Get-GitStatusLines",
    "Assert-CleanWorktree",
    "Get-CurrentBranch",
    "Test-LocalBranchExists",
    "Get-BetaTaskCatalog",
    "Get-BetaTask",
    "Get-RoadmapPath",
    "Test-RoadmapTaskChecked",
    "Assert-TaskDependenciesIntegrated",
    "Get-TaskPlanSection",
    "Get-ReservationDirectory",
    "Get-SafeLockName",
    "New-TaskReservations",
    "Remove-TaskReservations",
    "Install-WorkspaceDependencies",
    "Test-ApprovedMarker"
)
$definedNames = @($commonAst.EndBlock.Statements |
    Where-Object { $_ -is [System.Management.Automation.Language.FunctionDefinitionAst] } |
    ForEach-Object { $_.Name })
foreach ($fn in $exportedFunctions) {
    Assert-True ($definedNames -contains $fn) "Common.ps1 must export function: $fn"
}

# --- Executable path coverage: beta-tasks.psd1 data integrity ---
$catalogPath = Join-Path $betaRoot "beta-tasks.psd1"
$catalog = Import-PowerShellDataFile -LiteralPath $catalogPath
Assert-True ($catalog.Count -gt 0) "beta-tasks.psd1 must not be empty"

$expectedTaskIdPrefixes = @("F-", "V-", "A-", "G-", "D-", "R-")
$catalogKeys = @($catalog.Keys)
foreach ($prefix in $expectedTaskIdPrefixes) {
    Assert-True (@($catalogKeys | Where-Object { $_ -like "$prefix*" }).Count -gt 0) "beta-tasks.psd1 must contain tasks with prefix: $prefix"
}

$requiredTaskFields = @("Plan", "DependsOn", "Locks")
foreach ($taskId in $catalogKeys) {
    $taskEntry = $catalog[$taskId]
    foreach ($field in $requiredTaskFields) {
        Assert-True ($taskEntry.ContainsKey($field)) "beta-tasks.psd1 task $taskId must have field: $field"
    }
}

$bootstrap = Get-Content -Raw -LiteralPath (Join-Path $betaRoot "bootstrap.ps1")
$runTask = Get-Content -Raw -LiteralPath (Join-Path $betaRoot "run-task.ps1")
$integrate = Get-Content -Raw -LiteralPath (Join-Path $betaRoot "integrate.ps1")
$common = Get-Content -Raw -LiteralPath (Join-Path $betaRoot "Common.ps1")

Assert-Contains $bootstrap "Assert-CleanWorktree" "bootstrap.ps1 must stop on dirty worktrees"
Assert-Contains $bootstrap "InstallDependencies" "bootstrap.ps1 must make dependency installation opt-in"
Assert-Contains $bootstrap "InstallDesktopDependencies" "bootstrap.ps1 must make desktop dependency installation separately opt-in"
Assert-Contains $common "core\.excludesFile=" "Common.ps1 must isolate git from unreadable global ignore files"
Assert-Contains $common "IncludeDesktop" "Common.ps1 must keep Tauri and Rust dependency installation optional"
Assert-Contains $runTask "LaunchClaude" "run-task.ps1 must make Claude launch explicit"
Assert-Contains $runTask "AllowParallel" "run-task.ps1 must default to serial execution"
Assert-Contains $runTask "acceptEdits" "run-task.ps1 must use Claude acceptEdits mode"
Assert-Contains $integrate "ConfirmMerge" "integrate.ps1 must require explicit merge confirmation"
Assert-Contains $integrate "spec-approved\.txt" "integrate.ps1 must require spec-review evidence"
Assert-Contains $integrate "quality-approved\.txt" "integrate.ps1 must require quality-review evidence"
Assert-Contains $integrate "merge.+--ff-only" "integrate.ps1 must use fast-forward-only merge"
Assert-Contains $integrate "go.+test.+\./\.\.\." "integrate.ps1 must run backend tests"
Assert-Contains $integrate "npm.+run.+lint" "integrate.ps1 must run frontend lint"
Assert-Contains $integrate "npm.+run.+build" "integrate.ps1 must run frontend build"

# $catalog already imported above for executable-path-coverage checks
$roadmap = Get-Content -Raw -Encoding utf8 -LiteralPath (
    Join-Path $repoRoot "docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md"
)
$roadmapTaskIds = [regex]::Matches(
    $roadmap,
    "(?m)^\|\s*((?:F|V|A|G|D|R)-\d{2})\s*\|"
) | ForEach-Object { $_.Groups[1].Value } | Sort-Object -Unique
$catalogTaskIds = @($catalog.Keys) | Sort-Object

Assert-True (
    @(Compare-Object $roadmapTaskIds $catalogTaskIds).Count -eq 0
) "beta-tasks.psd1 must list every roadmap task exactly once"

. (Join-Path $betaRoot "Common.ps1")
$tempBase = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
$tempRoot = Join-Path $tempBase "omnicraft-beta-script-tests-$([guid]::NewGuid())"
Assert-True (
    [System.IO.Path]::GetFullPath($tempRoot).StartsWith($tempBase, [System.StringComparison]::OrdinalIgnoreCase)
) "Temporary test directory must stay inside the OS temp directory"

try {
    $tempPlans = Join-Path $tempRoot "docs\superpowers\plans"
    New-Item -ItemType Directory -Force -Path $tempPlans | Out-Null
    Set-Content -Encoding utf8 -LiteralPath (
        Join-Path $tempPlans "2026-05-30-omnicraft-dual-track-beta-roadmap.md"
    ) -Value @"
| F-01 | completed fixture | foundation | - | Shared | ``[x]`` |
| F-02 | pending fixture | foundation | F-01 | Shared | ``[ ]`` |
"@

    Assert-True (
        Test-RoadmapTaskChecked -RepoRoot $tempRoot -TaskId "F-01"
    ) "Roadmap parser must recognize checked Markdown task cells"
    Assert-True (
        -not (Test-RoadmapTaskChecked -RepoRoot $tempRoot -TaskId "F-02")
    ) "Roadmap parser must reject unchecked Markdown task cells"
}
finally {
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -Recurse -Force -LiteralPath $tempRoot
    }
}

Write-Host "PASS: scripts/beta safety checks"
