# markdown-indexer-cli

A command line tool over a markdown vault: it indexes the vault into documents
and a wikilink graph and prints them as JSON.

The indexing is not part of this module. It lives in the sibling
[`libs/markdown-index`](../markdown-index), whose package is named
`markdownindexer`. This module depends on it at `v0.0.1`.

```sh
go install github.com/dentropy/markdown-indexer/libs/markdown-indexer-cli@latest
```

That puts a `markdown-indexer-cli` binary on your `PATH`. Or from a clone:

```sh
go build -o markdown-indexer-cli .
```

Requires Go 1.27.

## Usage

```sh
./markdown-indexer-cli [flags]
```

The default operation writes the document index as JSON. `-wikilinks` writes the
wikilink graph instead, and `-vaultcheck` reports structural problems and exits
non-zero. Every operation is a one-shot scan; there is no daemon and no database.

```sh
./markdown-indexer-cli -dir examples -all              # document JSON list
./markdown-indexer-cli -dir examples -wikilinks        # wikilink graph JSON
./markdown-indexer-cli -dir testdata/vault -vaultcheck # structural problems
```

| Flag         | Default    | Description                                                    |
| ------------ | ---------- | -------------------------------------------------------------- |
| `-dir`       | `.`        | Directory to scan for markdown files                           |
| `-pattern`   | `**/*.md`  | Glob pattern, relative to `-dir` (supports `**` recursion)     |
| `-out`       | *(stdout)* | Output file for the JSON object                                |
| `-idkey`     | `uuid`     | YAML front matter key holding the document UUID                |
| `-all`       | `false`    | Load every document; by default only `share: true` documents   |
| `-wikilinks` | `false`    | Output the wikilink graph instead of the document index        |
| `-vaultcheck`| `false`    | Report broken front matter and duplicate/missing UUIDs          |
| `-checkdups` | `false`    | Report documents sharing a UUID, after writing the index       |
| `-stripdups` | `false`    | Assign a fresh UUID to every document sharing one (prompts)    |
| `-force`/`-F`| `false`    | Skip documents with broken front matter instead of aborting    |
| `-quiet`/`-q`| `false`    | Print only the JSON, with no progress notices on stderr        |
| `-memusage`  | `false`    | Print an estimate of the memory the document slice occupies    |

## Share filtering

By default the CLI loads only documents whose front matter sets `share: true`.
Pass `-all` to load the whole vault.

```sh
./markdown-indexer-cli -dir .        # only the shared half
./markdown-indexer-cli -dir . -all   # every document
```

`share` accepts a JSON boolean, a quoted string, or anything
`strconv.ParseBool` understands (`share: true`, `share: "true"`, `share: 1`,
`share: TRUE`). Anything else does not load.

This differs from the library, where `markdownindexer.Options{}.Shared` is **off** by
default. The CLI opts into the shared filter; the library does not.

The structural commands ignore the lens: `-vaultcheck` and `-stripdups` operate
on the whole vault, so CI still sees problems in documents that are not shared.

## Document JSON list

The index as a JSON object keyed by each document's UUID:

```json
{
  "123e4567-e89b-12d3-a456-426614174000": {
    "relativePath": "a/b/post.md",
    "name": "post",
    "modifiedUnix": 1789506535,
    "metadata": { "uuid": "123e4567-...", "title": "First Post" },
    "frontmatter_cid": "baguqeerahjiyqryrbbr2bua3ymeh3gftge4erhaho4shqzpcrjwm6rrbqcaa",
    "raw_markdown": "# First Post\n\nHello from the first markdown file.",
    "raw_markdown_hash": "afb00ef5983c7babc20d3eb45871aa371ab15814e96a9fdc2bf978d9944ba527"
  }
}
```

| Field                | Description                                                     |
| -------------------- | --------------------------------------------------------------- |
| `relativePath`       | Path relative to `-dir`, always forward slashes                  |
| `name`               | File name without the `.md` extension                            |
| `modifiedUnix`       | Last modified, as a unix timestamp in seconds                    |
| `metadata`           | Front matter parsed from YAML and re-encoded as JSON              |
| `frontmatter_cid`    | CIDv1 (dag-json, sha2-256) of the front matter serialized as DAG-JSON |
| `raw_markdown`       | File content with the front matter block stripped out             |
| `raw_markdown_hash`  | Hex sha256 of `raw_markdown`                                      |

