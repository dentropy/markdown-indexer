package markdowntools

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
	"gopkg.in/yaml.v3"
)

// RepairOptions controls RepairFrontMatter.
type RepairOptions struct {
	// Pattern is the glob, relative to the vault root, that selects documents.
	// It defaults to markdownindexer.DefaultPattern.
	Pattern string

	// Apply writes each repaired front matter back to disk. When false,
	// RepairFrontMatter only reports the files it would change.
	Apply bool
}

// FrontMatterFix is one document that RepairFrontMatter quoted: the keys whose
// values were rewritten, in document order.
type FrontMatterFix struct {
	Path   string
	Fields []string
}

// FrontMatterProblem is one document RepairFrontMatter could not repair,
// together with the reason it gave up. Such a file is left byte-for-byte
// untouched.
type FrontMatterProblem struct {
	Path string
	Err  error
}

// RepairResult is what RepairFrontMatter found: the documents it repaired (or
// would repair when RepairOptions.Apply is false), and the documents it could
// not.
type RepairResult struct {
	Fixes        []FrontMatterFix
	Unrepairable []FrontMatterProblem
}

// RepairFrontMatter quotes plain YAML scalar values that contain a colon
// followed by a space, and closes unbalanced quotes in a single-line flow
// sequence (`tags: [ 'a', 'b ]`). Those are the usual reasons a vault's front
// matter fails to parse. A file is only rewritten when the whole front matter
// block parses afterwards; anything still broken is reported in Unrepairable
// and left untouched. Bodies and front matter delimiters are never modified.
//
// Unlike the index-based writers, this one walks the vault directly: a file
// whose front matter does not parse never reaches an index, so it is invisible
// to InitFrontMatter and ReplaceDuplicateUUIDs.
func RepairFrontMatter(dir string, opts RepairOptions) (RepairResult, error) {
	pattern := opts.Pattern
	if pattern == "" {
		pattern = markdownindexer.DefaultPattern
	}
	pattern = strings.TrimPrefix(strings.TrimPrefix(filepath.ToSlash(pattern), "./"), "./")

	var result RepairResult
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if !markdownindexer.MatchGlob(pattern, name) {
			return nil
		}

		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		out, fields, changed, err := repairFrontMatterData(data)
		if err != nil {
			result.Unrepairable = append(result.Unrepairable, FrontMatterProblem{Path: name, Err: err})
			return nil
		}
		if !changed {
			return nil
		}
		if opts.Apply {
			if err := os.WriteFile(p, out, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", name, err)
			}
		}
		result.Fixes = append(result.Fixes, FrontMatterFix{Path: name, Fields: fields})
		return nil
	})
	if err != nil {
		return RepairResult{}, err
	}
	sort.Slice(result.Fixes, func(i, j int) bool { return result.Fixes[i].Path < result.Fixes[j].Path })
	sort.Slice(result.Unrepairable, func(i, j int) bool { return result.Unrepairable[i].Path < result.Unrepairable[j].Path })
	return result, nil
}

// repairFrontMatterData quotes the offending plain scalars in one document and
// reports the keys it touched. The returned bytes are only different when the
// rewritten block parses as YAML; otherwise the error explains why not.
func repairFrontMatterData(data []byte) ([]byte, []string, bool, error) {
	start, end, ok, err := frontMatterBounds(data)
	if err != nil {
		return nil, nil, false, err
	}
	if !ok {
		return data, nil, false, nil
	}

	lines := strings.Split(string(data[start:end]), "\n")
	if len(lines) == 0 {
		return data, nil, false, nil
	}
	content, closing := lines[:len(lines)-1], lines[len(lines)-1]

	var parsed map[string]any
	origErr := yaml.Unmarshal([]byte(strings.Join(content, "\n")), &parsed)
	if origErr == nil {
		return data, nil, false, nil
	}
	return repairQuotedFrontMatter(data, start, end, content, closing, origErr)
}

func repairQuotedFrontMatter(data []byte, start, end int, content []string, closing string, origErr error) ([]byte, []string, bool, error) {
	fixed := make([]string, len(content))
	copy(fixed, content)

	var fields []string
	var lastValueIndent, blockIndent = -1, -1
	for i, line := range content {
		body, eol := strings.TrimSuffix(line, "\r"), ""
		if strings.HasSuffix(line, "\r") {
			eol = "\r"
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		indent := leadingIndent(body)

		if blockIndent >= 0 {
			if indent > blockIndent {
				continue
			}
			blockIndent = -1
		}
		// A line deeper than the last key's value continues that value, so it
		// is part of a multi-line scalar or a flow collection, not a new
		// mapping entry of its own.
		if lastValueIndent >= 0 && indent > lastValueIndent {
			continue
		}

		key, value, valueStart, ok := splitFrontMatterEntry(body)
		if !ok {
			lastValueIndent = -1
			continue
		}

		scalar, comment := splitFrontMatterComment(value)
		switch {
		case scalar == "":
			lastValueIndent = -1 // key with an empty value: a nested block follows
		case startsBlockScalar(scalar):
			blockIndent = indent
			lastValueIndent = indent
		default:
			lastValueIndent = indent
			repaired, keepComment, ok := repairFrontMatterValue(value, scalar)
			if !ok {
				break
			}
			if !keepComment {
				comment = ""
			}
			fixed[i] = body[:valueStart] + repaired + comment + eol
			fields = append(fields, key)
		}
	}

	if len(fields) == 0 {
		return nil, nil, false, fmt.Errorf("front matter does not parse: %w", origErr)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(strings.Join(fixed, "\n")), &parsed); err != nil {
		return nil, nil, false, fmt.Errorf("front matter still does not parse after quoting: %w", err)
	}

	block := closing
	if len(fixed) > 0 {
		block = strings.Join(fixed, "\n") + "\n" + closing
	}
	out := make([]byte, 0, len(data))
	out = append(out, data[:start]...)
	out = append(out, block...)
	out = append(out, data[end:]...)
	return out, fields, true, nil
}

// leadingIndent returns the number of leading spaces and tabs in line.
func leadingIndent(line string) int {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

// splitFrontMatterEntry splits one YAML mapping line into its key and raw
// value, tolerating indentation and a "- " sequence indicator. valueStart is
// the offset at which the value begins, so callers can rebuild the line without
// disturbing its indentation, key, or spacing. It reports ok=false for blank
// lines, comments, and lines that are not mappings.
func splitFrontMatterEntry(line string) (key, value string, valueStart int, ok bool) {
	i := leadingIndent(line)
	if i < len(line) && line[i] == '-' &&
		(i+1 == len(line) || line[i+1] == ' ' || line[i+1] == '\t') {
		i++
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
	}
	if i >= len(line) || line[i] == '#' {
		return "", "", 0, false
	}

	keyStart := i
	var quote byte
	for ; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case ':':
			if i+1 < len(line) && line[i+1] != ' ' && line[i+1] != '\t' {
				continue
			}
			key = strings.TrimSpace(line[keyStart:i])
			if key == "" {
				return "", "", 0, false
			}
			j := i + 1
			for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
				j++
			}
			return key, line[j:], j, true
		}
	}
	return "", "", 0, false
}

