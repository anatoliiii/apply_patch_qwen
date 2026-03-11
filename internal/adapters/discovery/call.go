package discovery

import (
	"encoding/json"
	"fmt"
	"io"

	"apply_patch_qwen/internal/patchapply"
	"apply_patch_qwen/internal/toolcontract"
)

func Execute(root string, toolName string, input io.Reader, output io.Writer) error {
	payload, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	req, err := toolcontract.DecodeRequest(payload)
	if err != nil {
		return err
	}
	executor, err := patchapply.New(root)
	if err != nil {
		return err
	}
	var resp toolcontract.ApplyPatchResponse
	switch toolName {
	case toolcontract.ToolNameApplyPatch:
		resp, err = executor.Apply(req)
	case toolcontract.ToolNameDiff:
		resp, err = executor.Diff(req)
	default:
		return fmt.Errorf("unsupported tool %q", toolName)
	}
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resp)
}
