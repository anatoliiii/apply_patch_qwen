package fsguard

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"apply_patch_qwen/internal/toolcontract"
)

type TextFile struct {
	Exists   bool
	Content  string
	EOL      string
	FileMode os.FileMode
}

func ReadTextFile(path string) (TextFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return TextFile{}, nil
		}
		return TextFile{}, fmt.Errorf("read %s: %w", path, err)
	}
	if bytes.ContainsRune(data, 0) {
		return TextFile{}, &toolcontract.PatchError{
			Kind:    "binary_file",
			Path:    path,
			Message: "binary files are not supported",
		}
	}
	if !utf8.Valid(data) {
		return TextFile{}, &toolcontract.PatchError{
			Kind:    "encoding_error",
			Path:    path,
			Message: "only UTF-8 text files are supported",
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return TextFile{}, fmt.Errorf("stat %s: %w", path, err)
	}
	eol := detectEOL(string(data))
	return TextFile{
		Exists:   true,
		Content:  normalizeText(string(data)),
		EOL:      eol,
		FileMode: info.Mode().Perm(),
	}, nil
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

func detectEOL(text string) string {
	if strings.Contains(text, "\r\n") {
		return "\r\n"
	}
	return "\n"
}