// splitFrontMatterComment separates a raw value from a trailing YAML comment.
// The scalar is trimmed; the comment keeps its leading whitespace and hash.
func splitFrontMatterComment(value string) (scalar, comment string) {
	for i := 0; i < len(value); i++ {
		if value[i] == '#' && (i == 0 || value[i-1] == ' ' || value[i-1] == '\t') {
			scalar := strings.TrimRight(value[:i], " \t")
			return scalar, value[len(scalar):]
		}
	}
	return strings.TrimRight(value, " \t"), ""
}

// repairFrontMatterValue rewrites one mapping value. Unbalanced quotes are
// closed on the raw value so a `#` inside the quote is not treated as a
// comment; colon-space quoting still uses the comment-stripped scalar.
func repairFrontMatterValue(value, scalar string) (repaired string, keepComment, ok bool) {
	if unclosedQuote(value) != 0 {
		closed, changed := closeUnbalancedQuotes(value)
		return closed, false, changed
	}
	if startsPlainScalar(scalar) && needsQuoting(scalar) {
		return quoteScalar(scalar), true, true
	}
	return "", false, false
}

// unclosedQuote returns the quote byte still open at the end of s, or 0 when
// quotes are balanced. YAML single quotes are escaped by doubling; double
// quotes use a backslash.
func unclosedQuote(s string) byte {
	var q byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch q {
		case '\'':
			if c != '\'' {
				continue
			}
			if i+1 < len(s) && s[i+1] == '\'' {
				i++
				continue
			}
			q = 0
		case '"':
			if c == '\\' {
				if i+1 < len(s) {
					i++
				}
				continue
			}
			if c == '"' {
				q = 0
			}
		default:
			if c == '\'' || c == '"' {
				q = c
			}
		}
	}
	return q
}

// closeUnbalancedQuotes inserts the missing closer. In a flow collection it
// goes before the last ] or } so `tags: [ 'a', 'b ]` becomes
// `tags: [ 'a', 'b' ]`; otherwise it is appended.
func closeUnbalancedQuotes(s string) (string, bool) {
	q := unclosedQuote(s)
	if q == 0 {
		return s, false
	}
	trimmed := strings.TrimLeft(s, " \t")
	if trimmed != "" && (trimmed[0] == '[' || trimmed[0] == '{') {
		closer := byte(']')
		if trimmed[0] == '{' {
			closer = '}'
		}
		if idx := strings.LastIndexByte(s, closer); idx >= 0 {
			insert := idx
			for insert > 0 && (s[insert-1] == ' ' || s[insert-1] == '\t') {
				insert--
			}
			return s[:insert] + string(q) + s[insert:], true
		}
	}
	return s + string(q), true
}

// startsPlainScalar reports whether value can be read as a plain scalar in the
// first place. Quoted scalars, flow collections, block scalars, anchors,
// aliases, tags, and reserved indicators are left alone.
func startsPlainScalar(value string) bool {
	if value == "" {
		return false
	}
	switch value[0] {
	case '"', '\'', '[', '{', '#', '&', '*', '!', '?', '@', '`', '%':
		return false
	case '-', ':':
		return len(value) == 1 || (value[1] != ' ' && value[1] != '\t')
	}
	return true
}

// startsBlockScalar reports whether value introduces a literal or folded block
// scalar, such as "|", ">-", or "|2".
func startsBlockScalar(value string) bool {
	return value != "" && (value[0] == '|' || value[0] == '>')
}

// needsQuoting reports whether value contains a colon-space sequence, which a
// YAML plain scalar may not contain. A trailing colon is included because the
// parser reads it as the start of another mapping value.
func needsQuoting(value string) bool {
	return strings.Contains(value, ": ") ||
		strings.Contains(value, ":\t") ||
		strings.HasSuffix(value, ":")
}

// quoteScalar wraps a plain scalar in double quotes, escaping the characters
// double quotes treat specially.
func quoteScalar(value string) string {
	var b strings.Builder
	b.Grow(len(value) + 2)
	b.WriteByte('"')
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteByte(value[i])
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(value[i])
		}
	}
	b.WriteByte('"')
	return b.String()
}
