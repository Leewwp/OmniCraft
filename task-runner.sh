#!/bin/bash
# ============================================================
# OmniCraft Task Runner - 自动化任务执行脚本
# 用法: bash task-runner.sh [options]
#
# Options:
#   --max-tasks N   最多执行 N 个任务后停止 (0 = 不限, 默认)
#   --dry-run       只显示下一个任务，不执行
#   --stop-on-error 遇到任何错误立即停止 (默认: 仅阻塞时停止)
# ============================================================

set -euo pipefail

# --- 配置 ---
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
TASK_FILE="task.json"
PROGRESS_FILE="progress.txt"
LOG_DIR=".task-logs"
RUNNER_LOG="$LOG_DIR/runner-$(date +%Y%m%d-%H%M%S).log"

MAX_TASKS=0
DRY_RUN=false
STOP_ON_ERROR=false

# --- 参数解析 ---
while [[ $# -gt 0 ]]; do
  case "$1" in
    --max-tasks)
      MAX_TASKS="${2:-0}"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --stop-on-error)
      STOP_ON_ERROR=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: bash task-runner.sh [--max-tasks N] [--dry-run] [--stop-on-error]"
      exit 1
      ;;
  esac
done

# --- 初始化 ---
mkdir -p "$LOG_DIR"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$RUNNER_LOG"
}

# --- 查找下一个未完成任务 ---
find_next_task() {
  python3 -c "
import json, sys
try:
    sys.stdout.reconfigure(encoding='utf-8')
except Exception:
    pass
with open('$TASK_FILE', encoding='utf-8') as f:
    data = json.load(f)
for t in data['tasks']:
    if not t['passes']:
        # Use ASCII-safe output to avoid pipe encoding issues
        task_id = t['id']
        task_title = t['title']
        sys.stdout.write(f'{task_id}|{task_title}\n')
        sys.exit(0)
sys.exit(1)
"
}

# --- 检查 git 状态 ---
check_git_clean() {
  cd "$PROJECT_DIR"
  local dirty
  dirty=$(git status --porcelain 2>/dev/null)
  if [ -n "$dirty" ]; then
    log "WARNING: Git working directory has uncommitted changes:"
    echo "$dirty" | while read -r line; do log "  $line"; done
    log "Please commit or stash changes before running task-runner."
    return 1
  fi
  return 0
}

# --- 主循环 ---
main() {
  log "============================================"
  log "OmniCraft Task Runner - Starting"
  log "Project: $PROJECT_DIR"
  log "Max tasks: ${MAX_TASKS:-unlimited}"
  log "Dry run: $DRY_RUN"
  log "============================================"

  if ! check_git_clean; then
    if $DRY_RUN; then
      log "DRY-RUN: Would continue despite dirty git (not recommended)."
    else
      log "Aborting due to dirty git state."
      exit 1
    fi
  fi

  local task_count=0
  local consecutive_failures=0

  while true; do
    # 检查是否达到任务上限
    if [ "$MAX_TASKS" -gt 0 ] && [ "$task_count" -ge "$MAX_TASKS" ]; then
      log "Reached max tasks limit ($MAX_TASKS). Stopping."
      break
    fi

    # 检查连续失败
    if [ "$consecutive_failures" -ge 3 ]; then
      log "ERROR: 3 consecutive failures. Stopping for safety."
      exit 1
    fi

    # 查找下一个任务
    local next_task
    next_task=$(find_next_task 2>/dev/null) || true

    if [ -z "$next_task" ]; then
      log "All tasks are marked complete (passes: true). Nothing to do."
      break
    fi

    local task_id="${next_task%%|*}"
    local task_title="${next_task#*|}"

    log ""
    log "--- Next Task: [$task_id] $task_title ---"

    if $DRY_RUN; then
      log "DRY-RUN: Would execute Task $task_id. Stopping."
      break
    fi

    # 检查是否需要暂停 (存在未跟踪的 test_all_pages.py 等)
    if [ "$task_id" -ge 30 ] 2>/dev/null; then
      log "NOTE: Task $task_id is a high-number task. Ensure dependencies are met."
    fi

    # 执行任务
    log "Launching Claude Code for Task $task_id..."
    local task_start=$(date +%s)

    # 调用 claude -p 执行单任务开发
    # -p 模式: 非交互，执行完退出
    # Claude 会自动加载 CLAUDE.md 中的工作流程
    set +e
    claude -p "Complete Task $task_id: $task_title from task.json in the OmniCraft project.

IMPORTANT: Follow the CLAUDE.md workflow strictly:
1. Read task.json for the task details and steps
2. Implement all steps
3. Test and verify (go build, npm run build, browser test if UI)
4. Update progress.txt with what was done
5. Update task.json: set passes to true for task $task_id
6. Create ONE git commit with all changes (code + progress.txt + task.json)

CRITICAL RULES:
- Do NOT push to remote. Only commit locally.
- If the task requires external services (Alibaba Cloud keys, etc.) that are not configured, report the blockage clearly and STOP. Do NOT mark the task as complete.
- If you encounter any blocker that prevents task completion, explain what's needed and STOP.
- Only mark passes: true if ALL steps are verified.
- Never delete or modify task descriptions." 2>&1 | tee -a "$RUNNER_LOG"
    local exit_code=$?
    set -e

    local task_end=$(date +%s)
    local duration=$((task_end - task_start))

    if [ $exit_code -ne 0 ]; then
      log "Task $task_id exited with code $exit_code (possible blockage or error)"
      consecutive_failures=$((consecutive_failures + 1))

      # 检查 passes 是否已更新
      local is_done
      is_done=$(python3 -c "
import json, sys
with open('$TASK_FILE', encoding='utf-8') as f:
    data = json.load(f)
for t in data['tasks']:
    if t['id'] == $task_id:
        sys.stdout.write('true' if t['passes'] else 'false')
        sys.stdout.write('\n')
        break
" 2>/dev/null || echo "error")

      if [ "$is_done" = "true" ]; then
        log "Task $task_id marked complete despite non-zero exit. Continuing."
        consecutive_failures=0
        task_count=$((task_count + 1))
      elif [ "$is_done" = "error" ]; then
        log "ERROR: Could not read task.json. Stopping."
        exit 1
      else
        log "BLOCKED: Task $task_id not completed (duration: ${duration}s)."
        if $STOP_ON_ERROR; then
          log "Stopping due to --stop-on-error."
          exit 1
        fi
        log "Task requires manual intervention. Stopping."
        log ""
        log "===== BLOCKAGE REPORT ====="
        log "Task: [$task_id] $task_title"
        log "Duration: ${duration}s"
        log "Check progress.txt and $RUNNER_LOG for details."
        log "==========================="
        exit 0
      fi
    else
      log "Task $task_id completed successfully (duration: ${duration}s)."
      consecutive_failures=0
      task_count=$((task_count + 1))
    fi

    # 短暂等待，让系统稳定
    log "Waiting 5 seconds before next task..."
    sleep 5
  done

  log ""
  log "============================================"
  log "Task Runner finished. $task_count task(s) executed."
  log "Log: $RUNNER_LOG"
  log "============================================"
}

main
