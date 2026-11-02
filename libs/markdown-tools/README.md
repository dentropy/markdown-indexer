# markdown-tools

Writes derived output: front matter in a markdown vault, and backlinks JSON in a
publish directory. It is the write half of the
[markdown-indexer](../../README.md) repo: the sibling
[`markdown-indexer`](../markdown-indexer) only reads, and this package imports
its index to decide what to change.

```go
import (
	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
	"github.com/dentropy/markdown-indexer/libs/markdown-tools"
)
```

The package is named `markdowntools`.

```sh
go get github.com/dentropy/markdown-indexer/libs/markdown-tools
```

Requires Go 1.27.

## What it does

Scan with `markdownindexer`, then hand the resulting index to one of:

```go
func InitFrontMatter(ix *markdownindexer.Index, dir, idKey string, share *bool) ([]string, error)
func SetShare(ix *markdownindexer.Index, dir, idKey string, share bool) ([]string, error)
func ReplaceDuplicateUUIDs(ix *markdownindexer.Index, dir, idKey string) ([]string, error)
func WriteBacklinks(backlinks map[string]markdownindexer.Backlinks, dir string, opts BacklinksOptions) (written, removed []string, err error)
```

Each returns the relative paths of the files it actually rewrote. Nothing here
prints, prompts, or reads from stdin.

### Init missing front matter

```go
idx, err := markdownindexer.Scan("./notes", markdownindexer.Options{})
if err != nil {
	return err
}
changed, err := markdowntools.InitFrontMatter(idx, "./notes", "uuid", nil)
```

- A document without a UUID gets one under `idKey`.
- A document without a `share` key gets `share: false` when `share` is nil, or
  the pointed-to value when it is not.
- Existing UUIDs and share values are preserved.
- Files with no front matter at all get a new block.

Pass a non-nil bool to make missing share keys default to `true`:

```go
share := true
changed, err := markdowntools.InitFrontMatter(idx, "./notes", "uuid", &share)
```

### Set the share permission

```go
changed, err := markdowntools.SetShare(idx, "./notes", "uuid", false)
```

Every document gets `share` set to the requested value, and every document
without a UUID gets one. Existing UUIDs and all other front matter are kept. A
file whose share value already means the requested permission is not rewritten,
so repeating the call is safe.

### Repair duplicate UUIDs

```go
changed, err := markdowntools.ReplaceDuplicateUUIDs(idx, "./notes", "uuid")
```

Every document sharing a UUID gets a fresh one. Unique UUIDs are untouched.
This is the write behind the CLI's `-stripdups` flag; the CLI owns the
confirmation prompt.

### Publish backlinks

```go
backlinks := idx.Backlinks() // or markdownindexer.BuildBacklinks(docs)
written, removed, err := markdowntools.WriteBacklinks(
	backlinks,
	"./site/backlinks",
	markdowntools.BacklinksOptions{Prune: true},
)
```

One file per document, named `<document_id>.json`, matching
[`backlinks.schema.json`](../markdown-indexer/docs/backlinks.schema.json). The
map key is authoritative: it is both the file name and the `document_id` in the
file, and a nil slice becomes `"backlinks": []`.

Writes are atomic and skipped when the bytes do not change, so a repeat run is
cheap. With `Prune`, regular `*.json` files in the directory that do not belong
to the set are removed after the writes; subdirectories, symlinks, and other
files are left alone, and an empty set clears the directory. An ID that cannot
be a single file name (a path separator, `..`, a control character) fails the
call before anything is written: document IDs come from front matter, and one
must not be able to steer the writer outside the directory.

## Byte-level editing

When you already hold a file's bytes, `SetFrontMatterValue` performs the same
top-level YAML key upsert the vault functions use:

```go
data, err := markdowntools.SetFrontMatterValue(data, "share", "false")
```

It creates a front matter block when absent, replaces an existing key in place,
or appends a missing key before the closing `---`. All other bytes are
preserved. Unterminated front matter is an error rather than a silent no-op.

## Read/write split

This package never scans or indexes on its own. That keeps one source of truth
for what a vault contains (`markdownindexer.Scan`) and one place where writing
happens (here). Backlinks are computed by `markdownindexer` and handed over as a
map; this package only turns them into files. The CLI composes all of it.

## Test

```sh
go test ./...
```

Back to the [repository README](../../README.md).
