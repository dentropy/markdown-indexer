package markdownindexer

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Edge is one relationship leaving a document: a resolved wikilink or an inline
// tag. Identical edges share a content address and dedupe to a single entry, so
// the same vault always yields the same graph.
type Edge struct {
	// Label classifies the edge: INTERNAL, WEBSITE, or TAG.
	Label string `json:"label"`

	// Title is the link's display text: the alias when the wikilink has one,
	// otherwise the target text. For a tag it is the tag text without the
	// leading #.
	Title string `json:"title"`

	// FromDocumentID is the UUID of the document holding the wikilink or tag.
	FromDocumentID string `json:"from_document_id"`

	// ToDocumentID is the UUID of the target document when a wikilink resolves
	// to one in the index, and otherwise the raw target text. For a tag it is
	// the tag text. A tag does not resolve to a document.
	ToDocumentID string `json:"to_document_id"`
}

// edgeLabels classifies an edge from its target, or marks an inline tag.
const (
	labelInternal = "INTERNAL"
	labelWebsite  = "WEBSITE"
	labelTag      = "TAG"
)

// wikilinkRegex matches markdown wikilinks: [[target]], [[target#section]],
// [[target|alias]], and [[target#section|alias]].
var wikilinkRegex = regexp.MustCompile(`\[\[([^\]\|#]+)(?:#([^\]\|]+))?(?:\|([^\]]+))?\]\]`)

// EdgeCID returns the CIDv1 (dag-json codec, sha2-256) of an edge serialized as
// DAG-JSON, which is the key an edge is stored under.
func EdgeCID(e Edge) (string, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return dagJSONCID(data)
}

// ExtractWikilinks returns all wikilink targets found in markdown content.
// Each target is the raw text inside [[...]], with section anchors and pipe
// aliases stripped. Empty targets are omitted.
func ExtractWikilinks(content string) []string {
	matches := wikilinkRegex.FindAllStringSubmatch(content, -1)
	targets := make([]string, 0, len(matches))
	for _, m := range matches {
		target := strings.TrimSpace(m[1])
		if target != "" {
			targets = append(targets, target)
		}
	}
	return targets
}

// tagRegex matches an inline #tag. The predecessor is the start of a line or a
// character that does not continue a word, a wikilink section, or a URL
// fragment. Go's regexp is RE2 and has no lookbehind, so the predecessor is an
// ordinary group and is not captured. The body starts with a letter, number,
// underscore, or single-codepoint symbol, then continues with those plus
// hyphens and slashes.
var tagRegex = regexp.MustCompile(`(?m)(?:^|[^\p{L}\p{N}\p{So}_/])#([\p{L}\p{N}\p{So}_][\p{L}\p{N}\p{So}_/-]*)`)

// markdownTags returns the inline #tags in a markdown body, in order, with the
// leading # removed. The same spelling twice is returned once. An all-digit
// body (#1984) is not a tag; #y1984 is. Headings fail the match because a space
// follows the hashes. Fenced blocks and inline code are masked first. Front
// matter is not read: callers pass the body with it already stripped.
func markdownTags(content string) []string {
	matches := tagRegex.FindAllStringSubmatch(maskCode(content), -1)
	tags := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		tag := m[1]
		if allASCIIDigits(tag) {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}

func allASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// maskCode blanks fenced blocks and inline code so a # inside them is not a tag.
func maskCode(s string) string {
	return maskInlineCode(maskFences(s))
}

// maskFences blanks lines inside a fence of three or more backticks or tildes,
// including the fence lines. An unclosed fence blanks through the end.
func maskFences(s string) string {
	lines := strings.Split(s, "\n")
	var marker byte
	openLen := 0
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inFence {
			m, n, ok := openingFence(trimmed)
			if !ok {
				continue
			}
			inFence = true
			marker = m
			openLen = n
			lines[i] = ""
			continue
		}
		if closingFence(trimmed, marker, openLen) {
			inFence = false
		}
		lines[i] = ""
	}
	return strings.Join(lines, "\n")
}

