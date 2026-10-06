# markdown-indexer

Indexes a markdown vault into content-addressed documents and a wikilink edge
graph, and a command line tool that prints that index as JSON.

Both live here as separate Go modules under `libs/`.

## The libs

| Module                                                    | Package          | What it is                                                          |
| --------------------------------------------------------- | ---------------- | ------------------------------------------------------------------- |
| [`libs/markdown-index`](libs/markdown-index)               | `markdownindexer` | The indexing. A library with no CLI, no database, no watcher.       |
| [`libs/markdown-indexer-cli`](libs/markdown-indexer-cli)   | `main`           | A command line tool over that library. Prints JSON, checks vaults.  |

The library is the substance; the CLI is a thin shell around it and adds no
indexing logic of its own. Each has its own README with the details.

## Install the CLI

```sh
go install github.com/dentropy/markdown-indexer/libs/markdown-indexer-cli@latest
```

## Use the library

```sh
go get github.com/dentropy/markdown-indexer/libs/markdown-index
```

```go
import "github.com/dentropy/markdown-indexer/libs/markdown-index"

idx, err := markdownindexer.Scan("./notes", markdownindexer.Options{})
```

## A note on the two names

The **vault** is the domain term: a directory tree of markdown files, the source
of truth. It is defined in
[`libs/markdown-index/CONTEXT.md`](libs/markdown-index/CONTEXT.md) and it is what
the prose means throughout. The **package** is named `markdownindexer`, matching
the module it lives in, so that `markdownindexer.Scan` lines up with the import
path. Those are two different things that used to share a name.

## Versions

Each module is tagged independently, with the tag prefixed by its directory:

```sh
go get github.com/dentropy/markdown-indexer/libs/markdown-indexer-cli@libs/markdown-indexer-cli/v0.1.0
```

## Test

Each module is tested on its own, from its own directory:

```sh
cd libs/markdown-index        && go test ./...
cd libs/markdown-indexer-cli  && go test ./...
```

The CLI module depends on the library module, so building it needs the library
at a published version. There is no root `go.mod`: this repository is a container
for the two modules, not a module itself.