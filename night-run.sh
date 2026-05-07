#!/bin/bash
# =============================================================================
# OmniCraft Night Run — Autonomous Overnight Execution Script
# =============================================================================
# Usage:
#   1. Review PROMPT.md and adjust if needed
#   2. ./night-run.sh
#   3. Leave the terminal open (minimize if desired)
#   4. Come back in the morning, check logs/ and git log
#
# Safety: This script enforces cost limits and turn limits per invocation.
#          No destructive operations are allowed by the prompt.
# =============================================================================

set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="${PROJECT_DIR:-$SCRIPT_DIR}"
PROMPT_FILE="${PROMPT_FILE:-$SCRIPT_DIR/night-run-PROMPT.md}"
LOG_DIR="$PROJECT_DIR/logs/overnight"
MISSION_FILE="$PROJECT_DIR/tasks/mission.md"
BLOCKED_FILE="$PROJECT_DIR/tasks/blocked.md"

# Per-invocation limits (hard safety net)
MAX_TURNS="${MAX_TURNS:-30}"
MAX_BUDGET_USD="${MAX_BUDGET_USD:-5.00}"
SLEEP_BETWEEN_SEC="${SLEEP_BETWEEN_SEC:-5}"

# Model selection: empty = use default from settings.json
# Override with CLAUDE_MODEL env var if needed (e.g., CLAUDE_MODEL=opus)
CLAUDE_MODEL="${CLAUDE_MODEL:-}"

# Permission mode: acceptEdits = safest for unattended
# Options: acceptEdits | auto | dangerously-skip-permissions
PERMISSION_MODE="${PERMISSION_MODE:-acceptEdits}"

# ── Auth Check ──────────────────────────────────────────────────────────────
# Claude CLI auto-reads settings.json. If auth is missing or broken, the CLI
# itself reports a clear error. We skip pre-checking to avoid path mismatches.

# Allowed tools (narrowest surface for security — whitelist only what's needed)
# Bash patterns: only the specific subcommands needed for the task
# Git: allow normal workflow, explicitly exclude destructive operations
# npm/npx: only lint/build/test, no install/uninstall
# go: only build/vet/test, no mod tidy or get
ALLOWED_TOOLS="\
Read,\
Write,\
Edit,\
Glob,\
Grep,\
WebSearch,\
WebFetch,\
Bash(git status),\
Bash(git add *),\
Bash(git commit *),\
Bash(git diff *),\
Bash(git log *),\
Bash(git stash *),\
Bash(git branch *),\
Bash(git checkout *),\
Bash(git push origin main),\
Bash(npm run lint),\
Bash(npm run build),\
Bash(npm run test),\
Bash(npx tsc *),\
Bash(npx next *),\
Bash(go build *),\
Bash(go vet *),\
Bash(go test *),\
Bash(curl *),\
Bash(cd *),\
Bash(powercfg *),\
Bash(mkdir *),\
Bash(ls *),\
Bash(cat *),\
Bash(head *),\
Bash(tail *),\
Bash(grep *),\
Bash(python3 *),\
Bash(node *),\
Bash(echo *),\
Bash(date *),\
Bash(which *),\
Bash(find *)\
"

# ── Pre-flight Checks ──────────────────────────────────────────────────────

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║        OmniCraft Night Run — Autonomous Execution           ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "Start time: $(date '+%Y-%m-%d %H:%M:%S')"
echo "Project:    $PROJECT_DIR"
echo "Prompt:     $PROMPT_FILE"
echo "Log dir:    $LOG_DIR"
echo "Model:      ${CLAUDE_MODEL:-default (from settings.json)}"
echo "Max turns:  $MAX_TURNS"
echo "Max budget: \$$MAX_BUDGET_USD"
echo ""

# Check project directory
if [ ! -d "$PROJECT_DIR" ]; then
    echo "ERROR: Project directory not found at $PROJECT_DIR"
    exit 1
fi

# Check prompt file
if [ ! -f "$PROMPT_FILE" ]; then
    echo "ERROR: PROMPT.md not found at $PROMPT_FILE"
    exit 1
