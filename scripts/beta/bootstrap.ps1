[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [string]$RepoRoot,
    [string]$IntegrationPath,
    [string]$BaseRef = "HEAD",
    [switch]$InstallDependencies,
    [switch]$InstallDesktopDependencies
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

if ($InstallDesktopDependencies -and -not $InstallDependencies) {
    throw "-InstallDesktopDependencies requires -InstallDependencies."
}

Assert-CommandAvailable -Name "git"
Assert-GitRepository -RepoRoot $RepoRoot
Assert-CleanWorktree -RepoRoot $RepoRoot -Purpose "Beta integration bootstrap"

$integrationBranch = "codex/beta-integration"
if (Test-Path -LiteralPath $IntegrationPath) {
    Assert-GitRepository -RepoRoot $IntegrationPath
    if ((Get-CurrentBranch -RepoRoot $IntegrationPath) -ne $integrationBranch) {
        throw "Existing integration path is not on $integrationBranch`: $IntegrationPath"
    }
}
elseif ($PSCmdlet.ShouldProcess($IntegrationPath, "Create Beta integration worktree")) {
    if (Test-LocalBranchExists -RepoRoot $RepoRoot -Branch $integrationBranch) {
        [void](Invoke-Git -RepoRoot $RepoRoot -Arguments @("worktree", "add", $IntegrationPath, $integrationBranch))
    }
    else {
        [void](Invoke-Git -RepoRoot $RepoRoot -Arguments @(
            "worktree",
            "add",
            $IntegrationPath,
            "-b",
            $integrationBranch,
            $BaseRef
        ))
    }
}

if (-not (Test-Path -LiteralPath $IntegrationPath)) {
    Write-Host "Dry run complete. Integration worktree would be created at $IntegrationPath"
    exit 0
}

Assert-CleanWorktree -RepoRoot $IntegrationPath -Purpose "Beta integration bootstrap"

foreach ($command in @("go", "node", "npm.cmd", "docker")) {
    Assert-CommandAvailable -Name $command
}

Invoke-CheckedProcess -WorkingDirectory $IntegrationPath -FilePath "docker" -Arguments @("compose", "config")

if ($InstallDependencies) {
    Install-WorkspaceDependencies -WorktreePath $IntegrationPath -IncludeDesktop:$InstallDesktopDependencies
    Assert-CleanWorktree -RepoRoot $IntegrationPath -Purpose "dependency installation verification"
}

Write-Host "Beta integration worktree is ready: $IntegrationPath"
Write-Host "Dependencies were installed: $($InstallDependencies.IsPresent)"
Write-Host "Desktop dependencies were installed: $($InstallDesktopDependencies.IsPresent)"
