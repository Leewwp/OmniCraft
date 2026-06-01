$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Resolve-AbsolutePath {
    param([Parameter(Mandatory = $true)][string]$Path)

    return [System.IO.Path]::GetFullPath($Path)
}

function Get-DefaultRepoRoot {
    return Resolve-AbsolutePath (Join-Path $PSScriptRoot "..\..")
}

function Get-DefaultIntegrationPath {
    param([Parameter(Mandatory = $true)][string]$RepoRoot)

    $repo = Get-Item -LiteralPath $RepoRoot
    return Join-Path $repo.Parent.FullName "$($repo.Name)-beta-integration"
}

function Get-DefaultTaskPath {
    param(
        [Parameter(Mandatory = $true)][string]$RepoRoot,
        [Parameter(Mandatory = $true)][string]$TaskId
    )

    $repo = Get-Item -LiteralPath $RepoRoot
    return Join-Path $repo.Parent.FullName "$($repo.Name)-$TaskId"
}

function Get-OrchestrationRoot {
    if ($env:OMNICRAFT_ORCHESTRATION_ROOT) {
        return Resolve-AbsolutePath $env:OMNICRAFT_ORCHESTRATION_ROOT
    }

    return "C:\tmp\omnicraft-orchestration"
}

function Invoke-Git {
    param(
        [Parameter(Mandatory = $true)][string]$RepoRoot,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    $output = @(& git -c "core.excludesFile=" -C $RepoRoot @Arguments 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "git $($Arguments -join ' ') failed in $RepoRoot`n$($output -join [Environment]::NewLine)"
    }

    return $output
}

function Invoke-CheckedProcess {
    param(
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    Push-Location $WorkingDirectory
    try {
        & $FilePath @Arguments
        if ($LASTEXITCODE -ne 0) {
            throw "$FilePath $($Arguments -join ' ') failed in $WorkingDirectory"
        }
    }
    finally {
        Pop-Location
    }
}

function Assert-CommandAvailable {
    param([Parameter(Mandatory = $true)][string]$Name)

    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command is not available: $Name"
    }
}

function Assert-GitRepository {
    param([Parameter(Mandatory = $true)][string]$RepoRoot)

    if (-not (Test-Path -LiteralPath $RepoRoot)) {
        throw "Repository path does not exist: $RepoRoot"
    }

    [void](Invoke-Git -RepoRoot $RepoRoot -Arguments @("rev-parse", "--show-toplevel"))
}

function Get-GitStatusLines {
    param([Parameter(Mandatory = $true)][string]$RepoRoot)

    return @(Invoke-Git -RepoRoot $RepoRoot -Arguments @("status", "--porcelain"))
}

function Assert-CleanWorktree {
    param(
        [Parameter(Mandatory = $true)][string]$RepoRoot,
        [string]$Purpose = "operation"
    )

    $dirty = @(Get-GitStatusLines -RepoRoot $RepoRoot)
    if ($dirty.Count -gt 0) {
        throw "Refusing $Purpose because the worktree is dirty: $RepoRoot`n$($dirty -join [Environment]::NewLine)"
    }
}

function Get-CurrentBranch {
    param([Parameter(Mandatory = $true)][string]$RepoRoot)

    return (Invoke-Git -RepoRoot $RepoRoot -Arguments @("branch", "--show-current"))[0].Trim()
}

function Test-LocalBranchExists {
    param(
        [Parameter(Mandatory = $true)][string]$RepoRoot,
        [Parameter(Mandatory = $true)][string]$Branch
    )

    & git -c "core.excludesFile=" -C $RepoRoot show-ref --verify --quiet "refs/heads/$Branch"
    return $LASTEXITCODE -eq 0
}

function Get-BetaTaskCatalog {
    return Import-PowerShellDataFile -LiteralPath (Join-Path $PSScriptRoot "beta-tasks.psd1")
}

function Get-BetaTask {
    param([Parameter(Mandatory = $true)][string]$TaskId)

    $catalog = Get-BetaTaskCatalog
    if (-not $catalog.ContainsKey($TaskId)) {
        throw "Unknown Beta task ID: $TaskId"
    }

    return $catalog[$TaskId]
}

function Get-RoadmapPath {
    param([Parameter(Mandatory = $true)][string]$RepoRoot)

    return Join-Path $RepoRoot "docs\superpowers\plans\2026-05-30-omnicraft-dual-track-beta-roadmap.md"
}

function Test-RoadmapTaskChecked {
    param(
        [Parameter(Mandatory = $true)][string]$RepoRoot,
        [Parameter(Mandatory = $true)][string]$TaskId
    )

    $roadmap = Get-Content -Raw -Encoding utf8 -LiteralPath (Get-RoadmapPath -RepoRoot $RepoRoot)
    $escapedTaskId = [regex]::Escape($TaskId)
    $pattern = '(?m)^\|\s*' + $escapedTaskId + '\s*\|.*\|\s*`?\[x\]`?\s*\|$'
    return $roadmap -match $pattern
}

function Assert-TaskDependenciesIntegrated {
    param(
        [Parameter(Mandatory = $true)][string]$IntegrationPath,
        [Parameter(Mandatory = $true)][string]$TaskId
    )

    $task = Get-BetaTask -TaskId $TaskId
    foreach ($dependency in @($task.DependsOn)) {
        if (-not (Test-RoadmapTaskChecked -RepoRoot $IntegrationPath -TaskId $dependency)) {
            throw "Task $TaskId is not ready: dependency $dependency is not checked in the integration roadmap."
        }
    }
}

function Get-TaskPlanSection {
    param(
        [Parameter(Mandatory = $true)][string]$RepoRoot,
        [Parameter(Mandatory = $true)][string]$TaskId
    )

    $task = Get-BetaTask -TaskId $TaskId
    $planPath = Join-Path $RepoRoot $task.Plan
    $plan = Get-Content -Raw -Encoding utf8 -LiteralPath $planPath
    $escapedTaskId = [regex]::Escape($TaskId)
    $match = [regex]::Match($plan, "(?ms)^## Task $escapedTaskId`:.*?(?=^## Task |\z)")
    if (-not $match.Success) {
        throw "Unable to locate Task $TaskId in $planPath"
    }

    return $match.Value.Trim()
}

function Get-ReservationDirectory {
    $directory = Join-Path (Get-OrchestrationRoot) "locks"
    New-Item -ItemType Directory -Force -Path $directory | Out-Null
    return $directory
}

function Get-SafeLockName {
    param([Parameter(Mandatory = $true)][string]$Name)

    return ($Name -replace "[^A-Za-z0-9_.-]", "_")
}

function New-TaskReservations {
    param(
        [Parameter(Mandatory = $true)][string]$TaskId,
        [Parameter(Mandatory = $true)][string]$TaskPath,
        [switch]$AllowParallel
    )

    $task = Get-BetaTask -TaskId $TaskId
    $locks = @($task.Locks)
    if (-not $AllowParallel) {
        $locks = @("global-serial") + $locks
    }
    $locks = @($locks | Sort-Object -Unique)

    $directory = Get-ReservationDirectory
    $created = @()
    try {
        foreach ($lock in $locks) {
            $lockPath = Join-Path $directory "$(Get-SafeLockName -Name $lock).lock.json"
            if (Test-Path -LiteralPath $lockPath) {
                $existing = Get-Content -Raw -LiteralPath $lockPath | ConvertFrom-Json
                if ($existing.task_id -eq $TaskId) {
                    continue
                }
                throw "Lock '$lock' is already reserved by task $($existing.task_id)."
            }

            $payload = @{
                task_id     = $TaskId
                task_path   = $TaskPath
                lock        = $lock
                acquired_at = (Get-Date).ToString("o")
            } | ConvertTo-Json
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($payload)
            $stream = New-Object System.IO.FileStream(
                $lockPath,
                [System.IO.FileMode]::CreateNew,
                [System.IO.FileAccess]::Write,
                [System.IO.FileShare]::None
            )
            try {
                $stream.Write($bytes, 0, $bytes.Length)
            }
            finally {
                $stream.Dispose()
            }
            $created += $lockPath
        }
    }
    catch {
        foreach ($lockPath in $created) {
            Remove-Item -LiteralPath $lockPath -Force
        }
        throw
    }

    return $locks
}

function Remove-TaskReservations {
    param([Parameter(Mandatory = $true)][string]$TaskId)

    $directory = Get-ReservationDirectory
    Get-ChildItem -LiteralPath $directory -Filter "*.lock.json" -File | ForEach-Object {
        $reservation = Get-Content -Raw -LiteralPath $_.FullName | ConvertFrom-Json
        if ($reservation.task_id -eq $TaskId) {
            Remove-Item -LiteralPath $_.FullName -Force
        }
    }
}

function Install-WorkspaceDependencies {
    param(
        [Parameter(Mandatory = $true)][string]$WorktreePath,
        [switch]$IncludeDesktop
    )

    Invoke-CheckedProcess -WorkingDirectory (Join-Path $WorktreePath "backend") -FilePath "go" -Arguments @("mod", "download")
    Invoke-CheckedProcess -WorkingDirectory (Join-Path $WorktreePath "frontend") -FilePath "npm.cmd" -Arguments @("ci")

    if (-not $IncludeDesktop) {
        return
    }

    Assert-CommandAvailable -Name "cargo"
    Invoke-CheckedProcess -WorkingDirectory (Join-Path $WorktreePath "tauri-client") -FilePath "npm.cmd" -Arguments @("ci")
    Invoke-CheckedProcess -WorkingDirectory (Join-Path $WorktreePath "tauri-client") -FilePath "cargo" -Arguments @(
        "fetch",
        "--manifest-path",
        "src-tauri/Cargo.toml"
    )
}

function Test-ApprovedMarker {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        return $false
    }

    return (Get-Content -Raw -LiteralPath $Path).Trim() -eq "APPROVED"
}
