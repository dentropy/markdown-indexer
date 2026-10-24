package markdowntools

import (
	"bytes"
	"errors"
)

const frontMatterDelimiter = "---"

// SetFrontMatterValue returns the file content with the value of the top-level
// YAML key `key` set to `value`, preserving all other bytes of the file. If the
// file has no front matter, a block holding the key is prepended; if the front
// matter has no such key, the key is appended to it.
func SetFrontMatterValue(data []byte, key, value string) ([]byte, error) {
	if !hasFrontMatter(data) {
		var out bytes.Buffer
		out.WriteString(frontMatterDelimiter)
		out.WriteByte('\n')
		out.WriteString(key)
		out.WriteString(": ")
		out.WriteString(value)
		out.WriteByte('\n')
		out.WriteString(frontMatterDelimiter)
		out.WriteString("\n\n")
		out.Write(data)
		return out.Bytes(), nil
	}

	body := data[len(frontMatterDelimiter):]
	yamlStart := len(frontMatterDelimiter)
	trimmed := bytes.TrimLeft(body, " \t")
	yamlStart += len(body) - len(trimmed)
	if len(trimmed) > 0 && (trimmed[0] == '\n' || trimmed[0] == '\r') {
		n := 1
		if trimmed[0] == '\r' && len(trimmed) > 1 && trimmed[1] == '\n' {
			n = 2
		}
		yamlStart += n
		trimmed = trimmed[n:]
	}

	end := findClosingDelimiter(trimmed)
	if end < 0 {
		return nil, errors.New("unterminated front matter: missing closing ---")
	}
	yamlEnd := yamlStart + end

	// The YAML block ends with the closing --- delimiter, so every line before
	// the last one is front-matter content.
	lines := bytes.Split(data[yamlStart:yamlEnd], []byte("\n"))
	if len(lines) == 0 {
		return data, nil
	}
	content, closing := lines[:len(lines)-1], lines[len(lines)-1]

	keyBytes := []byte(key)
	replaced := false
	var block bytes.Buffer
	for _, line := range content {
		if !replaced {
			if valStart, ok := matchingKeyValueStart(line, keyBytes); ok {
				block.Write(line[:valStart])
				block.WriteString(value)
				block.WriteByte('\n')
				replaced = true
				continue
			}
		}
		block.Write(line)
		block.WriteByte('\n')
	}
	if !replaced {
		block.WriteString(key)
		block.WriteString(": ")
		block.WriteString(value)
		block.WriteByte('\n')
	}
	block.Write(closing)

	var out bytes.Buffer
	out.Write(data[:yamlStart])
	out.Write(block.Bytes())
	out.Write(data[yamlEnd:])
	return out.Bytes(), nil
}

// hasFrontMatter reports whether the file content opens with a --- delimiter
// line.
func hasFrontMatter(data []byte) bool {
	if !bytes.HasPrefix(data, []byte(frontMatterDelimiter)) {
		return false
	}
	rest := data[len(frontMatterDelimiter):]
	if len(rest) == 0 {
		return false
	}
	c := rest[0]
	return c == '\n' || c == '\r' || c == ' ' || c == '\t'
}

// findClosingDelimiter returns the offset just past the closing --- line within
// body, or -1 when the block is never closed.
func findClosingDelimiter(body []byte) int {
	for _, line := range bytes.Split(body, []byte("\n")) {
		trimmed := bytes.TrimRight(line, " \t\r")
		if bytes.Equal(trimmed, []byte(frontMatterDelimiter)) {
			return bytes.Index(body, line) + len(line)
		}
	}
	return -1
}

// matchingKeyValueStart reports whether line is a top-level "<key>: value"
// front-matter entry (wrapping quotes around the key are tolerated) and
// returns the byte offset at which the value begins.
func matchingKeyValueStart(line, key []byte) (int, bool) {
	if len(line) == 0 || line[0] == ' ' || line[0] == '\t' || line[0] == '\r' || line[0] == '#' {
		return 0, false
	}
	rest := line
	quoted := false
	if rest[0] == '\'' || rest[0] == '"' {
		quoted = true
		rest = rest[1:]
	}
	if !bytes.HasPrefix(rest, key) {
		return 0, false
	}
	rest = rest[len(key):]
	if quoted {
		if len(rest) == 0 || (rest[0] != '\'' && rest[0] != '"') {
			return 0, false
		}
		rest = rest[1:]
	}
	i := 0
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t') {
		i++
	}
	if i >= len(rest) || rest[i] != ':' {
		return 0, false
	}
	rest = rest[i+1:]
	i = 0
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t') {
		i++
	}
	return len(line) - len(rest) + i, true
}
