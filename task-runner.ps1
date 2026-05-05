# ============================================================
# OmniCraft Task Runner - 自动化任务执行脚本 (PowerShell)
# 用法: powershell -ExecutionPolicy Bypass -File task-runner.ps1 [options]
# ============================================================

param(
    [int]$MaxTasks = 0,
    [switch]$DryRun,
    [switch]$StopOnError
)

$ErrorActionPreference = "Stop"
$ProjectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$TaskFile = Join-Path $ProjectDir "task.json"
$LogDir = Join-Path $ProjectDir ".task-logs"
$RunnerLog = Join-Path $LogDir ("runner-" + (Get-Date -Format "yyyyMMdd-HHmmss") + ".log")

if (-not (Test-Path $LogDir)) {
    New-Item -ItemType Directory -Path $LogDir -Force | Out-Null
}

function Log {
    param([string]$Message)
    $line = "[$(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')] $Message"
    Write-Host $line
    Add-Content -Path $RunnerLog -Value $line -Encoding UTF8
}

function Find-NextTask {
    $tasks = Get-Content -Path $TaskFile -Raw -Encoding UTF8 | ConvertFrom-Json
    foreach ($t in $tasks.tasks) {
        if (-not $t.passes) {
            return "$($t.id)|$($t.title)"
        }
    }
    return $null
}

function Test-GitClean {
    Push-Location $ProjectDir
    try {
        $dirty = git status --porcelain 2>$null
        if ($dirty) {
            Log "WARNING: Git working directory has uncommitted changes:"
            foreach ($line in ($dirty -split "`n")) {
                if ($line.Trim()) { Log "  $($line.Trim())" }
            }
            Log "Please commit or stash changes before running task-runner."
            return $false
        }
        return $true
    }
    finally {
        Pop-Location
    }
}

function Invoke-Task {
    param([int]$TaskId, [string]$TaskTitle)

    Log ""
    Log "--- Next Task: [$TaskId] $TaskTitle ---"

    if ($DryRun) {
        Log "DRY-RUN: Would execute Task $TaskId. Stopping."
        return $true
    }

    Log "Launching Claude Code for Task $TaskId..."
    $startTime = Get-Date

    $prompt = "Complete Task ${TaskId}: $TaskTitle from task.json in the OmniCraft project.`n`n" +
        "IMPORTANT: Follow the CLAUDE.md workflow strictly:`n" +
        "1. Read task.json for the task details and steps`n" +
        "2. Implement all steps`n" +
        "3. Test and verify (go build, npm run build, browser test if UI)`n" +
        "4. Update progress.txt with what was done`n" +
        "5. Update task.json: set passes to true for task ${TaskId}`n" +
        "6. Create ONE git commit with all changes (code + progress.txt + task.json)`n`n" +
        "CRITICAL RULES:`n" +
        "- Do NOT push to remote. Only commit locally.`n" +
        "- If the task requires external services (Alibaba Cloud keys, etc.) that are not configured, report the blockage clearly and STOP. Do NOT mark the task as complete.`n" +
        "- If you encounter any blocker that prevents task completion, explain what is needed and STOP.`n" +
        "- Only mark passes: true if ALL steps are verified.`n" +
        "- Never delete or modify task descriptions."

    try {
        claude -p $prompt 2>&1 | ForEach-Object {
            $line = $_
            Write-Host $line
            Add-Content -Path $RunnerLog -Value $line -Encoding UTF8
        }
        $exitCode = $LASTEXITCODE
        $duration = [math]::Round(((Get-Date) - $startTime).TotalSeconds, 1)

        if ($exitCode -ne 0) {
            Log "Task $TaskId exited with code $exitCode (duration: ${duration}s)"
            $tasks = Get-Content -Path $TaskFile -Raw -Encoding UTF8 | ConvertFrom-Json
            $task = $tasks.tasks | Where-Object { $_.id -eq $TaskId }
            $isDone = ($task -and $task.passes)

            if ($isDone) {
                Log "Task $TaskId marked complete despite non-zero exit. Continuing."
                return $true
            }
            else {
                Log "BLOCKED: Task $TaskId not completed (duration: ${duration}s)."
                Log ""
                Log "===== BLOCKAGE REPORT ====="
                Log "Task: [$TaskId] $TaskTitle"
                Log "Duration: ${duration}s"
                Log "Check progress.txt and $RunnerLog for details."
                Log "==========================="
                return $false
            }
        }

        Log "Task $TaskId completed successfully (duration: ${duration}s)."
        return $true
    }
    catch {
        $duration = [math]::Round(((Get-Date) - $startTime).TotalSeconds, 1)
        Log "Task $TaskId raised exception (duration: ${duration}s): $_"
        return $false
    }
}

# ============================================================
# Main
# ============================================================

Log "============================================"
Log "OmniCraft Task Runner - Starting"
Log "Project: $ProjectDir"
$maxLabel = if ($MaxTasks -eq 0) { "unlimited" } else { "$MaxTasks" }
Log "Max tasks: $maxLabel"
Log "Dry run: $DryRun"
Log "============================================"

if (-not (Test-GitClean)) {
    if ($DryRun) {
        Log "DRY-RUN: Would continue despite dirty git."
    }
    else {
        Log "Aborting due to dirty git state."
        exit 1
    }
}

$taskCount = 0
$consecutiveFailures = 0

while ($true) {
    if ($MaxTasks -gt 0 -and $taskCount -ge $MaxTasks) {
        Log "Reached max tasks limit ($MaxTasks). Stopping."
        break
    }

    if ($consecutiveFailures -ge 3) {
        Log "ERROR: 3 consecutive failures. Stopping for safety."
        exit 1
    }

    $nextTask = Find-NextTask
    if (-not $nextTask) {
        Log "All tasks are marked complete (passes: true). Nothing to do."
        break
    }

    $parts = $nextTask -split '\|', 2
    $taskId = [int]$parts[0]
    $taskTitle = $parts[1]

    $success = Invoke-Task -TaskId $taskId -TaskTitle $taskTitle

    if ($success) {
        $taskCount++
        $consecutiveFailures = 0
    }
    else {
        $consecutiveFailures++
        if ($StopOnError) {
            Log "Stopping due to -StopOnError."
            exit 1
        }
        Log "Task requires manual intervention. Stopping."
        exit 0
    }

    if ($DryRun) { break }

    Log "Waiting 5 seconds before next task..."
    Start-Sleep -Seconds 5
}

Log ""
Log "============================================"
Log "Task Runner finished. $taskCount task(s) executed."
Log "Log: $RunnerLog"
Log "============================================"
