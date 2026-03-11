package patchapply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"apply_patch_qwen/internal/toolcontract"
)

func TestGeneratePatchAutoAdd(t *testing.T) {
	root := t.TempDir()
	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.GeneratePatch(toolcontract.GeneratePatchRequest{
		Path:       "a.txt",
		OldContent: "",
		NewContent: "hello\nworld\n",
		Mode:       "auto",
	})
	if err != nil {
		t.Fatalf("GeneratePatch() error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !strings.Contains(resp.Patch, "*** Add File: a.txt") {
		t.Fatalf("unexpected patch: %q", resp.Patch)
	}
	if len(resp.DisplayFiles) != 1 || resp.DisplayFiles[0].Path != "a.txt" {
		t.Fatalf("expected display file preview: %+v", resp.DisplayFiles)
	}
	if resp.DisplayFiles[0].NewContent == nil || *resp.DisplayFiles[0].NewContent != "hello\nworld\n" {
		t.Fatalf("unexpected display preview: %+v", resp.DisplayFiles[0])
	}
}

func TestGeneratePatchAutoUpdateRoundTrips(t *testing.T) {
	root := t.TempDir()
	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resp, err := executor.GeneratePatch(toolcontract.GeneratePatchRequest{
		Path:       "a.txt",
		OldContent: "one\ntwo\n",
		NewContent: "one\nthree\n",
		Mode:       "auto",
	})
	if err != nil {
		t.Fatalf("GeneratePatch() error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("unexpected response: %+v", resp)
	}

	applyResp, err := executor.Apply(toolcontract.ApplyPatchRequest{
		Patch:  resp.Patch,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !applyResp.OK {
		t.Fatalf("generated patch should validate: %+v", applyResp)
	}
	if len(resp.DisplayFiles) != 1 || resp.DisplayFiles[0].OriginalContent == nil {
		t.Fatalf("expected display diff preview: %+v", resp.DisplayFiles)
	}
}

func TestGeneratePatchRejectsNoOp(t *testing.T) {
	root := t.TempDir()
	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.GeneratePatch(toolcontract.GeneratePatchRequest{
		Path:       "a.txt",
		OldContent: "same\n",
		NewContent: "same\n",
	})
	if err != nil {
		t.Fatalf("GeneratePatch() error = %v", err)
	}
	if resp.OK {
		t.Fatalf("expected failure response: %+v", resp)
	}
	if resp.Hint == "" {
		t.Fatalf("expected hint in response: %+v", resp)
	}
}

func TestGeneratePatchRejectsUpdateForEmptyOldContent(t *testing.T) {
	root := t.TempDir()
	executor, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := executor.GeneratePatch(toolcontract.GeneratePatchRequest{
		Path:       "a.txt",
		OldContent: "",
		NewContent: "hello\n",
		Mode:       "update",
	})
	if err != nil {
		t.Fatalf("GeneratePatch() error = %v", err)
	}
	if resp.OK {
		t.Fatalf("expected failure response: %+v", resp)
	}
	if len(resp.Diagnostics) != 1 || resp.Diagnostics[0].Kind != "missing_file" {
		t.Fatalf("unexpected diagnostics: %+v", resp.Diagnostics)
	}
}
