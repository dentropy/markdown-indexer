# markdown-indexer

Indexes a markdown vault into content-addressed documents and a wikilink edge
graph, rewrites front matter, and provides a command line tool over both.

The work is split into three Go modules under `libs/`: the indexer only reads,
the tools only write, and the CLI is a thin shell over both.

## The libs

| Module                                                    | Package          | What it is                                                          |
| --------------------------------------------------------- | ---------------- | ------------------------------------------------------------------- |
| [`libs/markdown-indexer`](libs/markdown-indexer)           | `markdownindexer` | The indexing. Read-only, with no CLI, no database, no watcher.      |
| [`libs/markdown-tools`](libs/markdown-tools)               | `markdowntools`   | Front matter writes: init, share permission, duplicate UUID repair. |
| [`libs/markdown-tools-cli`](libs/markdown-tools-cli)       | `main`            | A command line tool over both libraries. Prints JSON, writes, checks. |

The indexer is the substance; the tools import its index and write back to the
vault; the CLI adds flags, prompts, and output but no indexing or writing logic
of its own. Each module has its own README with the details.

## Install the CLI

```sh
go install github.com/dentropy/markdown-indexer/libs/markdown-tools-cli@latest
```

## Use the indexer

```sh
go get github.com/dentropy/markdown-indexer/libs/markdown-indexer
```

```go
import "github.com/dentropy/markdown-indexer/libs/markdown-indexer"

idx, err := markdownindexer.Scan("./notes", markdownindexer.Options{})
```

## Use the tools

```sh
go get github.com/dentropy/markdown-indexer/libs/markdown-tools
```

```go
import (
	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
	"github.com/dentropy/markdown-indexer/libs/markdown-tools"
)

idx, err := markdownindexer.Scan("./notes", markdownindexer.Options{})
if err != nil {
	return err
}
changed, err := markdowntools.InitFrontMatter(idx, "./notes", "uuid", nil)
```

## A note on the names

The **vault** is the domain term: a directory tree of markdown files, the source
of truth. It is defined in
[`libs/markdown-indexer/CONTEXT.md`](libs/markdown-indexer/CONTEXT.md) and it is
what the prose means throughout. The read-only **package** is named
`markdownindexer`, the write package is `markdowntools`, and the module
directory names match their import paths.

## Versions

Each module is tagged independently, with the tag prefixed by its directory:

```sh
go get github.com/dentropy/markdown-indexer/libs/markdown-tools-cli@libs/markdown-tools-cli/v0.1.0
```

## Test

Each module is tested on its own, from its own directory:

```sh
cd libs/markdown-indexer    && go test ./...
cd libs/markdown-tools      && go test ./...
cd libs/markdown-tools-cli  && go test ./...
```

`markdown-tools` and `markdown-tools-cli` use `replace` directives so their
sibling modules are read from this checkout during development. Those directives
are ignored when the modules are consumed as dependencies. There is no root
`go.mod`: this repository is a container for the three modules, not a module
itself.
