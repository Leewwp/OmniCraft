[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern("^(F|V|A|G|D|R)-\d{2}$")]
    [string]$TaskId,
    [string]$RepoRoot,
    [string]$IntegrationPath,
    [string]$TaskPath,
    [switch]$ConfirmMerge,
    [switch]$ReleaseReservationOnly
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

if ($ReleaseReservationOnly) {
    Remove-TaskReservations -TaskId $TaskId
    Write-Host "Released reservations for $TaskId. No merge was attempted."
    exit 0
}

$task = Get-BetaTask -TaskId $TaskId
$taskBranch = "codex/beta/$TaskId"

Assert-GitRepository -RepoRoot $IntegrationPath
Assert-GitRepository -RepoRoot $TaskPath
if ((Get-CurrentBranch -RepoRoot $IntegrationPath) -ne "codex/beta-integration") {
    throw "Integration worktree must be on codex/beta-integration: $IntegrationPath"
}
if ((Get-CurrentBranch -RepoRoot $TaskPath) -ne $taskBranch) {
    throw "Task worktree must be on $taskBranch`: $TaskPath"
}
Assert-CleanWorktree -RepoRoot $IntegrationPath -Purpose "task integration"
Assert-CleanWorktree -RepoRoot $TaskPath -Purpose "task integration"

$reviewDirectory = Join-Path $TaskPath ".task-logs\beta-reviews\$TaskId"
$specApproval = Join-Path $reviewDirectory "spec-approved.txt"
$qualityApproval = Join-Path $reviewDirectory "quality-approved.txt"
if (-not (Test-ApprovedMarker -Path $specApproval)) {
    throw "Missing spec approval marker: $specApproval"
}
if (-not (Test-ApprovedMarker -Path $qualityApproval)) {
    throw "Missing quality approval marker: $qualityApproval"
}
if (-not (Test-RoadmapTaskChecked -RepoRoot $TaskPath -TaskId $TaskId)) {
    throw "Task $TaskId is not checked in the task branch roadmap."
}

& git -c "core.excludesFile=" -C $TaskPath merge-base --is-ancestor "codex/beta-integration" $taskBranch
if ($LASTEXITCODE -ne 0) {
    throw "Task branch is behind codex/beta-integration. Rebase it, resolve conflicts and rerun task verification before integration."
}

if (-not $ConfirmMerge) {
    Write-Host "Preflight passed for $TaskId."
    Write-Host "No merge was attempted. Re-run with -ConfirmMerge to execute verification and git merge --ff-only."
    exit 0
}

Invoke-CheckedProcess -WorkingDirectory (Join-Path $TaskPath "backend") -FilePath "go" -Arguments @("test", "./...")
Invoke-CheckedProcess -WorkingDirectory (Join-Path $TaskPath "backend") -FilePath "go" -Arguments @("vet", "./...")
Invoke-CheckedProcess -WorkingDirectory (Join-Path $TaskPath "backend") -FilePath "go" -Arguments @("build", "./...")
Invoke-CheckedProcess -WorkingDirectory (Join-Path $TaskPath "frontend") -FilePath "npm.cmd" -Arguments @("run", "lint")
Invoke-CheckedProcess -WorkingDirectory (Join-Path $TaskPath "frontend") -FilePath "npm.cmd" -Arguments @("run", "build")
Invoke-CheckedProcess -WorkingDirectory $TaskPath -FilePath "docker" -Arguments @("compose", "config")

if ($task.ContainsKey("Desktop") -and $task.Desktop) {
    Invoke-CheckedProcess -WorkingDirectory (Join-Path $TaskPath "tauri-client") -FilePath "npm.cmd" -Arguments @("run", "build")
    Invoke-CheckedProcess -WorkingDirectory (Join-Path $TaskPath "tauri-client") -FilePath "cargo" -Arguments @(
        "test",
        "--manifest-path",
        "src-tauri/Cargo.toml"
    )
}

[void](Invoke-Git -RepoRoot $IntegrationPath -Arguments @("merge", "--ff-only", $taskBranch))
Remove-TaskReservations -TaskId $TaskId

Write-Host "Integrated $TaskId into codex/beta-integration with git merge --ff-only."
Write-Host "Task worktree was preserved for manual inspection and cleanup."
