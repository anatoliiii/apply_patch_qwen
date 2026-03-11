package fsguard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"apply_patch_qwen/internal/toolcontract"
)

func WriteTextFile(path string, content string, eol string, mode os.FileMode) error {
	if eol == "" {
		eol = "\n"
	}
	if mode == 0 {
		mode = toolcontract.DefaultFileMode
	}
	serialized := content
	if eol != "\n" {
		serialized = strings.ReplaceAll(content, "\n", eol)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".apply_patch_*")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(serialized); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w", path, err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("chmod temp %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp %s: %w", path, err)
	}
	return nil
}
