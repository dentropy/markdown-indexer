// Package vault indexes a tree of markdown files into an in-memory index of
// documents and the wikilink edges between them.
//
// A vault is a directory tree of markdown files; it is the source of truth for
// what exists. A document is one markdown file as indexed: its relative path,
// name, modification time, front matter, front matter content address, raw
// markdown body, and the UUID read from its front matter. An edge is one
// resolved wikilink, keyed by its content address so identical edges dedupe.
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
