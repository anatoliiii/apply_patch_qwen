#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${INSTALL_BIN_DIR:-$HOME/.local/bin}"
QWEN_SETTINGS_PATH="${QWEN_SETTINGS_PATH:-$HOME/.qwen/settings.json}"
CLAUDE_SETTINGS_PATH="${CLAUDE_SETTINGS_PATH:-$HOME/.claude.json}"

MCP_SRC="$SCRIPT_DIR/bin/qwen-apply-patch-mcp"
TOOL_SRC="$SCRIPT_DIR/bin/qwen-apply-patch-tool"
MCP_DST="$BIN_DIR/qwen-apply-patch-mcp"
TOOL_DST="$BIN_DIR/qwen-apply-patch-tool"

require_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "missing required file: $path" >&2
    exit 1
  fi
}

require_cmd() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "missing required command: $name" >&2
    exit 1
  fi
}

install_binary() {
  local src="$1"
  local dst="$2"
  install -m 755 "$src" "$dst"
}

update_qwen_settings() {
  local path="$1"
  local mcp_command="$2"
  mkdir -p "$(dirname "$path")"
  python3 - "$path" "$mcp_command" <<'PY'
import json
import os
import sys

path = sys.argv[1]
command = sys.argv[2]

data = {}
if os.path.exists(path) and os.path.getsize(path) > 0:
    with open(path, "r", encoding="utf-8") as fh:
        data = json.load(fh)
    if not isinstance(data, dict):
        raise SystemExit(f"{path} must contain a JSON object")

tools = data.setdefault("tools", {})
exclude = tools.setdefault("exclude", [])
for item in [
    "write_file",
    "edit",
    "run_shell_command(cat >)",
    "run_shell_command(cat >>)",
    "run_shell_command(tee )",
    "run_shell_command(echo >)",
    "run_shell_command(printf >)",
]:
    if item not in exclude:
        exclude.append(item)

mcp = data.setdefault("mcpServers", {})
mcp["strictPatch"] = {
    "command": command,
    "args": ["--root", "."],
    "includeTools": ["apply_patch", "diff", "generate_patch"],
    "timeout": 30000,
}

with open(path, "w", encoding="utf-8") as fh:
    json.dump(data, fh, indent=2, ensure_ascii=False)
    fh.write("\n")
PY
}

update_claude_settings() {
  local path="$1"
  local mcp_command="$2"
  mkdir -p "$(dirname "$path")"
  python3 - "$path" "$mcp_command" <<'PY'
import json
import os
import sys

path = sys.argv[1]
command = sys.argv[2]

data = {}
if os.path.exists(path) and os.path.getsize(path) > 0:
    with open(path, "r", encoding="utf-8") as fh:
        data = json.load(fh)
    if not isinstance(data, dict):
        raise SystemExit(f"{path} must contain a JSON object")

mcp = data.setdefault("mcpServers", {})
mcp["strictPatch"] = {
    "command": command,
    "args": ["--root", "."],
}

with open(path, "w", encoding="utf-8") as fh:
    json.dump(data, fh, indent=2, ensure_ascii=False)
    fh.write("\n")
PY
}

require_cmd install
require_cmd python3
require_file "$MCP_SRC"
require_file "$TOOL_SRC"

mkdir -p "$BIN_DIR"
install_binary "$MCP_SRC" "$MCP_DST"
install_binary "$TOOL_SRC" "$TOOL_DST"
update_qwen_settings "$QWEN_SETTINGS_PATH" "$MCP_DST"
update_claude_settings "$CLAUDE_SETTINGS_PATH" "$MCP_DST"

echo "Installed binaries:"
echo "  $MCP_DST"
echo "  $TOOL_DST"
echo
echo "Updated configs:"
echo "  $QWEN_SETTINGS_PATH"
echo "  $CLAUDE_SETTINGS_PATH"
echo
echo "Restart Qwen Code / Claude Code or start a new session to pick up strictPatch."
if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
  echo
  echo "Note: $BIN_DIR is not currently in PATH."
  echo "Add it to your shell profile if you want to call the binaries directly."
fi
