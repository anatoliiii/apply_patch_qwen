# apply_patch_qwen

## Tool contract
- This project exposes a strict `apply_patch` tool for Qwen.
- Always send the patch as one string that starts with `*** Begin Patch` and ends with `*** End Patch`.
- Valid directives are only `*** Add File:`, `*** Update File:`, `*** Delete File:`, optional `*** Move to:`, and `*** Rename File:`.
- Never send git-style unified diff headers such as `--- a/file` or `+++ b/file`.
- For `*** Add File:`, every content line must start with `+`.
- For `*** Update File:`, each hunk starts with `@@`, and hunk lines must start with a space, `+`, or `-`.
- Paths must be relative to the workspace root. Do not use absolute paths, `~`, or `..`.

## Default patch templates
```text
*** Begin Patch
*** Add File: path/to/file.txt
+file contents
*** End Patch
```

```text
*** Begin Patch
*** Update File: path/to/file.txt
@@
 old line
-old value
+new value
*** End Patch
```

## Workspace limits
- `ReadFile` and this tool are limited to registered workspace directories.
- If you need files such as `~/.qwen/QWEN.md`, start Qwen from a directory that includes that path, or add that directory through `context.includeDirectories` or the `--include-directories` flag.
- The default global memory filename is `~/.qwen/QWEN.md` with uppercase `QWEN.md`. If you keep the file as `~/.qwen/qwen.md`, it will not be picked up unless you change `context.fileName`.
