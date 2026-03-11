package toolcontract

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ToolNameApplyPatch           = "apply_patch"
	ToolNameDiff                 = "diff"
	ToolNameGeneratePatch        = "generate_patch"
	ServerName                   = "qwen-apply-patch"
	ServerVersion                = "0.1.0"
	DefaultFileMode              = 0o644
	validUpdatePatchExample      = "*** Begin Patch\n*** Update File: path/to/file.txt\n@@\n old line\n-old value\n+new value\n*** End Patch"
	validUpdateOrAddPatchExample = "*** Begin Patch\n*** Update Or Add File: path/to/file.txt\n@@\n old line\n-old value\n+new value\n*** End Patch"
	validAddPatchExample         = "*** Begin Patch\n*** Add File: path/to/new-file.txt\n+first line\n+second line\n*** End Patch"
)

type ApplyPatchRequest struct {
	Patch  string `json:"patch"`
	DryRun bool   `json:"dry_run,omitempty"`
}

type GeneratePatchRequest struct {
	Path       string `json:"path"`
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
	Mode       string `json:"mode,omitempty"`
}

type Diagnostic struct {
	Kind    string `json:"kind"`
	Line    int    `json:"line,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type OperationPreview struct {
	Kind         string `json:"kind"`
	Path         string `json:"path"`
	ToPath       string `json:"to_path,omitempty"`
	AddedLines   int    `json:"added_lines,omitempty"`
	RemovedLines int    `json:"removed_lines,omitempty"`
	ChangedLines int    `json:"changed_lines,omitempty"`
}

type ChangeStats struct {
	Files        int `json:"files"`
	AddedLines   int `json:"added_lines"`
	RemovedLines int `json:"removed_lines"`
	ChangedLines int `json:"changed_lines"`
	CreatedFiles int `json:"created_files,omitempty"`
	UpdatedFiles int `json:"updated_files,omitempty"`
	DeletedFiles int `json:"deleted_files,omitempty"`
	RenamedFiles int `json:"renamed_files,omitempty"`
}

type DisplayFile struct {
	Path            string  `json:"path"`
	OriginalContent *string `json:"original_content,omitempty"`
	NewContent      *string `json:"new_content,omitempty"`
}

type ApplyPatchResponse struct {
	OK           bool               `json:"ok"`
	DryRun       bool               `json:"dry_run"`
	Summary      string             `json:"summary"`
	Hint         string             `json:"hint,omitempty"`
	FilesChanged []string           `json:"files_changed"`
	Stats        *ChangeStats       `json:"stats,omitempty"`
	Operations   []OperationPreview `json:"operations,omitempty"`
	DisplayFiles []DisplayFile      `json:"display_files,omitempty"`
	Diagnostics  []Diagnostic       `json:"diagnostics"`
}

type GeneratePatchResponse struct {
	OK           bool          `json:"ok"`
	Summary      string        `json:"summary"`
	Hint         string        `json:"hint,omitempty"`
	Patch        string        `json:"patch,omitempty"`
	DisplayFiles []DisplayFile `json:"display_files,omitempty"`
	Diagnostics  []Diagnostic  `json:"diagnostics"`
}

type DiscoveryEntry struct {
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	ParametersJSONSchema map[string]any `json:"parameters"`
}

func DecodeRequest(payload []byte) (ApplyPatchRequest, error) {
	return DecodeApplyPatchRequest(payload)
}

func DecodeApplyPatchRequest(payload []byte) (ApplyPatchRequest, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return ApplyPatchRequest{}, fmt.Errorf("decode request: %w", err)
	}
	var req ApplyPatchRequest
	if patch, ok := raw["patch"]; ok {
		if err := json.Unmarshal(patch, &req.Patch); err != nil {
			return ApplyPatchRequest{}, fmt.Errorf("decode request: patch: %w", err)
		}
	}
	if dryRun, ok := raw["dry_run"]; ok {
		if err := json.Unmarshal(dryRun, &req.DryRun); err != nil {
			var value string
			if stringErr := json.Unmarshal(dryRun, &value); stringErr != nil {
				return ApplyPatchRequest{}, fmt.Errorf("decode request: dry_run: %w", err)
			}
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "true":
				req.DryRun = true
			case "false":
				req.DryRun = false
			default:
				return ApplyPatchRequest{}, fmt.Errorf("decode request: dry_run: invalid boolean value %q", value)
			}
		}
	}
	if req.Patch == "" {
		return ApplyPatchRequest{}, fmt.Errorf("missing required field %q", "patch")
	}
	return req, nil
}

func DecodeGeneratePatchRequest(payload []byte) (GeneratePatchRequest, error) {
	var req GeneratePatchRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return GeneratePatchRequest{}, fmt.Errorf("decode request: %w", err)
	}
	req.Path = strings.TrimSpace(req.Path)
	req.Mode = strings.TrimSpace(strings.ToLower(req.Mode))
	if req.Path == "" {
		return GeneratePatchRequest{}, fmt.Errorf("missing required field %q", "path")
	}
	if req.Mode == "" {
		req.Mode = "auto"
	}
	switch req.Mode {
	case "auto", "update", "add":
	default:
		return GeneratePatchRequest{}, fmt.Errorf("decode request: mode: invalid value %q", req.Mode)
	}
	return req, nil
}

func DiscoveryDocument() []DiscoveryEntry {
	return []DiscoveryEntry{
		{
			Name:        ToolNameApplyPatch,
			Description: "Apply a strict Codex-style patch to text files under the workspace root. The patch string must start with *** Begin Patch and end with *** End Patch. Use only *** Add File:, *** Update File:, *** Update Or Add File:, *** Delete File:, optional *** Move to:, or *** Rename File:. Do not send unified diff headers like ---/+++. Paths must be relative to the workspace root only: no absolute paths, no ~, and no .. segments. A single patch document may contain multiple file operations across multiple files. If the patch is rejected, fix the patch instead of switching to another file-writing path. Do not use Delete File plus Add File to replace an existing file just to avoid fixing an Update File hunk. All changes are validated first and committed atomically.",
			ParametersJSONSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"patch": map[string]any{
						"type":        "string",
						"description": "Full patch text in Codex apply_patch format, including *** Begin Patch and *** End Patch. Use Update File for edits to existing files, Update Or Add File when the file may be missing, and do not replace an existing file with Delete File plus Add File to work around a rejected hunk. Example: *** Begin Patch\\n*** Update File: path/to/file.txt\\n@@\\n old line\\n-old value\\n+new value\\n*** End Patch",
					},
					"dry_run": map[string]any{
						"type":        "boolean",
						"description": "Validate and compute the patch without writing files.",
						"default":     false,
					},
				},
				"required": []string{"patch"},
			},
		},
		{
			Name:        ToolNameDiff,
			Description: "Preview a strict Codex-style patch as a diff without applying it. The patch format is identical to apply_patch: start with *** Begin Patch, end with *** End Patch, use relative workspace-root paths only, and do not use ---/+++ unified diff headers. This tool accepts Rename File sugar and returns a rendered diff preview plus structured change stats.",
			ParametersJSONSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"patch": map[string]any{
						"type":        "string",
						"description": "Full patch text in Codex apply_patch format, including *** Begin Patch and *** End Patch.",
					},
				},
				"required": []string{"patch"},
			},
		},
		{
			Name:        ToolNameGeneratePatch,
			Description: "Generate a strict Codex-style patch for one file from old_content and new_content without applying it. Use mode=auto to choose Add File when old_content is empty and Update File otherwise. The generated patch preserves exact whitespace from the provided texts and is intended as a helper for models that need a valid strict patch scaffold.",
			ParametersJSONSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Relative workspace path for the file.",
					},
					"old_content": map[string]any{
						"type":        "string",
						"description": "Current file content.",
					},
					"new_content": map[string]any{
						"type":        "string",
						"description": "Desired file content.",
					},
					"mode": map[string]any{
						"type":        "string",
						"enum":        []string{"auto", "update", "add"},
						"description": "Patch generation mode. auto prefers Add File when old_content is empty and new_content is non-empty.",
						"default":     "auto",
					},
				},
				"required": []string{"path", "old_content", "new_content"},
			},
		},
	}
}

func Success(summary string, files []string, stats *ChangeStats, operations []OperationPreview, displayFiles []DisplayFile, dryRun bool) ApplyPatchResponse {
	return ApplyPatchResponse{
		OK:           true,
		DryRun:       dryRun,
		Summary:      summary,
		FilesChanged: files,
		Stats:        stats,
		Operations:   operations,
		DisplayFiles: displayFiles,
		Diagnostics:  []Diagnostic{},
	}
}

func Failure(summary string, diagnostics ...Diagnostic) ApplyPatchResponse {
	return ApplyPatchResponse{
		OK:           false,
		Summary:      failureSummary(summary, diagnostics),
		Hint:         failureHint(diagnostics),
		FilesChanged: []string{},
		Diagnostics:  diagnostics,
	}
}

func GeneratePatchSuccess(summary string, patch string, displayFiles []DisplayFile) GeneratePatchResponse {
	return GeneratePatchResponse{
		OK:           true,
		Summary:      strings.TrimSpace(summary),
		Patch:        patch,
		DisplayFiles: displayFiles,
		Diagnostics:  []Diagnostic{},
	}
}

func GeneratePatchFailure(summary string, diagnostics ...Diagnostic) GeneratePatchResponse {
	return GeneratePatchResponse{
		OK:          false,
		Summary:     failureSummary(summary, diagnostics),
		Hint:        failureHint(diagnostics),
		Diagnostics: diagnostics,
	}
}

func failureSummary(summary string, diagnostics []Diagnostic) string {
	base := strings.TrimSpace(summary)
	base = strings.TrimSuffix(base, ".")
	if len(diagnostics) == 0 {
		return base + "."
	}

	parts := make([]string, 0, minInt(len(diagnostics), 3)+1)
	parts = append(parts, base)
	limit := minInt(len(diagnostics), 3)
	for _, diagnostic := range diagnostics[:limit] {
		parts = append(parts, summarizeDiagnostic(diagnostic))
	}
	if len(diagnostics) > limit {
		parts = append(parts, fmt.Sprintf("+%d more", len(diagnostics)-limit))
	}
	message := strings.Join(parts, "; ") + "."
	if example := formatHelpExample(diagnostics); example != "" {
		message += "\nValid example:\n" + example
	}
	return message
}

func summarizeDiagnostic(diagnostic Diagnostic) string {
	shortPath := filepath.Base(diagnostic.Path)
	switch diagnostic.Kind {
	case "context_mismatch":
		if shortPath != "." && shortPath != "" {
			return fmt.Sprintf("context mismatch in %s", shortPath)
		}
		return "context mismatch"
	case "missing_file":
		if shortPath != "." && shortPath != "" {
			return fmt.Sprintf("missing file %s", shortPath)
		}
		return "missing file"
	case "create_existing_file":
		if shortPath != "." && shortPath != "" {
			return fmt.Sprintf("file already exists %s", shortPath)
		}
		return "file already exists"
	case "duplicate_operation":
		if shortPath != "." && shortPath != "" {
			return fmt.Sprintf("duplicate operation for %s", shortPath)
		}
		return "duplicate operation"
	case "replace_via_delete_add":
		if shortPath != "." && shortPath != "" {
			return fmt.Sprintf("replace via delete+add for %s", shortPath)
		}
		return "replace via delete+add"
	case "invalid_update_or_add_create":
		if shortPath != "." && shortPath != "" {
			return fmt.Sprintf("invalid update-or-add create for %s", shortPath)
		}
		return "invalid update-or-add create"
	case "no_op":
		if shortPath != "." && shortPath != "" {
			return fmt.Sprintf("no changes for %s", shortPath)
		}
		return "patch does not change any files"
	}

	message := strings.TrimSpace(diagnostic.Message)
	if diagnostic.Line > 0 {
		lineSuffix := fmt.Sprintf(" at line %d", diagnostic.Line)
		message = strings.TrimSuffix(message, lineSuffix)
	}
	message = collapseWhitespace(message)
	if message == "" {
		return strings.ReplaceAll(diagnostic.Kind, "_", " ")
	}
	return lowerFirst(message)
}

var whitespaceRe = regexp.MustCompile(`\s+`)

func collapseWhitespace(value string) string {
	return whitespaceRe.ReplaceAllString(strings.TrimSpace(value), " ")
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	return strings.ToLower(value[:1]) + value[1:]
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatHelpExample(diagnostics []Diagnostic) string {
	for _, diagnostic := range diagnostics {
		switch diagnostic.Kind {
		case "invalid_add_line":
			return validAddPatchExample
		case "missing_begin_patch",
			"missing_end_patch",
			"patch_too_short",
			"unexpected_directive",
			"unexpected_blank_line",
			"missing_update_path",
			"missing_update_or_add_path",
			"expected_hunk_header",
			"blank_hunk_line",
			"invalid_hunk_line_prefix",
			"empty_hunk",
			"update_missing_hunks",
			"update_or_add_missing_hunks":
			return validUpdatePatchExample
		case "update_or_add_move_unsupported":
			return validUpdateOrAddPatchExample
		}
	}
	return ""
}

func failureHint(diagnostics []Diagnostic) string {
	for _, diagnostic := range diagnostics {
		switch diagnostic.Kind {
		case "missing_begin_patch",
			"missing_end_patch",
			"patch_too_short",
			"unexpected_directive",
			"unexpected_blank_line",
			"missing_update_path",
			"missing_update_or_add_path",
			"expected_hunk_header",
			"blank_hunk_line",
			"invalid_hunk_line_prefix",
			"empty_hunk",
			"update_missing_hunks",
			"update_or_add_missing_hunks",
			"invalid_add_line",
			"update_or_add_move_unsupported":
			return "Use strict Codex patch format; copy the valid example and retry."
		case "context_mismatch":
			if strings.Contains(diagnostic.Message, "whitespace differs") {
				return "Read the current file again and copy the context line exactly, including whitespace."
			}
			return "Read the current file again and rebuild the hunk with exact context."
		case "replace_via_delete_add":
			return "Use Update File instead of Delete File plus Add File."
		case "missing_file":
			return "Use Add File or Update Or Add File if the file may not exist."
		case "create_existing_file":
			return "Use Update File instead of Add File for an existing file."
		case "invalid_update_or_add_create":
			return "For a missing file, Update Or Add File needs at least one + line and no - lines."
		case "no_op":
			return "The patch does not change the file; rebuild it against the current content or omit it."
		}
	}
	return ""
}
