# markdown-indexer

Indexes a tree of markdown files into an in-memory index of documents, the
wikilink edges between them, body embed edges, an edge for each inline `#tag`,
the wikilinks, markdown links, and `#tags` found in front matter, the derived
backlinks for each document, and a manifest of every document's title, UUID, and
path.

## Use Case

> You have a vault of markdown notes full of `[[wikilinks]]` and `#tags`.  
> You want the documents and the links between them as data, not as a database.  
> Scan the vault and get a content-addressed index of every document, every
> resolved wikilink, every body embed, every inline tag, every wikilink,
> markdown link, and `#tag` in front matter, and each document's incoming
> links, then hand it to whatever consumes it.  
> The vault stays the source of truth: nothing here writes back.

```go
import "github.com/dentropy/markdown-indexer/libs/markdown-indexer"
```

A Go module with no dependency on a database, a file watcher, or anything else
an indexing pass does not need.

```sh
go get github.com/dentropy/markdown-indexer/libs/markdown-indexer
```

The package is named `markdownindexer`, so everything is reached through that
name:

```go
idx, err := markdownindexer.Scan("./notes", markdownindexer.Options{})
```

Requires Go 1.27.

This is one module of the [markdown-indexer](../../README.md) repo. Its sibling
[markdown-tools-cli](../markdown-tools-cli) is a command line tool over this
package, and [markdown-tools](../markdown-tools) holds every write operation that
acts on an index.

The vault is the source of truth; the index is derived. This package only ever
reads it.

```go
backlinks := idx.Backlinks() // map[target UUID]Backlinks, derived on demand
```

Backlinks are the resolved body `INTERNAL` edges arriving at a document. They
are computed from the graph, not stored, and every document with a UUID gets an
entry, empty when nothing links to it. See [Usage](docs/usage.md#backlinks).

```go
manifest := idx.Manifest() // Manifest{Documents: []ManifestEntry{{Title, UUID, Path}, ...}}
```

A manifest is the vault's file directory in one value: every document with a
UUID, ordered by path, carrying only its title, UUID, and path. It is what you
publish when a consumer needs to turn an identifier into a readable name and
location without loading the full index. See
[JSON schema](docs/json-schema.md#json-schema) and
[`manifest.schema.json`](docs/manifest.schema.json).

## What is not here

Mirroring. This package produces the index; persisting or querying it is the
caller's concern. A relational database, a search engine, a static site —
whatever consumes the index, it does so on its own terms.

Also absent: writing. Every entry point here only reads.

## Documentation

| Document                            | What it covers                                                              |
| ----------------------------------- | ---------------------------------------------------------------------------- |
| [Vocabulary](docs/vocabulary.md)    | Vault, document, edge, index, problem — the terms the API uses               |
| [Usage](docs/usage.md)              | Scanning a vault, narrowing the index, indexing a single document            |
| [Problems](docs/problems.md)        | Broken files, duplicate UUIDs, and checking a vault                          |
| [Content addressing](docs/content-addressing.md) | How front matter and edges get their CIDs                           |
| [JSON schema](docs/json-schema.md)  | The JSON shapes of documents, wikilinks, backlinks, and the manifest, as `documents.schema.json`, `wikilinks.schema.json`, `backlinks.schema.json`, and `manifest.schema.json` |

## Development

```sh
go test ./...
go doc .
```

The tests build their own vaults in `t.TempDir()`; there are no fixture files to
keep in sync.
