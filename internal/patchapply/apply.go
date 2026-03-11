package patchapply

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"apply_patch_qwen/internal/fsguard"
	"apply_patch_qwen/internal/patchparse"
	"apply_patch_qwen/internal/toolcontract"
)

type fileState struct {
	Exists   bool
	Content  string
	EOL      string
	FileMode os.FileMode
}

type Executor struct {
	root *fsguard.Root
}

func New(root string) (*Executor, error) {
	guard, err := fsguard.New(root)
	if err != nil {
		return nil, err
	}
	return &Executor{root: guard}, nil
}

func (e *Executor) Apply(req toolcontract.ApplyPatchRequest) (toolcontract.ApplyPatchResponse, error) {
	parsed, originals, logical, changedPaths, err := e.prepare(req)
	if err != nil {
		if patchErr, ok := err.(*toolcontract.PatchError); ok {
			return toolcontract.Failure("Patch rejected.", patchErr.Diagnostic()), nil
		}
		return toolcontract.Failure("Patch rejected.", toolcontract.Diagnostic{
			Kind:    "apply_error",
			Message: err.Error(),
		}), nil
	}

	files := setToSortedSlice(changedPaths)
	displayFiles := buildDisplayFiles(originals, logical, changedPaths)
	operations := buildOperationPreviews(parsed, originals, logical)
	stats := buildChangeStats(operations, len(files))
	summary := formatSummary("Applied patch", len(files), stats)
	if operationsSummary := summarizeOperations(operations); operationsSummary != "" {
		summary += "; " + operationsSummary
	}
	if req.DryRun {
		summary = formatSummary("Validated patch", len(files), stats)
		if operationsSummary := summarizeOperations(operations); operationsSummary != "" {
			summary += "; " + operationsSummary
		}
		return toolcontract.Success(summary, files, stats, operations, displayFiles, true), nil
	}

	if err := e.commit(originals, logical, changedPaths); err != nil {
		if patchErr, ok := err.(*toolcontract.PatchError); ok {
			return toolcontract.Failure("Patch rejected.", patchErr.Diagnostic()), nil
		}
		return toolcontract.Failure("Patch rejected.", toolcontract.Diagnostic{
			Kind:    "commit_error",
			Message: err.Error(),
		}), nil
	}
	return toolcontract.Success(summary, files, stats, operations, displayFiles, false), nil
}

func (e *Executor) Diff(req toolcontract.ApplyPatchRequest) (toolcontract.ApplyPatchResponse, error) {
	parsed, originals, logical, changedPaths, err := e.prepare(req)
	if err != nil {
		if patchErr, ok := err.(*toolcontract.PatchError); ok {
			return toolcontract.Failure("Patch rejected.", patchErr.Diagnostic()), nil
		}
		return toolcontract.Failure("Patch rejected.", toolcontract.Diagnostic{
			Kind:    "apply_error",
			Message: err.Error(),
		}), nil
	}

	files := setToSortedSlice(changedPaths)
	displayFiles := buildDisplayFiles(originals, logical, changedPaths)
	operations := buildOperationPreviews(parsed, originals, logical)
	stats := buildChangeStats(operations, len(files))
	summary := formatSummary("Previewed diff", len(files), stats)
	if operationsSummary := summarizeOperations(operations); operationsSummary != "" {
		summary += "; " + operationsSummary
	}
	return toolcontract.Success(summary, files, stats, operations, displayFiles, true), nil
}

func (e *Executor) prepare(req toolcontract.ApplyPatchRequest) (*patchparse.Patch, map[string]fileState, map[string]fileState, map[string]struct{}, error) {
	parsed, err := patchparse.Parse(req.Patch)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	originals, logical, changedPaths, err := e.buildPlan(parsed)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return parsed, originals, logical, changedPaths, nil
}

