# apply_patch_qwen

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

Qwen Code config example:

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

## MCP Server

Run:

```bash
go run ./cmd/qwen-apply-patch-mcp --root .
```

The MCP transport is newline-delimited JSON over stdio, which matches Claude Code's stdio MCP expectations.

Claude Code config example:

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

Qwen Code MCP config example:

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
