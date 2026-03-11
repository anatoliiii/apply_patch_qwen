package patchparse

import (
	"testing"

	"apply_patch_qwen/internal/toolcontract"
)

func TestParseUpdateMoveAndHunks(t *testing.T) {
	patch := "*** Begin Patch\n*** Update File: a.txt\n*** Move to: b.txt\n@@\n old\n-new\n+new\n*** End Patch\n"

	parsed, err := Parse(patch)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(parsed.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(parsed.Operations))
	}
	op := parsed.Operations[0]
	if op.Path != "a.txt" || op.MoveTo != "b.txt" {
		t.Fatalf("unexpected op paths: %+v", op)
	}
	if len(op.UpdateHunks) != 1 || len(op.UpdateHunks[0].Lines) != 3 {
		t.Fatalf("unexpected hunks: %+v", op.UpdateHunks)
	}
}

func TestParseRenameSugar(t *testing.T) {
	patch := "*** Begin Patch\n*** Rename File: a.txt\n*** Move to: b.txt\n*** End Patch\n"

	parsed, err := Parse(patch)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(parsed.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(parsed.Operations))
	}
	op := parsed.Operations[0]
	if op.Kind != OperationUpdate || op.Path != "a.txt" || op.MoveTo != "b.txt" {
		t.Fatalf("unexpected op: %+v", op)
	}
	if len(op.UpdateHunks) != 0 {
		t.Fatalf("expected rename-only operation, got hunks: %+v", op.UpdateHunks)
	}
}

func TestParseAddRejectsNonPatchLines(t *testing.T) {
	patch := "*** Begin Patch\n*** Add File: a.txt\ntext\n*** End Patch\n"
	_, err := Parse(patch)
	if err == nil {
		t.Fatal("expected parse error")
	}
	patchErr, ok := err.(*toolcontract.PatchError)
	if !ok {
		t.Fatalf("expected PatchError, got %T", err)
	}
	if patchErr.Kind != "invalid_add_line" {
		t.Fatalf("unexpected error kind: %+v", patchErr)
	}
	if patchErr.Line != 3 {
		t.Fatalf("unexpected error line: %+v", patchErr)
	}
	if patchErr.Message != "add file expects '+' lines, got \"text\" at line 3" {
		t.Fatalf("unexpected error message: %q", patchErr.Message)
	}
}

func TestParseAddAllowsEmptyFile(t *testing.T) {
	patch := "*** Begin Patch\n*** Add File: empty.txt\n*** End Patch\n"

	parsed, err := Parse(patch)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(parsed.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(parsed.Operations))
	}
	op := parsed.Operations[0]
	if op.Kind != OperationAdd || op.Path != "empty.txt" {
		t.Fatalf("unexpected op: %+v", op)
	}
	if len(op.AddLines) != 0 {
		t.Fatalf("expected empty file add, got lines: %+v", op.AddLines)
	}
}

func TestParseRejectsTrailingContent(t *testing.T) {
	patch := "*** Begin Patch\n*** Delete File: a.txt\n*** End Patch\nextra\n"
	_, err := Parse(patch)
	if err == nil {
		t.Fatal("expected trailing content error")
	}
	patchErr, ok := err.(*toolcontract.PatchError)
	if !ok {
		t.Fatalf("expected PatchError, got %T", err)
	}
	if patchErr.Kind != "missing_end_patch" {
		t.Fatalf("unexpected error kind: %+v", patchErr)
	}
	if patchErr.Line != 4 {
		t.Fatalf("unexpected error line: %+v", patchErr)
	}
}

func TestParseRejectsRenameWithoutTargetWithLine(t *testing.T) {
	patch := "*** Begin Patch\n*** Rename File: a.txt\n*** End Patch\n"
	_, err := Parse(patch)
	if err == nil {
		t.Fatal("expected parse error")
	}
	patchErr, ok := err.(*toolcontract.PatchError)
	if !ok {
		t.Fatalf("expected PatchError, got %T", err)
	}
	if patchErr.Kind != "missing_move_target" {
		t.Fatalf("unexpected error kind: %+v", patchErr)
	}
	if patchErr.Line != 2 {
		t.Fatalf("unexpected error line: %+v", patchErr)
	}
	if patchErr.Message != "rename file \"a.txt\" must be followed by \"*** Move to: \" at line 2" {
		t.Fatalf("unexpected error message: %q", patchErr.Message)
	}
}

func TestParseRejectsInvalidHunkPrefixWithLine(t *testing.T) {
	patch := "*** Begin Patch\n*** Update File: a.txt\n@@\n!bad\n*** End Patch\n"
	_, err := Parse(patch)
	if err == nil {
		t.Fatal("expected parse error")
	}
	patchErr, ok := err.(*toolcontract.PatchError)
	if !ok {
		t.Fatalf("expected PatchError, got %T", err)
	}
	if patchErr.Kind != "invalid_hunk_line_prefix" {
		t.Fatalf("unexpected error kind: %+v", patchErr)
	}
	if patchErr.Line != 4 {
		t.Fatalf("unexpected error line: %+v", patchErr)
	}
	if patchErr.Message != "invalid hunk line prefix \"!\" at line 4" {
		t.Fatalf("unexpected error message: %q", patchErr.Message)
	}
}
