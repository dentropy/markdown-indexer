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

Each edge's `ToDocumentID` is the target document's UUID when the link resolves
to a name in the index, and otherwise the raw target text. Only documents with a
non-empty UUID produce edges. Targets beginning `http://` or `https://` are
labeled `WEBSITE`; everything else is `INTERNAL`.

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
