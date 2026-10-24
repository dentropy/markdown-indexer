// Package markdownindexer indexes a tree of markdown files into an in-memory index of
// documents, the wikilink edges between them, an edge for each inline tag, and
// the wikilinks, markdown links, and #tags found in front matter.
//
// A vault is a directory tree of markdown files; it is the source of truth for
// what exists. A document is one markdown file as indexed: its relative path,
// name, modification time, front matter, front matter content address, raw
// markdown body, and the UUID read from its front matter. An edge is one
// relationship leaving a document: a resolved wikilink, an inline #tag, or the
// same kind of reference found in a front matter string. Front matter edges
// use their own labels. Identical edges dedupe by content address.
//
// The index is derived. It is a one-way read of the vault: this package never
// writes to it.
//
//	idx, err := markdownindexer.Scan("./notes", markdownindexer.Options{})
//	if err != nil {
//		return err
//	}
//	for _, doc := range idx.Documents {
//		fmt.Println(doc.RelativePath, doc.ID)
//	}
//	for cid, edge := range idx.Edges {
//		fmt.Println(cid, edge.FromDocumentID, edge.ToDocumentID)
//	}
//
// Nothing here knows about persistence. Mirroring an index somewhere queryable
// is the caller's concern.
package markdownindexer
