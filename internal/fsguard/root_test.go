package fsguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRejectsEscape(t *testing.T) {
	root := t.TempDir()
	guard, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := guard.Resolve("../x.txt"); err == nil {
		t.Fatal("expected path escape error")
	}
}

func TestResolveRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	guard, err := New(root)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := guard.Resolve("link/file.txt"); err == nil {
		t.Fatal("expected symlink escape error")
	}
}

func TestReadTextFileRejectsBinary(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "binary.bin")
	if err := os.WriteFile(path, []byte{0x00, 0x01}, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := ReadTextFile(path); err == nil {
		t.Fatal("expected binary file error")
	}
}
