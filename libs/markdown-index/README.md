# markdown-index

Indexes a tree of markdown files into an in-memory index of documents and the
wikilink edges between them.

```go
import "github.com/dentropy/markdown-indexer/libs/markdown-index"
```

A Go module with no dependency on a database, a file watcher, or anything else
an indexing pass does not need.

```sh
go get github.com/dentropy/markdown-indexer/libs/markdown-index
```

The package is named `markdownindexer`, so everything is reached through that
name:

```go
idx, err := markdownindexer.Scan("./notes", markdownindexer.Options{})
```

Requires Go 1.27.

This is one module of the [markdown-indexer](../../README.md) repo. Its sibling
[markdown-indexer-cli](../markdown-indexer-cli) is a command line tool over
this package.

The vault is the source of truth; the index is derived. This package only ever
reads it.

## Vocabulary

| Term                | Meaning                                                                                                             |
| ------------------- | ------------------------------------------------------------------------------------------------------------------- |
| **Vault**           | A directory tree of markdown files. The source of truth for what exists.                                            |
| **Document**        | One markdown file as indexed: relative path, name, modification time, front matter (as JSON), front matter content address, raw markdown body, and UUID. |
| **Front matter**    | The YAML block at the top of a document, held in the index as JSON.                                                |
| **UUID**            | A document's identifier, read from front matter. Its identity, not its path.                                        |
| **Content address** | The IPLD hash (CIDv1, dag-json codec, sha2-256) of front matter or an edge.                                          |
| **Wikilink**        | A `[[target]]` reference in a document body.                                                                        |
| **Edge**            | One resolved wikilink: from UUID, to UUID, label, title. Keyed by content address.                                   |
| **Index**           | The in-memory collection of documents and edges derived from the vault.                                              |
| **Problem**         | One structural fault found while indexing or checking: the paths it concerns, and the error that caused it.            |

A document's identity is its UUID, not its path: a rename leaves the same
document in place with a new `RelativePath`.

## Usage

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

### Narrowing the index

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

### Broken files

A file whose front matter does not parse does not abort the scan. It becomes a
`Problem` on the index, so one pass surfaces every fault in the vault:

```go
if err := idx.Err(); err != nil {
	return err // "1 problem(s): bad.md: parse front matter: yaml: ..."
}
for _, p := range idx.Problems {
	fmt.Println(p.Path(), p.Err)
}
```

Match on fault kind against `p.Err`, not against `idx.Err()` — the aggregate
error flattens every problem into one message and does not wrap them:

```go
var broken *markdownindexer.BrokenFrontMatterError
for _, p := range idx.Problems {
	if errors.As(p.Err, &broken) {
		// malformed YAML, or front matter opened with --- and never closed
	}
}
```

`Problem` also carries `Paths`, which is more than one path wide when several
documents are involved, and an `Error()` method that renders as
`a.md, b.md: <cause>`.

A duplicate UUID is **not** a scan problem. Both documents index cleanly, so
nothing lands in `idx.Problems`; ask the index instead:

```go
for _, id := range idx.DuplicateUUIDs() {
	fmt.Println(id, idx.PathsForID(id))
}
```

`idx.ByUUID()` keys the documents by UUID, and documents with no UUID are keyed
by the empty string, which is how you find them.

### Checking a vault

`Check` reports the vault's structural problems without keeping documents or
edges: front matter that fails to parse, UUIDs shared across documents, and
front matter that is missing the ID key. One `Problem` per issue; an empty slice
means the vault is clean. Files with no front matter at all are tolerated and
not reported.

```go
problems, err := markdownindexer.Check("./notes", markdownindexer.Options{})
for _, p := range problems {
	fmt.Println(p.Error()) // "a.md, b.md: duplicate UUID 1111..."
}
```

Duplicates arrive here as `*markdownindexer.DuplicateUUIDError`, carrying the contested
`UUID` and every `Paths` holding it:

```go
var dup *markdownindexer.DuplicateUUIDError
if errors.As(p.Err, &dup) {
	fmt.Println(dup.UUID, dup.Paths)
}
```

A missing ID key is a bare `fmt.Errorf` wrapping `markdownindexer.ErrMissingUUID`, so
match it with `errors.Is`.

Note that `Check` takes `Options`, which means `Shared: true` narrows the scan
*before* the duplicate and missing-UUID checks run — private documents are
excluded and their problems go unreported. To check the whole vault, leave
`Shared` off.

### A single document

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

## What is not here

Mirroring. This package produces the index; persisting or querying it is the
caller's concern. A relational database, a search engine, a static site —
whatever consumes the index, it does so on its own terms.

Also absent: writing. Every entry point here only reads.

## Content addressing

Front matter and edges are hashed the same way: a CIDv1 over DAG-JSON, with a
sha2-256 multihash. An edge is *keyed* by its address in `Index.Edges`; front
matter *carries* its address in `Document.FrontmatterCID`.

Identical input always yields the same address, and DAG-JSON canonicalizes map
ordering, so two front matter blocks that differ only in key order share an
address. `RawMarkdownHash` is a plain hex sha256 of the body, not an IPLD
address — it identifies content, but it is not a CID and is not used as a key.

## Development

```sh
go test ./...
go doc .
```

The tests build their own vaults in `t.TempDir()`; there are no fixture files to
keep in sync.