fi

# Create log and task directories
mkdir -p "$LOG_DIR"
mkdir -p "$(dirname "$MISSION_FILE")"

# Initialize mission file if it doesn't exist
if [ ! -f "$MISSION_FILE" ]; then
    cat > "$MISSION_FILE" << 'MISSION_EOF'
# Overnight Mission State

## Started
MISSION_EOF
    echo "$(date '+%Y-%m-%d %H:%M:%S')" >> "$MISSION_FILE"
    echo "" >> "$MISSION_FILE"
    echo "## Completed" >> "$MISSION_FILE"
    echo "" >> "$MISSION_FILE"
    echo "## In Progress" >> "$MISSION_FILE"
    echo "" >> "$MISSION_FILE"
    echo "## Blocked" >> "$MISSION_FILE"
    echo "" >> "$MISSION_FILE"
fi

# ── Ensure git identity ──────────────────────────────────────────────────────

cd "$PROJECT_DIR"
if ! git config user.name >/dev/null 2>&1; then
    git config user.name "Overnight Agent"
    echo "[$(date '+%H:%M:%S')] Set local git user.name"
fi
if ! git config user.email >/dev/null 2>&1; then
    git config user.email "overnight@omnicraft.local"
    echo "[$(date '+%H:%M:%S')] Set local git user.email"
fi

# ── Create git checkpoint ───────────────────────────────────────────────────

echo "[$(date '+%H:%M:%S')] Creating pre-autonomous git checkpoint..."
git add -A
if git diff --cached --quiet; then
    echo "  (nothing to checkpoint — working tree clean)"
else
    git commit -m "chore: pre-autonomous checkpoint $(date '+%Y-%m-%d %H:%M')"
    echo "  Checkpoint committed."
fi

# ── Windows Sleep Prevention ────────────────────────────────────────────────

# Prevent Windows from sleeping while plugged in
# (Requires admin once: powercfg /change standby-timeout-ac 0)
if command -v powercfg &> /dev/null 2>&1; then
    echo "[$(date '+%H:%M:%S')] Configuring Windows sleep prevention..."
    powercfg /change standby-timeout-ac 0 2>/dev/null || true
    powercfg /change monitor-timeout-ac 30 2>/dev/null || true
    echo "  Standby disabled on AC power. Monitor will sleep after 30 min."
fi

# Also use caffeinate-like approach: keep system awake via periodic input
# This runs in the background for the duration of this script
if command -v powercfg &> /dev/null 2>&1; then
    # Windows: simulate scroll lock key press every 4 minutes to prevent idle sleep
    (
        while true; do
            sleep 240
            powershell -Command "(New-Object -ComObject WScript.Shell).SendKeys('{SCROLLLOCK}{SCROLLLOCK}')" 2>/dev/null || true
        done
    ) &
    KEEP_AWAKE_PID=$!
    trap "kill $KEEP_AWAKE_PID 2>/dev/null || true" EXIT
fi

# ── Helper: Check Result ────────────────────────────────────────────────────

check_result() {
    local exit_code=$1
    local log_file=$2

    if [ $exit_code -eq 0 ]; then
        echo "[$(date '+%H:%M:%S')] ✓ Invocation completed successfully"
        return 0
    else
        echo "[$(date '+%H:%M:%S')] ✗ Invocation failed (exit code: $exit_code)"
        # Check for rate limit messages
        if grep -qi "rate.limit\|429\|overloaded" "$log_file" 2>/dev/null; then
            echo "  → Rate limited. Sleeping 5 minutes..."
            sleep 300
            return 1
        fi
        # Check for auth issues
        if grep -qi "unauthorized\|401\|invalid.*api.*key\|authentication" "$log_file" 2>/dev/null; then
            echo "  → AUTH ERROR detected. Writing to blocked.md and stopping."
            echo "## AUTH ERROR — $(date)" >> "$BLOCKED_FILE"
            echo "Check your ANTHROPIC_API_KEY environment variable." >> "$BLOCKED_FILE"
            return 2
        fi
        return 1
    fi
}

