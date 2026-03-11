package patchparse

type OperationKind string

const (
	OperationUpdate OperationKind = "update"
	OperationAdd    OperationKind = "add"
	OperationDelete OperationKind = "delete"
)

type Patch struct {
	Operations []FileOperation
}

type FileOperation struct {
	Kind        OperationKind
	Path        string
	MoveTo      string
	UpdateHunks []Hunk
	AddLines    []string
}

type Hunk struct {
	Header string
	Lines  []HunkLine
}

type HunkLine struct {
	Kind byte
	Text string
}