func openingFence(trimmed string) (marker byte, n int, ok bool) {
	if len(trimmed) < 3 {
		return 0, 0, false
	}
	marker = trimmed[0]
	if marker != '`' && marker != '~' {
		return 0, 0, false
	}
	for n < len(trimmed) && trimmed[n] == marker {
		n++
	}
	if n < 3 {
		return 0, 0, false
	}
	// A backtick fence's info string cannot itself contain a backtick.
	if marker == '`' && strings.Contains(trimmed[n:], "`") {
		return 0, 0, false
	}
	return marker, n, true
}

func closingFence(trimmed string, marker byte, openLen int) bool {
	if len(trimmed) < openLen {
		return false
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != marker {
			return false
		}
	}
	return true
}

// maskInlineCode blanks `code` spans on each line. The closing run must be the
// same length as the opening run. An unclosed run leaves the rest of the line,
// so one stray backtick cannot swallow the line.
func maskInlineCode(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = maskInlineCodeLine(line)
	}
	return strings.Join(lines, "\n")
}

func maskInlineCodeLine(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	for i := 0; i < len(line); {
		if line[i] != '`' {
			b.WriteByte(line[i])
			i++
			continue
		}
		n := 1
		for i+n < len(line) && line[i+n] == '`' {
			n++
		}
		closeAt := inlineCodeClose(line, i+n, n)
		if closeAt < 0 {
			b.WriteString(line[i:])
			break
		}
		for k := i; k < closeAt; k++ {
			b.WriteByte(' ')
		}
		i = closeAt
	}
	return b.String()
}

func inlineCodeClose(line string, from, n int) int {
	for j := from; j < len(line); {
		if line[j] != '`' {
			j++
			continue
		}
		k := 1
		for j+k < len(line) && line[j+k] == '`' {
			k++
		}
		if k == n {
			return j + n
		}
		j += k
	}
	return -1
}

// collectEdges scans every document for [[wikilinks]] and inline #tags and
// returns a graph edge for each one found. Documents without a UUID are
// skipped, since an edge has to start from a UUID to mean anything.
func collectEdges(docs []Document) []Edge {
	nameToID := make(map[string]string, len(docs))
	for _, d := range docs {
		nameToID[d.Name] = d.ID
	}

	var edges []Edge
	for _, doc := range docs {
		if doc.ID == "" {
			continue
		}
		matches := wikilinkRegex.FindAllStringSubmatch(doc.RawMarkdown, -1)
		for _, m := range matches {
			target := strings.TrimSpace(m[1])
			if target == "" {
				continue
			}

			alias := ""
			if len(m) > 3 {
				alias = strings.TrimSpace(m[3])
			}
			title := alias
			if title == "" {
				title = target
			}

			label := labelInternal
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
				label = labelWebsite
			}

			toID := target
			if id, ok := nameToID[target]; ok {
				toID = id
			}

			edges = append(edges, Edge{
				Label:          label,
				Title:          title,
				FromDocumentID: doc.ID,
				ToDocumentID:   toID,
			})
		}
		for _, tag := range markdownTags(doc.RawMarkdown) {
			edges = append(edges, Edge{
				Label:          labelTag,
				Title:          tag,
				FromDocumentID: doc.ID,
				ToDocumentID:   tag,
			})
		}
	}
	return edges
}

// BuildEdges scans documents for [[wikilinks]] and returns the graph keyed by
// each edge's content address, the same way a Scan populates Index.Edges. It
// exists for callers holding documents from somewhere other than a vault walk,
// such as a database read-back.
func BuildEdges(docs []Document) map[string]Edge {
	return buildGraph(docs)
}

// buildGraph keys every collected edge by its content address. An Edge holds
// only plain strings, so neither its JSON encoding nor the DAG-JSON CID derived
// from it can fail in practice.
func buildGraph(docs []Document) map[string]Edge {
	graph := make(map[string]Edge)
	for _, e := range collectEdges(docs) {
		cid, err := EdgeCID(e)
		if err != nil {
			continue
		}
		graph[cid] = e
	}
	return graph
}
