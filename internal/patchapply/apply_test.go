package patchapply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"apply_patch_qwen/internal/toolcontract"
)

func TestApplyUpdateMoveDelete(t *testing.T) {
	root := t.TempDir()
	original := "hello\nold\nworld\n"
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.Apply(toolcontract.ApplyPatchRequest{
		Patch: "*** Begin Patch\n*** Update File: a.txt\n*** Move to: b.txt\n@@\n hello\n-old\n+new\n*** Delete File: missing.txt\n*** End Patch\n",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if resp.OK {
		t.Fatal("expected failure because delete targets missing file")
	}
	if _, err := os.Stat(filepath.Join(root, "a.txt")); err != nil {
		t.Fatalf("expected original file to stay in place, err = %v", err)
	}
}

func TestApplyDryRunAddAndUpdate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.Apply(toolcontract.ApplyPatchRequest{
		DryRun: true,
		Patch:  "*** Begin Patch\n*** Update File: a.txt\n@@\n one\n-two\n+three\n*** Add File: b.txt\n+hello\n*** End Patch\n",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !resp.OK || !resp.DryRun {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(root, "b.txt")); !os.IsNotExist(err) {
		t.Fatalf("dry run should not create files, err = %v", err)
	}
}

func TestApplyCommitsChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.Apply(toolcontract.ApplyPatchRequest{
		Patch: "*** Begin Patch\n*** Update File: a.txt\n@@\n one\n-two\n+three\n*** Add File: nested/b.txt\n+hello\n*** End Patch\n",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("unexpected response: %+v", resp)
	}

	data, err := os.ReadFile(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "one\nthree\n" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestApplyCommitsEmptyAddedFile(t *testing.T) {
	root := t.TempDir()

	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.Apply(toolcontract.ApplyPatchRequest{
		Patch: "*** Begin Patch\n*** Add File: empty.txt\n*** End Patch\n",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("unexpected response: %+v", resp)
	}

	data, err := os.ReadFile(filepath.Join(root, "empty.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "" {
		t.Fatalf("expected empty file, got %q", string(data))
	}
}

func TestApplyRejectsDeleteAddReplacementForSamePath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.Apply(toolcontract.ApplyPatchRequest{
		Patch: "*** Begin Patch\n*** Delete File: a.txt\n*** Add File: a.txt\n+rewritten\n*** End Patch\n",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if resp.OK {
		t.Fatalf("expected failure response: %+v", resp)
	}
	if len(resp.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %+v", resp.Diagnostics)
	}
	if resp.Diagnostics[0].Kind != "replace_via_delete_add" {
		t.Fatalf("unexpected diagnostic kind: %+v", resp.Diagnostics[0])
	}
	if !strings.Contains(resp.Diagnostics[0].Message, "use Update File instead") {
		t.Fatalf("unexpected diagnostic message: %+v", resp.Diagnostics[0])
	}
}

func TestApplyPreservesSpecificParseErrorKind(t *testing.T) {
	root := t.TempDir()

	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.Apply(toolcontract.ApplyPatchRequest{
		Patch: "*** Begin Patch\n*** Rename File: a.txt\n*** End Patch\n",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if resp.OK {
		t.Fatalf("expected failure response: %+v", resp)
	}
	if len(resp.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %+v", resp.Diagnostics)
	}
	if resp.Diagnostics[0].Kind != "missing_move_target" {
		t.Fatalf("unexpected diagnostic kind: %+v", resp.Diagnostics[0])
	}
	if resp.Diagnostics[0].Line != 2 {
		t.Fatalf("unexpected diagnostic line: %+v", resp.Diagnostics[0])
	}
}

func TestApplyContextMismatchIncludesWhitespaceHint(t *testing.T) {
	root := t.TempDir()
	content := strings.Join([]string{
		"package main",
		"",
		"func main() {",
		"\tif part == \"\" {",
		"\t\treturn nil",
		"\t}",
		"}",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.Apply(toolcontract.ApplyPatchRequest{
		Patch: "*** Begin Patch\n*** Update File: a.txt\n@@\n func main() {\n     if part == \"\" {\n-\t\treturn nil\n+\t\tcontinue\n \t}\n }\n*** End Patch\n",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if resp.OK {
		t.Fatalf("expected failure response: %+v", resp)
	}
	if len(resp.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %+v", resp.Diagnostics)
	}
	if resp.Diagnostics[0].Kind != "context_mismatch" {
		t.Fatalf("unexpected diagnostic kind: %+v", resp.Diagnostics[0])
	}
	got := resp.Diagnostics[0].Message
	if !strings.Contains(got, "whitespace differs") || !strings.Contains(got, "expected") || !strings.Contains(got, "found") {
		t.Fatalf("expected whitespace hint in diagnostic message, got %q", got)
	}
	if !strings.Contains(got, `expected "    if part == \"\" {"`) {
		t.Fatalf("expected mismatch to point at the indented line, got %q", got)
	}
	if !strings.Contains(got, `found "\tif part == \"\" {"`) {
		t.Fatalf("expected mismatch to point at the tab-indented line, got %q", got)
	}
}

func TestApplyRenameSugarCommitsMove(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.Apply(toolcontract.ApplyPatchRequest{
		Patch: "*** Begin Patch\n*** Rename File: a.txt\n*** Move to: b.txt\n*** End Patch\n",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Stats == nil || resp.Stats.RenamedFiles != 1 {
		t.Fatalf("expected rename stats, got %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(root, "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected original file to be moved, err = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "b.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "one\ntwo\n" {
		t.Fatalf("unexpected moved file content: %q", string(data))
	}
}

func TestDiffReturnsPreviewWithoutWriting(t *testing.T) {
	root := t.TempDir()

	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.Diff(toolcontract.ApplyPatchRequest{
		Patch: "*** Begin Patch\n*** Add File: preview.txt\n+hello\n*** End Patch\n",
	})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if !resp.OK || !resp.DryRun {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Stats == nil || resp.Stats.CreatedFiles != 1 || resp.Stats.AddedLines != 1 {
		t.Fatalf("unexpected stats: %+v", resp)
	}
	if resp.Summary != "Previewed diff for 1 file(s); +1 -0 ~0; created 1; add preview.txt" {
		t.Fatalf("unexpected summary: %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(root, "preview.txt")); !os.IsNotExist(err) {
		t.Fatalf("diff preview should not create files, err = %v", err)
	}
}

func TestApplySummaryIncludesOperationLabels(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.Apply(toolcontract.ApplyPatchRequest{
		Patch: "*** Begin Patch\n*** Update File: a.txt\n@@\n one\n-two\n+three\n*** Add File: b.txt\n+hello\n*** End Patch\n",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Summary != "Applied patch for 2 file(s); +2 -1 ~1; created 1; updated 1; update a.txt, add b.txt" {
		t.Fatalf("unexpected summary: %q", resp.Summary)
	}
}

func TestSummarizeOperationsCapsOutput(t *testing.T) {
	summary := summarizeOperations([]toolcontract.OperationPreview{
		{Kind: "add", Path: "a.txt"},
		{Kind: "update", Path: "b.txt"},
		{Kind: "rename", Path: "old.txt", ToPath: "new.txt"},
		{Kind: "delete", Path: "c.txt"},
	})
	if summary != "add a.txt, update b.txt, rename old.txt -> new.txt, +1 more" {
		t.Fatalf("unexpected summary: %q", summary)
	}
}
