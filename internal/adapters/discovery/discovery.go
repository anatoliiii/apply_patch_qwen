package discovery

import (
	"encoding/json"
	"io"

	"apply_patch_qwen/internal/toolcontract"
)

func WriteDocument(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(toolcontract.DiscoveryDocument())
}