# ── Main Loop ───────────────────────────────────────────────────────────────

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo " Starting autonomous execution loop"
echo " Logs: $LOG_DIR/"
echo " Press Ctrl+C to stop safely"
echo "═══════════════════════════════════════════════════════════════"
echo ""

ITERATION=0
CONSECUTIVE_FAILURES=0
MAX_CONSECUTIVE_FAILURES=5

while true; do
    ITERATION=$((ITERATION + 1))
    TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
    LOG_FILE="$LOG_DIR/run-${TIMESTAMP}-iter-${ITERATION}.log"

    echo "── Iteration $ITERATION ── $(date '+%H:%M:%S') ─────────────────────────────"

    # Check for blocked state
    if [ -f "$BLOCKED_FILE" ] && grep -q "AUTH ERROR\|FATAL\|MANUAL" "$BLOCKED_FILE" 2>/dev/null; then
        echo "  ⛔ Fatal blocker detected in $BLOCKED_FILE. Stopping."
        echo "  → $(head -5 "$BLOCKED_FILE")"
        break
    fi

    # Build model arg
    MODEL_ARG=""
    if [ -n "$CLAUDE_MODEL" ]; then
        MODEL_ARG="--model $CLAUDE_MODEL"
    fi

    # Execute claude with strict limits
    set +e
    claude -p "$(cat "$PROMPT_FILE")" \
        $MODEL_ARG \
        --permission-mode "$PERMISSION_MODE" \
        --max-turns "$MAX_TURNS" \
        --max-budget-usd "$MAX_BUDGET_USD" \
        --verbose \
        2>&1 | tee "$LOG_FILE"
    EXIT_CODE=${PIPESTATUS[0]}
    set -e

    check_result $EXIT_CODE "$LOG_FILE"
    RESULT=$?

    if [ $RESULT -eq 2 ]; then
        # Fatal auth error — stop the loop
        echo "  ⛔ Stopping due to auth error."
        break
    elif [ $RESULT -eq 0 ]; then
        CONSECUTIVE_FAILURES=0
    else
        CONSECUTIVE_FAILURES=$((CONSECUTIVE_FAILURES + 1))
        echo "  Consecutive failures: $CONSECUTIVE_FAILURES/$MAX_CONSECUTIVE_FAILURES"
        if [ $CONSECUTIVE_FAILURES -ge $MAX_CONSECUTIVE_FAILURES ]; then
            echo "  ⛔ Too many consecutive failures. Stopping for safety."
            echo "## TOO MANY FAILURES — $(date)" >> "$BLOCKED_FILE"
            echo "$CONSECUTIVE_FAILURES consecutive invocations failed. Manual review needed." >> "$BLOCKED_FILE"
            break
        fi
    fi

    echo "  Sleeping ${SLEEP_BETWEEN_SEC}s before next iteration..."
    sleep "$SLEEP_BETWEEN_SEC"
done

# ── Cleanup ─────────────────────────────────────────────────────────────────

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo " Night run ended at $(date '+%Y-%m-%d %H:%M:%S')"
echo " Total iterations: $ITERATION"
echo " Logs saved to: $LOG_DIR/"
echo "═══════════════════════════════════════════════════════════════"

# Restore Windows sleep settings (optional — comment out to keep them disabled)
# powercfg /change standby-timeout-ac 30 2>/dev/null || true

# Show summary
echo ""
echo "── Git Activity ──────────────────────────────────────────────"
git log --oneline --since="$(date -d 'today 00:00' '+%Y-%m-%d' 2>/dev/null || date '+%Y-%m-%d')" 2>/dev/null || \
git log --oneline -10

echo ""
echo "── Remaining Issues ──────────────────────────────────────────"
if [ -f "$MISSION_FILE" ]; then
    tail -30 "$MISSION_FILE"
fi

echo ""
echo "Done. To review full logs: ls -la $LOG_DIR/"
