package markdownindexer

import "sort"

// Backlink is one resolved body wikilink arriving at a document. It is one
// entry in that document's published backlinks file.
type Backlink struct {
	// FromDocumentID is the UUID of the source document, the one holding the
	// wikilink.
	FromDocumentID string `json:"from_document_id"`

	// FromName is the source document's display title: its front matter title
	// when it has one, otherwise its file name.
	FromName string `json:"from_name"`

	// FromRelativePath is the source document's path relative to the vault
	// root, always with forward slashes.
	FromRelativePath string `json:"from_relative_path"`

	// Title is the link's display text: the pipe alias when the wikilink has
	// one, otherwise the raw target text.
	Title string `json:"title"`

	// Label is always INTERNAL. Only resolved body wikilinks become
	// backlinks: a website link, a tag, an embed, an unresolved target, and a
	// front matter edge are all dropped.
	Label string `json:"label"`
}

// Backlinks is the incoming links for one document, the shape published at
// backlinks/<document_id>.json. Every document with a non-empty UUID has an
// entry, and Backlinks is an empty slice rather than nil when nothing links to
// it.
type Backlinks struct {
	// DocumentID is the UUID of the document these are backlinks for. It is
	// also the file name.
	DocumentID string `json:"document_id"`

	// Backlinks holds the incoming links, sorted by source title, then link
	// title, then source UUID.
	Backlinks []Backlink `json:"backlinks"`
}

// Backlinks derives the incoming links for every document in the index, keyed
// by the target document's UUID. It reads Index.Edges; use BuildBacklinks for
// documents held somewhere other than an index.
func (ix *Index) Backlinks() map[string]Backlinks {
	return buildBacklinks(ix.Documents, ix.Edges)
}

// BuildBacklinks derives the incoming links for every document from the
// documents alone, the same way BuildEdges builds a graph without a vault
// walk. It exists for callers holding documents from somewhere else, such as a
// database read-back.
func BuildBacklinks(docs []Document) map[string]Backlinks {
	return buildBacklinks(docs, buildGraph(docs))
}

// buildBacklinks keeps the resolved body INTERNAL edges whose source and
// target are both indexed documents with a UUID, drops self-links, and sorts
// each document's entries. Every indexed document gets an entry, so a document
// nothing links to still publishes an empty array.
func buildBacklinks(docs []Document, edges map[string]Edge) map[string]Backlinks {
	// Documents are ordered by relative path, so a duplicated UUID
	// deterministically resolves to the greatest path, matching Index.ByUUID.
	byID := make(map[string]Document, len(docs))
	for _, d := range docs {
		if d.ID == "" {
			continue
		}
		byID[d.ID] = d
	}

	out := make(map[string]Backlinks, len(byID))
	for id := range byID {
		out[id] = Backlinks{DocumentID: id, Backlinks: []Backlink{}}
	}
	if edges == nil {
		return out
	}

	for _, e := range edges {
		if e.Label != labelInternal || e.FromDocumentID == "" || e.ToDocumentID == "" {
			continue
		}
		if e.FromDocumentID == e.ToDocumentID {
			continue
		}
		from, ok := byID[e.FromDocumentID]
		if !ok {
			continue
		}
		entry, ok := out[e.ToDocumentID]
		if !ok {
			continue
		}
		entry.Backlinks = append(entry.Backlinks, Backlink{
			FromDocumentID:   e.FromDocumentID,
			FromName:         from.DisplayName(),
			FromRelativePath: from.RelativePath,
			Title:            e.Title,
			Label:            labelInternal,
		})
		out[e.ToDocumentID] = entry
	}

	for id, entry := range out {
		sort.SliceStable(entry.Backlinks, func(i, j int) bool {
			a, b := entry.Backlinks[i], entry.Backlinks[j]
			if a.FromName != b.FromName {
				return a.FromName < b.FromName
			}
			if a.Title != b.Title {
				return a.Title < b.Title
			}
			return a.FromDocumentID < b.FromDocumentID
		})
		out[id] = entry
	}
	return out
}