func (e *Executor) buildPlan(parsed *patchparse.Patch) (map[string]fileState, map[string]fileState, map[string]struct{}, error) {
	pathSeen := map[string]string{}
	originals := map[string]fileState{}
	logical := map[string]fileState{}
	changedPaths := map[string]struct{}{}

	for _, op := range parsed.Operations {
		resolvedPath, err := e.root.Resolve(op.Path)
		if err != nil {
			return nil, nil, nil, err
		}
		if prev, ok := pathSeen[resolvedPath.Relative]; ok {
			if isDeleteAddReplacement(prev, op.Kind) {
				return nil, nil, nil, &toolcontract.PatchError{
					Kind:    "replace_via_delete_add",
					Path:    resolvedPath.Relative,
					Message: "use Update File instead of Delete File plus Add File to modify an existing file",
				}
			}
			return nil, nil, nil, &toolcontract.PatchError{
				Kind:    "duplicate_operation",
				Path:    resolvedPath.Relative,
				Message: fmt.Sprintf("path is already used by %s", prev),
			}
		}
		pathSeen[resolvedPath.Relative] = string(op.Kind)

		current, err := e.readState(resolvedPath.Absolute)
		if err != nil {
			return nil, nil, nil, err
		}
		originals[resolvedPath.Relative] = current
		logical[resolvedPath.Relative] = current

		destRelative := resolvedPath.Relative
		if op.MoveTo != "" {
			resolvedDest, err := e.root.Resolve(op.MoveTo)
			if err != nil {
				return nil, nil, nil, err
			}
			if prev, ok := pathSeen[resolvedDest.Relative]; ok {
				return nil, nil, nil, &toolcontract.PatchError{
					Kind:    "duplicate_operation",
					Path:    resolvedDest.Relative,
					Message: fmt.Sprintf("destination path is already used by %s", prev),
				}
			}
			pathSeen[resolvedDest.Relative] = "move_destination"
			destCurrent, err := e.readState(resolvedDest.Absolute)
			if err != nil {
				return nil, nil, nil, err
			}
			originals[resolvedDest.Relative] = destCurrent
			if _, ok := logical[resolvedDest.Relative]; !ok {
				logical[resolvedDest.Relative] = destCurrent
			}
			destRelative = resolvedDest.Relative
		}

		switch op.Kind {
		case patchparse.OperationAdd:
			if logical[resolvedPath.Relative].Exists {
				return nil, nil, nil, &toolcontract.PatchError{
					Kind:    "create_existing_file",
					Path:    resolvedPath.Relative,
					Message: "cannot add a file that already exists",
				}
			}
			logical[resolvedPath.Relative] = fileState{
				Exists:   true,
				Content:  joinAddedContent(op.AddLines),
				EOL:      "\n",
				FileMode: toolcontract.DefaultFileMode,
			}
			changedPaths[resolvedPath.Relative] = struct{}{}
		case patchparse.OperationDelete:
			if !logical[resolvedPath.Relative].Exists {
				return nil, nil, nil, &toolcontract.PatchError{
					Kind:    "missing_file",
					Path:    resolvedPath.Relative,
					Message: "cannot delete a file that does not exist",
				}
			}
			logical[resolvedPath.Relative] = fileState{}
			changedPaths[resolvedPath.Relative] = struct{}{}
		case patchparse.OperationUpdate:
			currentState := logical[resolvedPath.Relative]
			if !currentState.Exists {
				return nil, nil, nil, &toolcontract.PatchError{
					Kind:    "missing_file",
					Path:    resolvedPath.Relative,
					Message: "cannot update a file that does not exist",
				}
			}
			updatedContent, err := applyHunks(currentState.Content, op.UpdateHunks, resolvedPath.Relative)
			if err != nil {
				return nil, nil, nil, err
			}
			if updatedContent == currentState.Content && destRelative == resolvedPath.Relative {
				return nil, nil, nil, &toolcontract.PatchError{
					Kind:    "no_op",
					Path:    resolvedPath.Relative,
					Message: "patch does not change the file",
				}
			}
			newState := currentState
			newState.Content = updatedContent
			if destRelative != resolvedPath.Relative {
				newState.Exists = true
				logical[resolvedPath.Relative] = fileState{}
				logical[destRelative] = newState
				changedPaths[resolvedPath.Relative] = struct{}{}
				changedPaths[destRelative] = struct{}{}
			} else {
				logical[resolvedPath.Relative] = newState
				changedPaths[resolvedPath.Relative] = struct{}{}
			}
		case patchparse.OperationUpdateOrAdd:
			currentState := logical[resolvedPath.Relative]
			if currentState.Exists {
				updatedContent, err := applyHunks(currentState.Content, op.UpdateHunks, resolvedPath.Relative)
				if err != nil {
					return nil, nil, nil, err
				}
				if updatedContent == currentState.Content {
					return nil, nil, nil, &toolcontract.PatchError{
						Kind:    "no_op",
						Path:    resolvedPath.Relative,
						Message: "patch does not change the file",
					}
				}
				newState := currentState
				newState.Content = updatedContent
				logical[resolvedPath.Relative] = newState
				changedPaths[resolvedPath.Relative] = struct{}{}
				continue
			}
			createdContent, err := buildCreatedContentFromHunks(op.UpdateHunks, resolvedPath.Relative)
			if err != nil {
				return nil, nil, nil, err
			}
			logical[resolvedPath.Relative] = fileState{
				Exists:   true,
				Content:  createdContent,
				EOL:      "\n",
				FileMode: toolcontract.DefaultFileMode,
			}
			changedPaths[resolvedPath.Relative] = struct{}{}
		default:
			return nil, nil, nil, fmt.Errorf("unsupported operation kind %q", op.Kind)
		}
	}

	if len(changedPaths) == 0 {
		return nil, nil, nil, &toolcontract.PatchError{
			Kind:    "no_op",
			Message: "patch does not change any files",
		}
	}
	return originals, logical, changedPaths, nil
}