A document's identity is its UUID, not its path, so a renamed file keeps the
same key. Two documents sharing a UUID collapse into one entry (the later in
path order wins), which is what `-checkdups` exists to catch. Files with no front
matter are still indexed: `metadata` is `{}` and they land under the `""` key.

Output is deterministic. Front matter and edges are content addressed with
DAG-JSON, which sorts map keys, so two runs over the same vault produce
byte-identical JSON.

## Wikilink JSON list

`-wikilinks` outputs a JSON object of graph edges, keyed by the edge's CID — the
same content addressing as `frontmatter_cid`, so identical `[[links]]` collapse
to a single entry:

```json
{
  "baguqeera26xs3d6ohtxpvjcnhrscxhhl6royph3herettrphcdiugzwidx2q": {
    "label": "INTERNAL",
    "title": "post",
    "from_document_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
    "to_document_id": "123e4567-e89b-12d3-a456-426614174000"
  }
}
```

| Field              | Description                                                                |
| ------------------ | -------------------------------------------------------------------------- |
| `label`            | `INTERNAL` or `WEBSITE`                                                    |
| `title`            | The pipe alias, or the raw target when there is none                       |
| `from_document_id` | UUID of the document holding the wikilink                                  |
| `to_document_id`   | The target's UUID when it resolves in the index, otherwise the raw target   |

Targets starting with `http://` or `https://` are labeled `WEBSITE`; everything
else is `INTERNAL`. Only documents with a non-empty UUID produce edges. Section
anchors (`[[target#section]]`) and aliases (`[[target|alias]]`) are both
understood.

## Checking the vault

`-vaultcheck` reports structural problems on stderr and exits non-zero, so it can
gate CI:

```sh
$ ./markdown-indexer-cli -dir testdata/vault -vaultcheck

broken-frontmatter.md: broken front matter: parse front matter: yaml: line 1: did not find expected ',' or ']'
duplicate UUID 77679f2b-... -> duplicate-a.md, duplicate-b.md
error: vault check found 2 problem(s)
```

It reports front matter that fails to parse, UUIDs shared across documents, and
front matter missing the `-idkey`. Files with no front matter at all are
tolerated. When nothing is wrong it prints `vault check: ok` and exits zero.

`-checkdups` reports duplicate UUIDs while still writing the index. `-stripdups`
goes further: it lists the colliding documents, asks for confirmation, and assigns
each one a fresh UUID, leaving the rest of every file's front matter untouched.
Anything but `y`/`yes` aborts without modifying a file.

## Broken front matter

By default the first file whose front matter fails to parse aborts the run and
names the file. Pass `-force` (or `-F`) to skip those files instead, logging one
line each and indexing the rest:

```sh
$ ./markdown-indexer-cli -dir testdata/vault

error: vault/broken-frontmatter.md: parse front matter: yaml: line 1: did not find expected ',' or ']'

$ ./markdown-indexer-cli -dir testdata/vault -force -all

skipping vault/broken-frontmatter.md: parse front matter: yaml: line 1: did not find expected ',' or ']'
```

`-force` also covers the output phase itself: a document that fails to serialize
is dropped and logged rather than aborting, so the JSON stays valid.

## Piping the JSON

Each skipped file is reported on stderr, so the JSON on stdout is already clean
and pipes into `jq` on its own. `-quiet` (or `-q`) drops those notices when you
would rather not see them:

```sh
$ markdown-indexer-cli -dir . -all -force | jq '.[] | .name'
$ markdown-indexer-cli -dir . -all -force -q | jq '.[] | .name'
```

`-quiet` suppresses progress notices only. A run that fails still says so on
stderr and still exits non-zero, and `-vaultcheck` still reports normally,
because there the report *is* the output.

## The library dependency

```go
require github.com/dentropy/markdown-indexer/libs/markdown-index v0.0.1
```

The package is named `markdownindexer`, so it is reached through that name:

```go
import "github.com/dentropy/markdown-indexer/libs/markdown-index"

idx, err := markdownindexer.Scan(dir, markdownindexer.Options{})
```

The library has no dependency on a database or a file watcher, which is why this
CLI's only other direct dependency is `google/uuid`.

## Test

This module has a dependency on its sibling, so build it in module mode from its
own directory:

```sh
go test ./...
```

Manual verification steps are in [MANUAL-TESTING.md](MANUAL-TESTING.md).

## Layout

```
main.go              flags, orchestration, share filtering, -stripdups, JSON encoding
main_test.go         the tests
testdata/ examples/  fixtures
MANUAL-TESTING.md    manual verification steps
```