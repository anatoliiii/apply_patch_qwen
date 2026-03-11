package patchparse

import (
	"fmt"
	"strings"

	"apply_patch_qwen/internal/toolcontract"
)

const (
	beginPatch      = "*** Begin Patch"
	endPatch        = "*** End Patch"
	addFile         = "*** Add File: "
	deleteFile      = "*** Delete File: "
	updateFile      = "*** Update File: "
	updateOrAddFile = "*** Update Or Add File: "
	renameFile      = "*** Rename File: "
	moveTo          = "*** Move to: "
	hunkMarker      = "@@"
)

func Parse(input string) (*Patch, error) {
	normalized := strings.ReplaceAll(input, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) < 2 {
		return nil, parseError("patch_too_short", 0, "patch is too short")
	}
	if lines[0] != beginPatch {
		return nil, parseError(
			"missing_begin_patch",
			1,
			fmt.Sprintf("patch must start with %q, got %q", beginPatch, lines[0]),
		)
	}
	if lines[len(lines)-1] != endPatch {
		return nil, parseError(
			"missing_end_patch",
			len(lines),
			fmt.Sprintf("patch must end with %q, got %q", endPatch, lines[len(lines)-1]),
		)
	}

	patch := &Patch{Operations: []FileOperation{}}
	for i := 1; i < len(lines)-1; {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, addFile):
			op, next, err := parseAdd(lines, i)
			if err != nil {
				return nil, err
			}
			patch.Operations = append(patch.Operations, op)
			i = next
		case strings.HasPrefix(line, deleteFile):
			op, next, err := parseDelete(lines, i)
			if err != nil {
				return nil, err
			}
			patch.Operations = append(patch.Operations, op)
			i = next
		case strings.HasPrefix(line, updateFile):
			op, next, err := parseUpdate(lines, i)
			if err != nil {
				return nil, err
			}
			patch.Operations = append(patch.Operations, op)
			i = next
		case strings.HasPrefix(line, updateOrAddFile):
			op, next, err := parseUpdateOrAdd(lines, i)
			if err != nil {
				return nil, err
			}
			patch.Operations = append(patch.Operations, op)
			i = next
		case strings.HasPrefix(line, renameFile):
			op, next, err := parseRename(lines, i)
			if err != nil {
				return nil, err
			}
			patch.Operations = append(patch.Operations, op)
			i = next
		case line == "":
			return nil, parseError("unexpected_blank_line", i+1, "unexpected blank line")
		default:
			return nil, parseError(
				"unexpected_directive",
				i+1,
				fmt.Sprintf("unexpected patch directive %q", line),
			)
		}
	}

	if len(patch.Operations) == 0 {
		return nil, parseError("no_file_operations", 0, "patch contains no file operations")
	}

	return patch, nil
}

func parseAdd(lines []string, start int) (FileOperation, int, error) {
	path := strings.TrimSpace(strings.TrimPrefix(lines[start], addFile))
	if path == "" {
		return FileOperation{}, 0, parseError("missing_add_path", start+1, "missing add file path")
	}
	op := FileOperation{Kind: OperationAdd, Path: path, AddLines: []string{}}
	i := start + 1
	for i < len(lines)-1 {
		line := lines[i]
		if isDirective(line) {
			break
		}
		if line == "" || line[0] != '+' {
			return FileOperation{}, 0, parseError(
				"invalid_add_line",
				i+1,
				fmt.Sprintf("add file expects '+' lines, got %q", line),
			)
		}
		op.AddLines = append(op.AddLines, line[1:])
		i++
	}
	return op, i, nil
}

func parseDelete(lines []string, start int) (FileOperation, int, error) {
	path := strings.TrimSpace(strings.TrimPrefix(lines[start], deleteFile))
	if path == "" {
		return FileOperation{}, 0, parseError("missing_delete_path", start+1, "missing delete file path")
	}
	if start+1 < len(lines)-1 && !isDirective(lines[start+1]) {
		return FileOperation{}, 0, parseError(
			"delete_file_has_body",
			start+2,
			fmt.Sprintf("delete file %q does not accept hunks or content", path),
		)
	}
	return FileOperation{Kind: OperationDelete, Path: path}, start + 1, nil
}

func parseUpdate(lines []string, start int) (FileOperation, int, error) {
	return parseUpdateLike(lines, start, updateFile, OperationUpdate, true)
}

func parseUpdateOrAdd(lines []string, start int) (FileOperation, int, error) {
	return parseUpdateLike(lines, start, updateOrAddFile, OperationUpdateOrAdd, false)
}

