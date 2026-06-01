[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern("^(F|V|A|G|D|R)-\d{2}$")]
    [string]$TaskId,
    [string]$RepoRoot,
    [string]$IntegrationPath,
    [string]$TaskPath,
    [string]$Model,
    [switch]$InstallDependencies,
    [switch]$LaunchClaude,
    [switch]$AllowParallel,
    [switch]$Resume
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot "Common.ps1")

if (-not $RepoRoot) {
    $RepoRoot = Get-DefaultRepoRoot
}
$RepoRoot = Resolve-AbsolutePath $RepoRoot
if (-not $IntegrationPath) {
    $IntegrationPath = Get-DefaultIntegrationPath -RepoRoot $RepoRoot
}
$IntegrationPath = Resolve-AbsolutePath $IntegrationPath
if (-not $TaskPath) {
    $TaskPath = Get-DefaultTaskPath -RepoRoot $RepoRoot -TaskId $TaskId
}
$TaskPath = Resolve-AbsolutePath $TaskPath

$task = Get-BetaTask -TaskId $TaskId
$taskBranch = "codex/beta/$TaskId"

Assert-GitRepository -RepoRoot $RepoRoot
Assert-GitRepository -RepoRoot $IntegrationPath
if ((Get-CurrentBranch -RepoRoot $IntegrationPath) -ne "codex/beta-integration") {
    throw "Integration worktree must be on codex/beta-integration: $IntegrationPath"
}
Assert-CleanWorktree -RepoRoot $IntegrationPath -Purpose "task preparation"
Assert-TaskDependenciesIntegrated -IntegrationPath $IntegrationPath -TaskId $TaskId

if (-not (Test-Path -LiteralPath $TaskPath)) {
    if (Test-LocalBranchExists -RepoRoot $RepoRoot -Branch $taskBranch) {
        [void](Invoke-Git -RepoRoot $RepoRoot -Arguments @("worktree", "add", $TaskPath, $taskBranch))
    }
    else {
        [void](Invoke-Git -RepoRoot $RepoRoot -Arguments @(
            "worktree",
            "add",
            $TaskPath,
            "-b",
            $taskBranch,
            "codex/beta-integration"
        ))
    }
}

Assert-GitRepository -RepoRoot $TaskPath
if ((Get-CurrentBranch -RepoRoot $TaskPath) -ne $taskBranch) {
    throw "Task worktree must be on $taskBranch`: $TaskPath"
}
if (-not $Resume) {
    Assert-CleanWorktree -RepoRoot $TaskPath -Purpose "task preparation"
}

$reservations = @(New-TaskReservations -TaskId $TaskId -TaskPath $TaskPath -AllowParallel:$AllowParallel)

if ($InstallDependencies) {
    $includeDesktopDependencies = $task.ContainsKey("Desktop") -and $task.Desktop
    Install-WorkspaceDependencies -WorktreePath $TaskPath -IncludeDesktop:$includeDesktopDependencies
    if (-not $Resume) {
        Assert-CleanWorktree -RepoRoot $TaskPath -Purpose "dependency installation verification"
    }
}

$promptDirectory = Join-Path $TaskPath ".task-logs\beta-prompts"
$runDirectory = Join-Path $TaskPath ".task-logs\beta-runs"
$reviewDirectory = Join-Path $TaskPath ".task-logs\beta-reviews\$TaskId"
New-Item -ItemType Directory -Force -Path $promptDirectory, $runDirectory, $reviewDirectory | Out-Null

$taskSection = Get-TaskPlanSection -RepoRoot $TaskPath -TaskId $TaskId
$promptPath = Join-Path $promptDirectory "$TaskId.md"
$prompt = @"
# OmniCraft Beta Task Controller

Task: $TaskId
Branch and worktree: $taskBranch at $TaskPath
Reserved shared surfaces: $($reservations -join ", ")
Plan file: $($task.Plan)

You are the controller for exactly one Beta roadmap task. Read AGENTS.md, the roadmap, the implementation notes and the full task text below before doing anything.

Required workflow:
1. Use superpowers:subagent-driven-development.
2. Dispatch one fresh implementer subagent for this task. The implementer must follow TDD: write a failing test, run it and confirm the expected failure before production code.
3. Do not silently edit outside the task's planned files or reserved surfaces. Report any required contract expansion as DONE_WITH_CONCERNS and stop before broadening scope.
4. Run the task-local checks and the repository-wide gates from AGENTS.md. UI work requires Playwright evidence. Desktop work requires Rust gates.
5. Commit exactly one task commit with exact-path staging. Never use git add . and never push main.
6. Dispatch a fresh spec-compliance reviewer. Fix and re-review until approved.
7. Dispatch a fresh code-quality reviewer only after spec compliance is approved. Fix and re-review until approved.
8. Read and resolve DONE_WITH_CONCERNS. Correctness, security, scope or contract concerns block approval.
9. Only after both reviews approve, write the literal text APPROVED into:
   - $reviewDirectory\spec-approved.txt
   - $reviewDirectory\quality-approved.txt
10. Report one status: DONE, DONE_WITH_CONCERNS, NEEDS_CONTEXT or BLOCKED.

Do not merge the task branch. Integration is a separate maintainer-confirmed operation.

## Full Task Text

$taskSection
"@
Set-Content -LiteralPath $promptPath -Value $prompt -Encoding utf8

Write-Host "Prepared task $TaskId at $TaskPath"
Write-Host "Reserved surfaces: $($reservations -join ', ')"
Write-Host "Prompt: $promptPath"

if (-not $LaunchClaude) {
    Write-Host "Claude was not launched. Re-run with -LaunchClaude after reviewing the prompt."
    exit 0
}

Assert-CommandAvailable -Name "claude"
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$logPath = Join-Path $runDirectory "$TaskId-$timestamp.log"
$claudeArgs = @(
    "-p",
    (Get-Content -Raw -LiteralPath $promptPath),
    "--permission-mode",
    "acceptEdits"
)
if ($Model) {
    $claudeArgs += @("--model", $Model)
}

& claude @claudeArgs 2>&1 | Tee-Object -FilePath $logPath
if ($LASTEXITCODE -ne 0) {
    throw "Claude exited with code $LASTEXITCODE. Reservation locks remain in place for investigation. Log: $logPath"
}

Write-Host "Claude session ended. Review its status and use integrate.ps1 only after approval markers exist."
