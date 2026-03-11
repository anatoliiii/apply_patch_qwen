package patchapply

import (
	"fmt"
	"strings"

	"apply_patch_qwen/internal/toolcontract"
)

func (e *Executor) GeneratePatch(req toolcontract.GeneratePatchRequest) (toolcontract.GeneratePatchResponse, error) {
	resolved, err := e.root.Resolve(req.Path)
	if err != nil {
		if patchErr, ok := err.(*toolcontract.PatchError); ok {
			return toolcontract.GeneratePatchFailure("Patch generation rejected.", patchErr.Diagnostic()), nil
		}
		return toolcontract.GeneratePatchFailure("Patch generation rejected.", toolcontract.Diagnostic{
			Kind:    "generate_error",
			Message: err.Error(),
		}), nil
	}

	if req.OldContent == req.NewContent {
		return toolcontract.GeneratePatchFailure("Patch generation rejected.", toolcontract.Diagnostic{
			Kind:    "no_op",
			Path:    resolved.Relative,
			Message: "old_content and new_content are identical",
		}), nil
	}

	mode := req.Mode
	if mode == "" {
		mode = "auto"
	}
	switch mode {
	case "auto":
		if req.OldContent == "" && req.NewContent != "" {
			return toolcontract.GeneratePatchSuccess(
				fmt.Sprintf("Generated add patch for %s", resolved.Relative),
				buildAddPatch(resolved.Relative, req.NewContent),
				buildGenerateDisplayFiles(resolved.Relative, req.OldContent, req.NewContent),
			), nil
		}
		return toolcontract.GeneratePatchSuccess(
			fmt.Sprintf("Generated update patch for %s", resolved.Relative),
			buildUpdatePatch(resolved.Relative, req.OldContent, req.NewContent),
			buildGenerateDisplayFiles(resolved.Relative, req.OldContent, req.NewContent),
		), nil
	case "add":
		return toolcontract.GeneratePatchSuccess(
			fmt.Sprintf("Generated add patch for %s", resolved.Relative),
			buildAddPatch(resolved.Relative, req.NewContent),
			buildGenerateDisplayFiles(resolved.Relative, req.OldContent, req.NewContent),
		), nil
	case "update":
		if req.OldContent == "" {
			return toolcontract.GeneratePatchFailure("Patch generation rejected.", toolcontract.Diagnostic{
				Kind:    "missing_file",
				Path:    resolved.Relative,
				Message: "cannot generate Update File for empty old_content; use Add File or auto mode",
			}), nil
		}
		return toolcontract.GeneratePatchSuccess(
			fmt.Sprintf("Generated update patch for %s", resolved.Relative),
			buildUpdatePatch(resolved.Relative, req.OldContent, req.NewContent),
			buildGenerateDisplayFiles(resolved.Relative, req.OldContent, req.NewContent),
		), nil
	default:
		return toolcontract.GeneratePatchFailure("Patch generation rejected.", toolcontract.Diagnostic{
			Kind:    "invalid_mode",
			Path:    resolved.Relative,
			Message: fmt.Sprintf("unsupported generation mode %q", mode),
		}), nil
	}
}

func buildGenerateDisplayFiles(path string, oldContent string, newContent string) []toolcontract.DisplayFile {
	display := toolcontract.DisplayFile{Path: path}
	if oldContent != "" {
		oldValue := oldContent
		display.OriginalContent = &oldValue
	}
	newValue := newContent
	display.NewContent = &newValue
	return []toolcontract.DisplayFile{display}
}

func buildAddPatch(path string, content string) string {
	lines := patchLines(content)
	var b strings.Builder
	b.WriteString("*** Begin Patch\n")
	b.WriteString("*** Add File: ")
	b.WriteString(path)
	b.WriteByte('\n')
	for _, line := range lines {
		b.WriteByte('+')
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("*** End Patch\n")
	return b.String()
}

func buildUpdatePatch(path string, oldContent string, newContent string) string {
	oldLines := patchLines(oldContent)
	newLines := patchLines(newContent)
	var b strings.Builder
	b.WriteString("*** Begin Patch\n")
	b.WriteString("*** Update File: ")
	b.WriteString(path)
	b.WriteByte('\n')
	b.WriteString("@@\n")
	for _, line := range oldLines {
		b.WriteByte('-')
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, line := range newLines {
		b.WriteByte('+')
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("*** End Patch\n")
	return b.String()
}

func patchLines(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if normalized == "" {
		return nil
	}
	if strings.HasSuffix(normalized, "\n") {
		normalized = normalized[:len(normalized)-1]
	}
	return strings.Split(normalized, "\n")
}