func isDeleteAddReplacement(previous string, next patchparse.OperationKind) bool {
	return (previous == string(patchparse.OperationDelete) && next == patchparse.OperationAdd) ||
		(previous == string(patchparse.OperationAdd) && next == patchparse.OperationDelete)
}

func buildCreatedContentFromHunks(hunks []patchparse.Hunk, path string) (string, error) {
	createdLines := []string{}
	addedLines := 0
	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			switch line.Kind {
			case '-':
				return "", &toolcontract.PatchError{
					Kind:    "invalid_update_or_add_create",
					Path:    path,
					Message: "cannot use '-' lines when creating a missing file with Update Or Add File",
				}
			case ' ':
				createdLines = append(createdLines, line.Text)
			case '+':
				createdLines = append(createdLines, line.Text)
				addedLines++
			}
		}
	}
	if addedLines == 0 {
		return "", &toolcontract.PatchError{
			Kind:    "invalid_update_or_add_create",
			Path:    path,
			Message: "creating a missing file with Update Or Add File requires at least one '+' line",
		}
	}
	return joinAddedContent(createdLines), nil
}

func (e *Executor) readState(abs string) (fileState, error) {
	textFile, err := fsguard.ReadTextFile(abs)
	if err != nil {
		return fileState{}, err
	}
	if !textFile.Exists {
		return fileState{}, nil
	}
	return fileState{
		Exists:   true,
		Content:  textFile.Content,
		EOL:      textFile.EOL,
		FileMode: textFile.FileMode,
	}, nil
}

func (e *Executor) commit(originals map[string]fileState, final map[string]fileState, changedPaths map[string]struct{}) error {
	orderedPaths := setToSortedSlice(changedPaths)
	applied := []string{}
	for _, rel := range orderedPaths {
		resolved, err := e.root.Resolve(rel)
		if err != nil {
			_ = e.rollback(originals, applied)
			return err
		}
		state := final[rel]
		if state.Exists {
			if err := fsguard.WriteTextFile(resolved.Absolute, state.Content, state.EOL, state.FileMode); err != nil {
				_ = e.rollback(originals, applied)
				return &toolcontract.PatchError{
					Kind:    "commit_error",
					Path:    rel,
					Message: err.Error(),
				}
			}
		} else if err := os.Remove(resolved.Absolute); err != nil && !os.IsNotExist(err) {
			_ = e.rollback(originals, applied)
			return &toolcontract.PatchError{
				Kind:    "commit_error",
				Path:    rel,
				Message: err.Error(),
			}
		}
		applied = append(applied, rel)
	}
	return nil
}

