package markdownindexer

import "sort"

// ManifestEntry is one document in a vault's manifest: the title it displays
// as, the UUID that identifies it, and the path it lives at. It is the whole of
// what a consumer needs to join a published file back to a readable document.
type ManifestEntry struct {
	// Title is the document's display title: its front matter title when that
	// is a non-empty string, otherwise its file name. It matches
	// Document.DisplayName and a backlink's from_name.
	Title string `json:"title"`

	// UUID is the document's identifier, read from front matter under
	// Options.IDKey.
	UUID string `json:"uuid"`

	// Path is the document's path relative to the vault root, always with
	// forward slashes. It is Document.RelativePath.
	Path string `json:"path"`
}

// Manifest is the single-file directory of a vault: every document listed by
// title, UUID, and path, ordered by path. It is what
// manifest.schema.json describes.
//
// A manifest is derived, never stored. It is separate from the document index
// because it drops the bodies, front matter, and content addresses: a consumer
// that only needs to resolve an identifier to a readable name and location
// should not have to carry the vault with it.
//
// Documents without a UUID are omitted. A manifest entry is a way to find a
// document by identity, and a document with no identity has nothing to list.
type Manifest struct {
	// Documents holds one entry per identified document, ordered by path and
	// then UUID. It is an empty slice when the vault has none, so it never
	// encodes as null.
	Documents []ManifestEntry `json:"documents"`
}

// Manifest derives the vault's manifest, ordered by path. It reads
// Index.Documents; use BuildManifest for documents held somewhere else.
func (ix *Index) Manifest() Manifest {
	return BuildManifest(ix.Documents)
}

// BuildManifest lists the documents by title, UUID, and path, the same way
// Index.Manifest does without a vault walk. It exists for callers holding
// documents from somewhere else, such as a database read-back.
//
// Duplicate UUIDs are kept: the manifest describes the files that exist, so
// two documents claiming one identity appear as two entries. -check-duplicates
// is the tool for that fault.
func BuildManifest(docs []Document) Manifest {
	entries := make([]ManifestEntry, 0, len(docs))
	for _, d := range docs {
		if d.ID == "" {
			continue
		}
		entries = append(entries, ManifestEntry{
			Title: d.DisplayName(),
			UUID:  d.ID,
			Path:  d.RelativePath,
		})
	}

	// A scan already orders by path, but BuildManifest accepts documents from
	// anywhere, so the order is guaranteed here rather than assumed. The UUID
	// tie-break keeps a merged document set deterministic when two entries
	// share a path.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].UUID < entries[j].UUID
	})
	return Manifest{Documents: entries}
}
