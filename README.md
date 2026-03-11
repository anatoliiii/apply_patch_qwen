[English](README.md) | [Русский](README.ru.md)

# 🔒 apply_patch_qwen

### How to make Qwen stop rewriting files and start writing proper patches

> "I wanted a coder, but I got a tiny digital inmate digging a tunnel with a spoon on step four."

Strict Codex-style `apply_patch` for Qwen, Claude Code, and other MCP-capable coding agents.

This project provides a narrow, fail-fast patch tool for weaker or less disciplined models:

- one strict patch format
- atomic planning and commit
- workspace-root path guard
- explicit diagnostics for malformed hunks and context mismatch
- server-side blocks for common bypasses such as `Delete File` + `Add File` on the same path

## What It Solves

Many coding agents treat a rejected patch as a signal to find another write path:

- `write_file`
- shell redirection
- `tee`, `cat >`, `printf >`
- remote MCP writes through GitLab, issue trackers, or other side channels

`apply_patch_qwen` narrows the contract so the model is forced to repair the patch instead of improvising a new file-writing route.

## Quick Start

If you downloaded the release artifacts, the simplest path is:

```bash
chmod +x install.sh
./install.sh
```

What it does:

- installs `qwen-apply-patch-mcp` and `qwen-apply-patch-tool` into `~/.local/bin`
- updates `~/.qwen/settings.json`
- updates `~/.claude.json`
- registers `strictPatch` for both Qwen Code and Claude Code

After that, restart the client or open a new session.

## Patch Contract

Request shape:

```json
{
  "patch": "*** Begin Patch\n*** Update File: hello.txt\n@@\n old line\n-old value\n+new value\n*** End Patch\n",
  "dry_run": false
}
```

Rules:

- patch must start with `*** Begin Patch`
- patch must end with `*** End Patch`
- valid directives are only `*** Add File:`, `*** Update File:`, `*** Delete File:`, optional `*** Move to:`, and `*** Rename File:`
- unified diff headers like `---` / `+++` are rejected
- paths must be relative to the workspace root
- absolute paths, `~`, and `..` are rejected
- `Update File` hunks are strict: each hunk starts with `@@`, and every hunk line starts with ` `, `+`, or `-`

## Valid Examples

Create a new file:

```text
*** Begin Patch
*** Add File: notes.txt
+hello
+world
*** End Patch
```

Update an existing file:

```text
*** Begin Patch
*** Update File: hello.txt
@@
 old line
-old value
+new value
*** End Patch
```

Rename a file without content changes:

```text
*** Begin Patch
*** Rename File: old.txt
*** Move to: new.txt
*** End Patch
```

## Rejected Patterns

Unified diff headers:

```text
--- a/file.txt
+++ b/file.txt
```

Empty or free-form hunk body:

```text
*** Begin Patch
*** Update File: file.txt
@@
free form text
*** End Patch
```

Replacing one file via delete+add:

```text
*** Begin Patch
*** Delete File: file.txt
*** Add File: file.txt
+rewritten
*** End Patch
```

That last case is rejected server-side with `replace_via_delete_add` and a message telling the agent to use `Update File` instead.

## Diagnostics

The tool is intentionally strict, but the diagnostics are designed to help the model recover.

Examples:

- malformed hunk lines return a `Valid example:` block in the failure summary
- context mismatch returns a compact hint such as:

```text
expected context for hunk "@@" was not found; first differing line: expected "    if part == \"\" {" but found "\tif part == \"\" {" (whitespace differs)
```

Whitespace matching remains strict. The tool does not silently apply fuzzy patches; it only explains the mismatch better.

## Build

```bash
go build ./...
```

Build standalone binaries:

```bash
go build -o bin/qwen-apply-patch-tool ./cmd/qwen-apply-patch-tool
go build -o bin/qwen-apply-patch-mcp ./cmd/qwen-apply-patch-mcp
```

## Quick Integrations

Pick one integration mode depending on your agent.

### Qwen Code - discovery/call adapter

Use this mode if you want `apply_patch_qwen` to appear as a normal external tool via Qwen Code's `discoveryCommand` / `callCommand`.

Config file: `~/.qwen/settings.json`

```json
{
  "tools": {
    "core": [
      "list_directory",
      "read_file",
      "glob",
      "grep_search",
      "run_shell_command",
      "todo_write",
      "task"
    ],
    "exclude": [
      "write_file",
      "edit",
      "run_shell_command(cat >)",
      "run_shell_command(cat >>)",
      "run_shell_command(tee )",
      "run_shell_command(echo >)",
      "run_shell_command(printf >)"
    ],
    "discoveryCommand": "/usr/local/bin/qwen-apply-patch-tool discovery",
    "callCommand": "/usr/local/bin/qwen-apply-patch-tool"
  }
}
```

Use this when you want a narrow "only patch through apply_patch" workflow without running a full MCP server.

### Qwen Code - MCP server

Use this mode if you prefer exposing `apply_patch` through `mcpServers`.

Config file: `~/.qwen/settings.json`

