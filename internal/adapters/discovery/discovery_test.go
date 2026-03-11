package discovery

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDocumentIncludesApplyPatch(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteDocument(&buf); err != nil {
		t.Fatalf("WriteDocument() error = %v", err)
	}
	if !strings.Contains(buf.String(), `"name": "apply_patch"`) {
		t.Fatalf("unexpected discovery output: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"name": "diff"`) {
		t.Fatalf("expected diff tool in discovery output: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "multiple file operations") {
		t.Fatalf("expected multi-file guidance in discovery output: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "Do not send unified diff headers like ---/+++") {
		t.Fatalf("expected strict patch guidance in discovery output: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "no absolute paths, no ~, and no .. segments") {
		t.Fatalf("expected workspace-root path guidance in discovery output: %s", buf.String())
	}
}

func TestExecuteAppliesPatch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	input := strings.NewReader(`{"patch":"*** Begin Patch\n*** Update File: a.txt\n@@\n one\n-two\n+three\n*** End Patch\n"}`)
	if err := Execute(root, "apply_patch", input, &out); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `"ok": true`) {
		t.Fatalf("unexpected response: %s", out.String())
	}
	if !strings.Contains(out.String(), `"display_files"`) {
		t.Fatalf("expected display_files in response: %s", out.String())
	}
	if !strings.Contains(out.String(), `"stats"`) {
		t.Fatalf("expected stats in response: %s", out.String())
	}
	if !strings.Contains(out.String(), `"operations"`) {
		t.Fatalf("expected operations in response: %s", out.String())
	}
}

func TestExecuteDiffPreview(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	input := strings.NewReader(`{"patch":"*** Begin Patch\n*** Update File: a.txt\n@@\n one\n-two\n+three\n*** End Patch\n"}`)
	if err := Execute(root, "diff", input, &out); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) {
		t.Fatalf("expected diff preview response: %s", out.String())
	}
	if !strings.Contains(out.String(), `"summary": "Previewed diff for 1 file(s); +1 -1 ~1; updated 1; update a.txt"`) {
		t.Fatalf("expected diff preview summary: %s", out.String())
	}
}
