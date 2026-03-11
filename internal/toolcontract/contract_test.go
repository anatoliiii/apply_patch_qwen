package toolcontract

import (
	"strings"
	"testing"
)

func TestDecodeRequestAcceptsBooleanDryRun(t *testing.T) {
	req, err := DecodeRequest([]byte(`{"patch":"*** Begin Patch\n*** End Patch","dry_run":true}`))
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}
	if !req.DryRun {
		t.Fatal("expected dry_run to be true")
	}
}

func TestDecodeRequestAcceptsStringDryRun(t *testing.T) {
	req, err := DecodeRequest([]byte(`{"patch":"*** Begin Patch\n*** End Patch","dry_run":"true"}`))
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}
	if !req.DryRun {
		t.Fatal("expected dry_run to be true")
	}
}

func TestDecodeRequestRejectsInvalidStringDryRun(t *testing.T) {
	if _, err := DecodeRequest([]byte(`{"patch":"*** Begin Patch\n*** End Patch","dry_run":"sometimes"}`)); err == nil {
		t.Fatal("expected invalid boolean error")
	}
}

func TestFailureAddsHumanReadableSummaryForSingleDiagnostic(t *testing.T) {
	resp := Failure("Patch rejected.", Diagnostic{
		Kind:    "context_mismatch",
		Path:    "taskmaster/models.py",
		Message: "expected context for hunk \"@@\" was not found",
	})

	if resp.Summary != "Patch rejected; context mismatch in models.py." {
		t.Fatalf("unexpected summary: %q", resp.Summary)
	}
}

func TestFailureAddsValidExampleForFormatErrors(t *testing.T) {
	resp := Failure("Patch rejected.", Diagnostic{
		Kind:    "invalid_hunk_line_prefix",
		Line:    6,
		Message: "invalid hunk line prefix \"<\" at line 6",
	})

	if !strings.Contains(resp.Summary, "Valid example:\n*** Begin Patch") {
		t.Fatalf("expected valid example in summary, got: %q", resp.Summary)
	}
	if !strings.Contains(resp.Summary, "*** Update File: path/to/file.txt") {
		t.Fatalf("expected update example in summary, got: %q", resp.Summary)
	}
}

func TestFailureAddsHumanReadableSummaryForMultipleDiagnostics(t *testing.T) {
	resp := Failure(
		"Patch rejected.",
		Diagnostic{
			Kind:    "missing_file",
			Path:    "a.txt",
			Message: "cannot update a file that does not exist",
		},
		Diagnostic{
			Kind:    "commit_error",
			Path:    "b.py",
			Message: "update b.py not applied",
		},
	)

	if resp.Summary != "Patch rejected; missing file a.txt; update b.py not applied." {
		t.Fatalf("unexpected summary: %q", resp.Summary)
	}
}

func TestFailureSummarizesReplaceViaDeleteAdd(t *testing.T) {
	resp := Failure("Patch rejected.", Diagnostic{
		Kind:    "replace_via_delete_add",
		Path:    "a.txt",
		Message: "use Update File instead of Delete File plus Add File to modify an existing file",
	})

	if resp.Summary != "Patch rejected; replace via delete+add for a.txt." {
		t.Fatalf("unexpected summary: %q", resp.Summary)
	}
}