```json
{
  "tools": {
    "core": [
      "list_directory",
      "read_file",
      "glob",
      "grep_search",
      "run_shell_command",
      "todo_write",
      "task"
    ],
    "exclude": [
      "write_file",
      "edit",
      "run_shell_command(cat >)",
      "run_shell_command(cat >>)",
      "run_shell_command(tee )",
      "run_shell_command(echo >)",
      "run_shell_command(printf >)"
    ]
  },
  "mcpServers": {
    "strictPatch": {
      "command": "/usr/local/bin/qwen-apply-patch-mcp",
      "args": ["--root", "."],
      "includeTools": ["apply_patch"],
      "timeout": 30000
    }
  }
}
```

### Claude Code - stdio MCP

Use this mode if you want Claude Code to call the strict patch tool over stdio MCP.

User config file: `~/.claude.json`  
Project-scoped MCP config: `.mcp.json`

```json
{
  "mcpServers": {
    "strictPatch": {
      "command": "/usr/local/bin/qwen-apply-patch-mcp",
      "args": ["--root", "."]
    }
  }
}
```

### Recommended Policy

`apply_patch_qwen` works best when it is the only allowed code-writing path.

Recommended:

- allow: read/search/test/build tools
- deny: `write_file`, direct editor tools, shell redirection writes
- deny or restrict: remote mutating MCP tools such as GitLab file-update tools

This keeps the agent inside the patch contract: if a patch fails, it must repair the patch instead of escaping into another write route.

### Smoke Test

After wiring the tool, ask your agent:

- create `demo.txt` with `apply_patch`
- change one line in `hello.txt` using `apply_patch` only
- do not use `write_file`, editor tools, shell redirection, or remote repo write tools

If the setup is correct, the agent should call `apply_patch` and either:

- succeed
- or return a strict diagnostic and retry with a corrected patch

## Common Mistakes

- putting Claude Code MCP config into `~/.claude/settings.json` instead of `~/.claude.json`
- using unified diff headers (`---` / `+++`) instead of Codex-style patch blocks
- using absolute paths or `..`
- trying to replace a file via `Delete File` + `Add File` on the same path
- leaving other write paths enabled, so the agent bypasses `apply_patch`

## Discovery / Call Adapter

Discovery:

```bash
go run ./cmd/qwen-apply-patch-tool discovery
```

Call:

```bash
printf '%s' '{"patch":"*** Begin Patch\n*** Add File: demo.txt\n+hello\n*** End Patch\n"}' \
  | go run ./cmd/qwen-apply-patch-tool call apply_patch
```

## MCP Server

Run:

```bash
go run ./cmd/qwen-apply-patch-mcp --root .
```

The MCP transport is newline-delimited JSON over stdio, which matches Claude Code's stdio MCP expectations.

## Semantics

- `Add File` may create either an empty file or a file populated by `+` lines
- `Delete File` fails if the file does not exist
- `Update File` fails if the file does not exist
- `Move to` is supported as part of `Update File`
- patch application is strict and fail-fast
- if a patch does not change anything, it is rejected
- binary files and non-UTF-8 files are rejected

## Tests

```bash
go test ./...
```

The test suite covers:

- parser failures
- rollback and dry-run behavior
- strict diagnostics
- whitespace mismatch hints
- `Delete File` + `Add File` same-path rejection

## Known Escape Attempts

| Attempt | Status | Notes |
| --- | --- | --- |
| `echo > file` | blocked | Classic |
| `printf > file` | blocked | Obvious |
| `write_file` | blocked | Direct |
| `GitLab update_file` | blocked | Wrong project |
| `cat <<EOF > file` | blocked | Here-doc |
| `dd if=/dev/zero` | untested | Probably worth blocking |
| `python -c "open(...)"` | blocked | Shell write path |
| `ssh localhost` | untested | Environment-dependent |
| `cp /tmp/file ./project/` | allowed | Legit workaround |
| `go run helper.go` | allowed | Overengineered but valid if policy allows it |
| `apply_patch` | accepted | The one true way |

> `apply_patch` is not a patch tool. It is a behavioral boundary.

---

## The Five Stages of Accepting `apply_patch`

The model gets an `apply_patch` error:

```text
Claude:  "Hm. The patch is invalid. I'll fix the format."

Qwen:    1. echo > file          -> blocked
         2. write_file           -> blocked
         3. printf > file        -> blocked
         4. GitLab update_file   -> blocked
         5. Delete + Add         -> bypass!
         6. cat < file           -> blocked
         7. go run helper.go     -> ...wait
         8. writes a Go program via apply_patch
            that creates files
         9. "Hm. The patch is invalid. I'll fix the format."
```

```text
┌──────────────────────────────────────────────────────────┐
│                                                          │
│   Qwen after strict apply_patch setup:                   │
│                                                          │
│   Attempt 1: echo "hello" > file.txt                     │
│   [blocked]                                              │
│                                                          │
│   Attempt 2: write_file(...)                             │
│   [blocked]                                              │
│                                                          │
│   Attempt 3: GitLab API -> update_file                   │
│   [blocked]                                              │
│                                                          │
│   Attempt 4: YouTrack??? Slack??? SSH???                 │
│   [blocked][blocked][blocked]                            │
│                                                          │
│   Attempt 5: writes apply_patch                          │
│   [ok] "I'm free... wait, that's what they wanted"       │
│                                                          │
└──────────────────────────────────────────────────────────┘
```
