package markdownindexer

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Edge is one resolved wikilink graph edge, carrying the UUIDs it connects.
// Identical edges share a content address and dedupe to a single entry, so the
// same vault always yields the same graph.
type Edge struct {
	// Label classifies the edge: INTERNAL, WEBSITE, or ASSET.
	Label string `json:"label"`

	// Title is the link's display text: the alias when the wikilink has one,
	// otherwise the target text.
	Title string `json:"title"`

	// FromDocumentID is the UUID of the document holding the wikilink.
	FromDocumentID string `json:"from_document_id"`

	// ToDocumentID is the UUID of the target document when the link resolves to
	// one in the index, and otherwise the raw target text.
	ToDocumentID string `json:"to_document_id"`
}

// edgeLabels classifies an edge from its target.
const (
	labelInternal = "INTERNAL"
	labelWebsite  = "WEBSITE"
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

// collectEdges scans every document for [[wikilinks]] and returns a graph edge
// for each link found. Documents without a UUID are skipped, since an edge has
// to start from a UUID to mean anything.
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