func (e *Executor) rollback(originals map[string]fileState, applied []string) error {
	for i := len(applied) - 1; i >= 0; i-- {
		rel := applied[i]
		resolved, err := e.root.Resolve(rel)
		if err != nil {
			continue
		}
		original := originals[rel]
		if original.Exists {
			if err := fsguard.WriteTextFile(resolved.Absolute, original.Content, original.EOL, original.FileMode); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(resolved.Absolute); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func setToSortedSlice(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for path := range values {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func joinAddedContent(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func buildDisplayFiles(originals map[string]fileState, final map[string]fileState, changedPaths map[string]struct{}) []toolcontract.DisplayFile {
	files := make([]toolcontract.DisplayFile, 0, len(changedPaths))
	for _, rel := range setToSortedSlice(changedPaths) {
		original := originals[rel]
		next := final[rel]
		display := toolcontract.DisplayFile{Path: rel}
		if original.Exists {
			content := original.Content
			display.OriginalContent = &content
		}
		if next.Exists {
			content := next.Content
			display.NewContent = &content
		}
		files = append(files, display)
	}
	return files
}

func buildOperationPreviews(parsed *patchparse.Patch, originals map[string]fileState, final map[string]fileState) []toolcontract.OperationPreview {
	operations := make([]toolcontract.OperationPreview, 0, len(parsed.Operations))
	for _, op := range parsed.Operations {
		preview := toolcontract.OperationPreview{
			Kind: operationPreviewKind(op),
			Path: op.Path,
		}
		if op.MoveTo != "" {
			preview.ToPath = op.MoveTo
		}
		switch op.Kind {
		case patchparse.OperationAdd:
			preview.AddedLines = len(op.AddLines)
		case patchparse.OperationDelete:
			preview.RemovedLines = countContentLines(originals[op.Path].Content)
		case patchparse.OperationUpdate:
			added, removed := countHunkChanges(op.UpdateHunks)
			preview.AddedLines = added
			preview.RemovedLines = removed
			preview.ChangedLines = minInt(added, removed)
			if op.MoveTo != "" && len(op.UpdateHunks) == 0 {
				preview.ChangedLines = 0
				preview.AddedLines = 0
				preview.RemovedLines = 0
			}
			if op.MoveTo != "" && len(op.UpdateHunks) > 0 {
				if finalState, ok := final[op.MoveTo]; ok && finalState.Exists && preview.AddedLines == 0 && preview.RemovedLines == 0 {
					preview.AddedLines = countContentLines(finalState.Content)
				}
			}
		case patchparse.OperationUpdateOrAdd:
			if originals[op.Path].Exists {
				preview.Kind = "update"
				added, removed := countHunkChanges(op.UpdateHunks)
				preview.AddedLines = added
				preview.RemovedLines = removed
				preview.ChangedLines = minInt(added, removed)
			} else {
				preview.Kind = "add"
				if finalState, ok := final[op.Path]; ok && finalState.Exists {
					preview.AddedLines = countContentLines(finalState.Content)
				}
			}
		}
		operations = append(operations, preview)
	}
	return operations
}

func buildChangeStats(operations []toolcontract.OperationPreview, files int) *toolcontract.ChangeStats {
	stats := &toolcontract.ChangeStats{Files: files}
	for _, op := range operations {
		stats.AddedLines += op.AddedLines
		stats.RemovedLines += op.RemovedLines
		stats.ChangedLines += op.ChangedLines
		switch op.Kind {
		case "add":
			stats.CreatedFiles++
		case "delete":
			stats.DeletedFiles++
		case "rename":
			stats.RenamedFiles++
		case "update":
			stats.UpdatedFiles++
		case "rename_update":
			stats.RenamedFiles++
			stats.UpdatedFiles++
		}
	}
	return stats
}

func formatSummary(prefix string, files int, stats *toolcontract.ChangeStats) string {
	parts := []string{
		fmt.Sprintf("%s for %d file(s)", prefix, files),
		fmt.Sprintf("+%d -%d ~%d", stats.AddedLines, stats.RemovedLines, stats.ChangedLines),
	}
	if stats.CreatedFiles > 0 {
		parts = append(parts, fmt.Sprintf("created %d", stats.CreatedFiles))
	}
	if stats.UpdatedFiles > 0 {
		parts = append(parts, fmt.Sprintf("updated %d", stats.UpdatedFiles))
	}
	if stats.RenamedFiles > 0 {
		parts = append(parts, fmt.Sprintf("renamed %d", stats.RenamedFiles))
	}
	if stats.DeletedFiles > 0 {
		parts = append(parts, fmt.Sprintf("deleted %d", stats.DeletedFiles))
	}
	return strings.Join(parts, "; ")
}

func operationPreviewKind(op patchparse.FileOperation) string {
	switch op.Kind {
	case patchparse.OperationAdd:
		return "add"
	case patchparse.OperationDelete:
		return "delete"
	case patchparse.OperationUpdate:
		if op.MoveTo != "" && len(op.UpdateHunks) == 0 {
			return "rename"
		}
		if op.MoveTo != "" {
			return "rename_update"
		}
		return "update"
	default:
		return string(op.Kind)
	}
}

func countContentLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n")
}

func countHunkChanges(hunks []patchparse.Hunk) (int, int) {
	added := 0
	removed := 0
	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			switch line.Kind {
			case '+':
				added++
			case '-':
				removed++
			}
		}
	}
	return added, removed
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func summarizeOperations(operations []toolcontract.OperationPreview) string {
	if len(operations) == 0 {
		return ""
	}

	labels := make([]string, 0, minInt(len(operations), 3))
	limit := minInt(len(operations), 3)
	for _, op := range operations[:limit] {
		switch op.Kind {
		case "add":
			labels = append(labels, fmt.Sprintf("add %s", op.Path))
		case "delete":
			labels = append(labels, fmt.Sprintf("delete %s", op.Path))
		case "update":
			labels = append(labels, fmt.Sprintf("update %s", op.Path))
		case "rename":
			labels = append(labels, fmt.Sprintf("rename %s -> %s", op.Path, op.ToPath))
		case "rename_update":
			labels = append(labels, fmt.Sprintf("rename+update %s -> %s", op.Path, op.ToPath))
		default:
			labels = append(labels, fmt.Sprintf("%s %s", op.Kind, op.Path))
		}
	}
	if len(operations) > limit {
		labels = append(labels, fmt.Sprintf("+%d more", len(operations)-limit))
	}
	return strings.Join(labels, ", ")
}
