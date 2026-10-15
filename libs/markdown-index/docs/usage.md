# Usage

```go
idx, err := markdownindexer.Scan("./notes", markdownindexer.Options{})
if err != nil {
	return err
}

for _, doc := range idx.Documents {
	fmt.Println(doc.RelativePath, doc.ID)
}

for cid, edge := range idx.Edges {
	fmt.Println(cid, edge.FromDocumentID, "->", edge.ToDocumentID)
}
```

`Options{}` loads every markdown file at any depth and reads the UUID from the
`uuid` front matter key.

`Documents` is ordered by relative path. `Edges` is a map keyed by content
address, so identical `[[links]]` dedupe to one entry and iteration order is not
stable — read the CID when you need an address, not the loop position.

Each edge's `ToDocumentID` is the target document's UUID when a wikilink resolves
to a name in the index, and otherwise the raw target text. Only documents with a
non-empty UUID produce edges. Body wikilinks whose targets begin with a URL
scheme are labeled `URL`; every other body wikilink is `INTERNAL`. A scheme is
CommonMark's absolute-URI scheme: 2–32 characters, then `:`. `https://`,
`mailto:`, and `ipfs://` are URLs. A note title with a colon and a space
(`[[Design: the indexer]]`) stays `INTERNAL`, as does a drive path (`C:/Users`).

An inline `#tag` in the body becomes an edge labeled `TAG`. The tag text,
without the leading `#`, is both `Title` and `ToDocumentID`. A tag is not
resolved to a document. Headings (`# Title`), fenced and inline code, and
wikilink section anchors (`[[note#section]]`) do not produce tag edges.

The same shapes in front matter strings are separate edges, labeled
`FRONTMATTER_INTERNAL`, `FRONTMATTER_URL`, and `FRONTMATTER_TAG`. A `#name`
is read with the body tag parser. A `[[wikilink]]` uses the body rules, including
alias, section, and name resolution. An inline markdown link `[text](destination)`
is `FRONTMATTER_URL` when the destination begins with a URL scheme, and
`FRONTMATTER_INTERNAL` otherwise. Its destination is stored as written and is not
resolved to a UUID. Image embeds (`![](...)`), reference-style links, bare URL
fields, and a `tags` list of bare words do not produce edges. The same hit in
the body and in front matter stays two edges, because the label differs.

`Shared` applies to every edge, because the graph is built from the documents
that survive the filter.

## Narrowing the index

```go
idx, err := markdownindexer.Scan("./notes", markdownindexer.Options{
	Pattern: "journals/**/*.md",
	IDKey:   "id",
	Shared:  true,
})
```

`Shared: true` keeps only documents whose front matter sets `share: true`. It is
off by default: an index is the whole vault unless you ask for less. The value
may be a JSON boolean or a string `strconv.ParseBool` accepts, so `share: true`,
`share: TRUE`, `share: 1`, and `share: "true"` all count; anything else does not.

`Shared` narrows documents *and* edges — the graph is built from what survives
the filter.

Files that cannot be indexed do not abort a scan; they become problems instead.
See [Problems](problems.md).

## A single document

`Parse` indexes bytes you already hold, without touching the filesystem. Pass an
empty `idKey` to take the `uuid` default:

```go
doc, err := markdownindexer.Parse(data, "notes/idea.md", modTime, "")
if err != nil {
	return err
}
fmt.Println(doc.FrontMatter()["title"])
```

`FrontMatter()` returns a decoded map, empty when the document has no front
matter. `Parse` reads the file's modification time from the `time.Time` you pass
and stores it as `ModifiedUnix`.

Back to the [README](../README.md).
