package toolcontract

import "fmt"

type PatchError struct {
	Kind    string
	Line    int
	Path    string
	Message string
}

func (e *PatchError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("%s: %s", e.Kind, e.Message)
	}
	return fmt.Sprintf("%s (%s): %s", e.Kind, e.Path, e.Message)
}

func (e *PatchError) Diagnostic() Diagnostic {
	return Diagnostic{
		Kind:    e.Kind,
		Line:    e.Line,
		Path:    e.Path,
		Message: e.Message,
	}
}
