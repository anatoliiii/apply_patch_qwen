package fsguard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"apply_patch_qwen/internal/toolcontract"
)

type Root struct {
	root     string
	rootReal string
}

type ResolvedPath struct {
	Relative string
	Absolute string
}

func New(root string) (*Root, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve root symlinks: %w", err)
	}
	return &Root{root: abs, rootReal: real}, nil
}

func (r *Root) Resolve(rel string) (ResolvedPath, error) {
	if rel == "" {
		return ResolvedPath{}, &toolcontract.PatchError{
			Kind:    "path_error",
			Message: "empty paths are not allowed",
		}
	}
	if filepath.IsAbs(rel) {
		return ResolvedPath{}, &toolcontract.PatchError{
			Kind:    "path_error",
			Path:    rel,
			Message: "absolute paths are not allowed",
		}
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return ResolvedPath{}, &toolcontract.PatchError{
			Kind:    "path_error",
			Path:    rel,
			Message: "path escapes the workspace root",
		}
	}
	abs := filepath.Join(r.root, clean)
	if err := r.ensureInsideRoot(abs); err != nil {
		return ResolvedPath{}, err
	}
	return ResolvedPath{
		Relative: filepath.ToSlash(clean),
		Absolute: abs,
	}, nil
}

func (r *Root) ensureInsideRoot(abs string) error {
	current := abs
	for {
		info, err := os.Lstat(current)
		if err == nil {
			realPath := current
			if info.Mode()&os.ModeSymlink != 0 || current != abs {
				realPath, err = filepath.EvalSymlinks(current)
				if err != nil {
					return &toolcontract.PatchError{
						Kind:    "path_error",
						Path:    abs,
						Message: fmt.Sprintf("resolve symlinks: %v", err),
					}
				}
			}
			if !hasPathPrefix(realPath, r.rootReal) {
				return &toolcontract.PatchError{
					Kind:    "path_error",
					Path:    abs,
					Message: "path resolves outside the workspace root",
				}
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return &toolcontract.PatchError{
				Kind:    "path_error",
				Path:    abs,
				Message: fmt.Sprintf("inspect path: %v", err),
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return &toolcontract.PatchError{
		Kind:    "path_error",
		Path:    abs,
		Message: "could not validate path within workspace root",
	}
}

func hasPathPrefix(pathValue, prefix string) bool {
	rel, err := filepath.Rel(prefix, pathValue)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