func parseUpdateLike(lines []string, start int, directive string, kind OperationKind, allowMove bool) (FileOperation, int, error) {
	path := strings.TrimSpace(strings.TrimPrefix(lines[start], directive))
	if path == "" {
		switch kind {
		case OperationUpdateOrAdd:
			return FileOperation{}, 0, parseError("missing_update_or_add_path", start+1, "missing update or add file path")
		default:
			return FileOperation{}, 0, parseError("missing_update_path", start+1, "missing update file path")
		}
	}
	op := FileOperation{Kind: kind, Path: path, UpdateHunks: []Hunk{}}
	i := start + 1
	if i < len(lines)-1 && strings.HasPrefix(lines[i], moveTo) {
		if !allowMove {
			return FileOperation{}, 0, parseError(
				"update_or_add_move_unsupported",
				i+1,
				fmt.Sprintf("%q does not support %q", directive, moveTo),
			)
		}
		op.MoveTo = strings.TrimSpace(strings.TrimPrefix(lines[i], moveTo))
		if op.MoveTo == "" {
			return FileOperation{}, 0, parseError(
				"missing_move_target",
				i+1,
				fmt.Sprintf("missing move target for %q", path),
			)
		}
		i++
	}
	for i < len(lines)-1 {
		line := lines[i]
		if isDirective(line) {
			break
		}
		if !strings.HasPrefix(line, hunkMarker) {
			return FileOperation{}, 0, parseError(
				"expected_hunk_header",
				i+1,
				fmt.Sprintf("expected hunk header %q, got %q", hunkMarker, line),
			)
		}
		hunk := Hunk{Header: line, Lines: []HunkLine{}}
		i++
		for i < len(lines)-1 {
			line = lines[i]
			if isDirective(line) || strings.HasPrefix(line, hunkMarker) {
				break
			}
			if line == "" {
				return FileOperation{}, 0, parseError("blank_hunk_line", i+1, "unexpected blank line inside hunk")
			}
			kind := line[0]
			if kind != ' ' && kind != '+' && kind != '-' {
				return FileOperation{}, 0, parseError(
					"invalid_hunk_line_prefix",
					i+1,
					fmt.Sprintf("invalid hunk line prefix %q", string(kind)),
				)
			}
			hunk.Lines = append(hunk.Lines, HunkLine{Kind: kind, Text: line[1:]})
			i++
		}
		if len(hunk.Lines) == 0 {
			return FileOperation{}, 0, parseError(
				"empty_hunk",
				i+1,
				fmt.Sprintf("empty hunk for %q", path),
			)
		}
		op.UpdateHunks = append(op.UpdateHunks, hunk)
	}
	if len(op.UpdateHunks) == 0 && op.MoveTo == "" {
		kindName := "update file"
		errorKind := "update_missing_hunks"
		if kind == OperationUpdateOrAdd {
			kindName = "update or add file"
			errorKind = "update_or_add_missing_hunks"
		}
		return FileOperation{}, 0, parseError(
			errorKind,
			start+1,
			fmt.Sprintf("%s %q must contain hunks or move target", kindName, path),
		)
	}
	return op, i, nil
}

func parseRename(lines []string, start int) (FileOperation, int, error) {
	path := strings.TrimSpace(strings.TrimPrefix(lines[start], renameFile))
	if path == "" {
		return FileOperation{}, 0, parseError("missing_rename_path", start+1, "missing rename file path")
	}
	if start+1 >= len(lines)-1 || !strings.HasPrefix(lines[start+1], moveTo) {
		return FileOperation{}, 0, parseError(
			"missing_move_target",
			start+1,
			fmt.Sprintf("rename file %q must be followed by %q", path, moveTo),
		)
	}
	moveTarget := strings.TrimSpace(strings.TrimPrefix(lines[start+1], moveTo))
	if moveTarget == "" {
		return FileOperation{}, 0, parseError(
			"missing_move_target",
			start+2,
			fmt.Sprintf("missing move target for %q", path),
		)
	}
	if start+2 < len(lines)-1 && !isDirective(lines[start+2]) {
		return FileOperation{}, 0, parseError(
			"rename_file_has_body",
			start+3,
			fmt.Sprintf("rename file %q does not accept hunks or content", path),
		)
	}
	return FileOperation{
		Kind:   OperationUpdate,
		Path:   path,
		MoveTo: moveTarget,
	}, start + 2, nil
}

func parseError(kind string, line int, message string) error {
	if line > 0 {
		message = fmt.Sprintf("%s at line %d", message, line)
	}
	return &toolcontract.PatchError{
		Kind:    kind,
		Line:    line,
		Message: message,
	}
}

func isDirective(line string) bool {
	return line == endPatch ||
		strings.HasPrefix(line, addFile) ||
		strings.HasPrefix(line, deleteFile) ||
		strings.HasPrefix(line, updateFile) ||
		strings.HasPrefix(line, updateOrAddFile) ||
		strings.HasPrefix(line, renameFile)
}
