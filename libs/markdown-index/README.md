# markdown-index

Indexes a tree of markdown files into an in-memory index of documents and the
wikilink edges between them.

## Use Case

> You have a vault of markdown notes full of `[[wikilinks]]`.  
> You want the documents and the links between them as data, not as a database.  
> Scan the vault and get a content-addressed index of every document and every
> resolved wikilink, then hand it to whatever consumes it.  
> The vault stays the source of truth: nothing here writes back.

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
| [JSON schema](docs/json-schema.md)  | The JSON shapes of documents and wikilinks, as `documents.schema.json` and `wikilinks.schema.json` |

## Development

```sh
go test ./...
go doc .
```

The tests build their own vaults in `t.TempDir()`; there are no fixture files to
keep in sync.
