package patchapply

import (
	"fmt"
	"strings"

	"apply_patch_qwen/internal/patchparse"
	"apply_patch_qwen/internal/toolcontract"
)

func applyHunks(content string, hunks []patchparse.Hunk, path string) (string, error) {
	if len(hunks) == 0 {
		return content, nil
	}
	lines, trailing := splitContent(content)
	var output []string
	cursor := 0

	for _, hunk := range hunks {
		oldLines, newLines := hunkOldAndNew(hunk)
		if len(oldLines) == 0 {
			return "", &toolcontract.PatchError{
				Kind:    "context_mismatch",
				Path:    path,
				Message: fmt.Sprintf("hunk %q has no stable context to match", hunk.Header),
			}
		}
		matches := findMatches(lines, oldLines, cursor)
		if len(matches) == 0 {
			mismatchDetail := describeContextMismatch(lines, oldLines, cursor)
			message := fmt.Sprintf("expected context for hunk %q was not found", hunk.Header)
			if mismatchDetail != "" {
				message += "; " + mismatchDetail
			}
			return "", &toolcontract.PatchError{
				Kind:    "context_mismatch",
				Path:    path,
				Message: message,
			}
		}
		if len(matches) > 1 {
			return "", &toolcontract.PatchError{
				Kind:    "ambiguous_context",
				Path:    path,
				Message: fmt.Sprintf("hunk %q matches multiple locations", hunk.Header),
			}
		}
		start := matches[0]
		output = append(output, lines[cursor:start]...)
		output = append(output, newLines...)
		cursor = start + len(oldLines)
	}

	output = append(output, lines[cursor:]...)
	return joinContent(output, trailing), nil
}

func splitContent(content string) ([]string, bool) {
	trailing := strings.HasSuffix(content, "\n")
	trimmed := strings.TrimSuffix(content, "\n")
	if trimmed == "" {
		return []string{}, trailing
	}
	return strings.Split(trimmed, "\n"), trailing
}

func joinContent(lines []string, trailing bool) string {
	if len(lines) == 0 {
		if trailing {
			return "\n"
		}
		return ""
	}
	content := strings.Join(lines, "\n")
	if trailing {
		return content + "\n"
	}
	return content
}

func hunkOldAndNew(hunk patchparse.Hunk) ([]string, []string) {
	var oldLines []string
	var newLines []string
	for _, line := range hunk.Lines {
		switch line.Kind {
		case ' ':
			oldLines = append(oldLines, line.Text)
			newLines = append(newLines, line.Text)
		case '-':
			oldLines = append(oldLines, line.Text)
		case '+':
			newLines = append(newLines, line.Text)
		}
	}
	return oldLines, newLines
}

func findMatches(lines []string, pattern []string, from int) []int {
	var matches []int
	if len(pattern) == 0 || len(pattern) > len(lines) {
		return matches
	}
	for i := from; i+len(pattern) <= len(lines); i++ {
		matched := true
		for j := range pattern {
			if lines[i+j] != pattern[j] {
				matched = false
				break
			}
		}
		if matched {
			matches = append(matches, i)
		}
	}
	return matches
}

func describeContextMismatch(lines []string, pattern []string, from int) string {
	best := bestMismatchWindow(lines, pattern, from)
	if best.expected == "" {
		return ""
	}
	if best.whitespaceOnly {
		return fmt.Sprintf("first differing line: expected %q but found %q (whitespace differs)", best.expected, best.got)
	}
	if best.got == "<EOF>" {
		return fmt.Sprintf("first differing line: expected %q but found end of file", best.expected)
	}
	return fmt.Sprintf("first differing line: expected %q but found %q", best.expected, best.got)
}

func normalizeWhitespaceHint(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

type mismatchWindow struct {
	expected       string
	got            string
	whitespaceOnly bool
	score          int
	distance       int
}

func bestMismatchWindow(lines []string, pattern []string, from int) mismatchWindow {
	if len(pattern) == 0 {
		return mismatchWindow{}
	}
	if from < 0 {
		from = 0
	}
	if len(lines) == 0 {
		return mismatchWindow{expected: pattern[0], got: "<EOF>"}
	}

	best := mismatchWindow{score: -1}
	for start := from; start < len(lines); start++ {
		candidate, ok := scoreMismatchWindow(lines, pattern, start, from)
		if !ok {
			continue
		}
		if candidate.score > best.score || (candidate.score == best.score && candidate.distance < best.distance) {
			best = candidate
		}
	}
	if best.score >= 0 {
		return best
	}
	if from < len(lines) {
		return mismatchWindow{expected: pattern[0], got: lines[from]}
	}
	return mismatchWindow{expected: pattern[0], got: "<EOF>"}
}

func scoreMismatchWindow(lines []string, pattern []string, start int, anchor int) (mismatchWindow, bool) {
	exactMatches := 0
	whitespaceMatches := 0
	var result mismatchWindow
	foundMismatch := false

	for i, expected := range pattern {
		lineIndex := start + i
		if lineIndex >= len(lines) {
			if !foundMismatch {
				result.expected = expected
				result.got = "<EOF>"
				result.whitespaceOnly = false
				foundMismatch = true
			}
			break
		}

		got := lines[lineIndex]
		switch {
		case got == expected:
			exactMatches++
		case normalizeWhitespaceHint(got) == normalizeWhitespaceHint(expected):
			whitespaceMatches++
			if !foundMismatch {
				result.expected = expected
				result.got = got
				result.whitespaceOnly = true
				foundMismatch = true
			}
		default:
			if !foundMismatch {
				result.expected = expected
				result.got = got
				result.whitespaceOnly = false
				foundMismatch = true
			}
		}
	}

	if !foundMismatch {
		return mismatchWindow{}, false
	}

	result.score = exactMatches*4 + whitespaceMatches*2
	result.distance = start - anchor
	if result.distance < 0 {
		result.distance = -result.distance
	}
	return result, true
}
