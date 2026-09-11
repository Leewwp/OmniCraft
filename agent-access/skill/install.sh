#!/usr/bin/env bash
# OmniCraft Agent Skill installer (SP-16 #449).
#   ./install.sh                # install to ~/.agents/skills/omnicraft
#   ./install.sh --target claude  # also create the Claude Code skills symlink
#   ./install.sh --target agents  # explicit default target
set -euo pipefail

TARGET="agents"
while [ $# -gt 0 ]; do
  case "$1" in
    --target)
      [ $# -ge 2 ] || { echo "--target requires a value (agents|claude)" >&2; exit 1; }
      TARGET="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
done

SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEST="$HOME/.agents/skills/omnicraft"

mkdir -p "$(dirname "$DEST")"
rm -rf "$DEST"
cp -R "$SRC_DIR" "$DEST"
chmod +x "$DEST/install.sh" 2>/dev/null || true
echo "installed: $DEST"

if [ "$TARGET" = "claude" ]; then
  CLAUDE_SKILLS_DIR="$HOME/.claude/skills"
  mkdir -p "$CLAUDE_SKILLS_DIR"
  rm -rf "$CLAUDE_SKILLS_DIR/omnicraft"
  ln -s "$DEST" "$CLAUDE_SKILLS_DIR/omnicraft"
  echo "linked:   $CLAUDE_SKILLS_DIR/omnicraft -> $DEST"
fi

echo
echo "next: export OMNICRAFT_BASE_URL=\"https://app.leeppp.online\""
echo "      then ask your agent: 搜一下乐谱，并给出第一首的使用步骤"
