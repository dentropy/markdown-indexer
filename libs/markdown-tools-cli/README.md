# markdown-tools-cli

A command line tool over a markdown vault: it indexes the vault into documents
and a wikilink graph, prints them as JSON, and applies front matter fixes.

The indexing is not part of this module. It lives in the sibling
[`libs/markdown-indexer`](../markdown-indexer), whose package is named
`markdownindexer`. The writes live in [`libs/markdown-tools`](../markdown-tools),
whose package is named `markdowntools`. This CLI composes both; it adds flags,
prompts, and output, not indexing or rewriting logic.

```sh
go install github.com/dentropy/markdown-indexer/libs/markdown-tools-cli@latest
```

That puts a `markdown-tools-cli` binary on your `PATH`. Or from a clone:

```sh
go build -o markdown-tools-cli .
```

Requires Go 1.27.

## Usage

```sh
./markdown-tools-cli [flags]
```

The default operation writes the document index as JSON. `-wikilinks` writes the
wikilink graph instead, and `-validate` reports structural problems and exits
non-zero. Every operation is a one-shot scan; there is no daemon and no database.

```sh
./markdown-tools-cli -dir examples -include-unshared              # document JSON list
./markdown-tools-cli -dir examples -wikilinks        # wikilink graph JSON
./markdown-tools-cli -dir testdata/vault -validate # structural problems
```

``` bash
# For Example
export VAULT_DIR=`~/Vault`

# Add frontmatter to all the markdown documents so they have a UUID
./markdown-tools-cli -dir $VAULT_DIR -initfrontmatter

# May require -f for force
./markdown-tools-cli -dir $VAULT_DIR -initfrontmatter -f

# Set all documents to in the directory to be shared
./markdown-tools-cli -dir $VAULT_DIR share true

# Get the markdown directory as a JSON Struct
./markdown-tools-cli -dir $VAULT_DIR

# Get the wikilinks
./markdown-tools-cli -dir $VAULT_DIR -wikilinks

# Get the backlinks
./markdown-tools-cli -dir $VAULT_DIR -backlinks

```

| Flag         | Default    | Description                                                    |
| ------------ | ---------- | -------------------------------------------------------------- |
| `-dir`       | `.`        | Directory to scan for markdown files                           |
| `-glob`   | `**/*.md`  | Glob pattern, relative to `-dir` (supports `**` recursion)     |
| `-out`       | *(stdout)* | Output file for the JSON object                                |
| `-idkey`     | `uuid`     | YAML front matter key holding the document UUID                |
| `-include-unshared`       | `false`    | Load every document; by default only `share: true` documents   |
| `-wikilinks` | `false`    | Output the wikilink graph instead of the document index        |
| `-validate`| `false`    | Report broken front matter and duplicate/missing UUIDs          |
| `-check-duplicates` | `false`    | Report documents sharing a UUID, after writing the index       |
| `-fix-duplicates` | `false`    | Assign a fresh UUID to every document sharing one (prompts)    |
| `-initfrontmatter` | `false` | Add missing `uuid` and `share` keys; `share` defaults to `false` |
| `-setshare`  | `false`    | Set `share` on every matched document and add missing UUIDs; requires `-share` |
| `-share`     | *(unset)*  | `true` or `false`, used by `-initfrontmatter` and `-setshare`  |
| `-force`/`-F`| `false`    | Skip documents with broken front matter instead of aborting    |
| `-quiet`/`-q`| `false`    | Print only the JSON, with no progress notices on stderr        |
| `-memusage`  | `false`    | Print an estimate of the memory the document slice occupies    |

## Share filtering

By default the CLI loads only documents whose front matter sets `share: true`.
Pass `-include-unshared` to load the whole vault.

```sh
./markdown-tools-cli -dir .        # only the shared half
./markdown-tools-cli -dir . -include-unshared   # every document
```

`share` accepts a JSON boolean, a quoted string, or anything
`strconv.ParseBool` understands (`share: true`, `share: "true"`, `share: 1`,
`share: TRUE`). Anything else does not load.

This differs from the library, where `markdownindexer.Options{}.Shared` is **off** by
default. The CLI opts into the shared filter; the library does not.

The structural commands mostly ignore the lens: `-validate` and `-fix-duplicates`
operate on the whole vault, so CI still sees problems in documents that are not
shared. `-check-duplicates` is the exception: it reports on what was indexed, so it
needs `-include-unshared` to see unshared documents.

## Writing front matter

Two flags add or change front matter in place. Both walk every markdown file the
scan matches (`-glob`, recursively by default) under `-dir`.

`-initfrontmatter` adds what is missing and leaves existing values alone:

```sh
# Give every document a UUID and share: false if it is missing either.
./markdown-tools-cli -dir notes -initfrontmatter

# Same, but missing share keys become true. Existing share values stay put.
./markdown-tools-cli -dir notes -initfrontmatter -share=true
```

`-setshare` forces the permission and still fills in missing UUIDs. Existing
UUIDs and the rest of each file's front matter are preserved:

```sh
# Set share: false on every document under notes/, recursively.
./markdown-tools-cli -dir notes -setshare -share=false

# Set share: true on a single folder.
./markdown-tools-cli -dir notes/public -setshare -share=true
```

`-share` takes `true` or `false` (anything `strconv.ParseBool` accepts). It is
required with `-setshare`; with `-initfrontmatter` it only decides which value
missing share keys get. The write modes are mutually exclusive with each other
and with `-fix-duplicates`.

Writes are immediate and unconfirmed: run bulk changes on a copy or from version
control. Files whose bytes would not change are not rewritten, so repeating a
command is safe. Broken front matter follows the usual rule: the first bad file
aborts the run unless `-force` is passed, in which case it is skipped. After the
rewrite the CLI rescans the vault and writes JSON as usual.

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
path order wins), which is what `-check-duplicates` exists to catch. Files with no front
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
| `label`            | `INTERNAL`, `URL`, `TAG`, `INTERNAL_ASSET`, `EXTERNAL_ASSET`, `EMBEDDED_MARKDOWN`, `FRONTMATTER_INTERNAL`, `FRONTMATTER_URL`, or `FRONTMATTER_TAG` |
| `title`            | The pipe alias, or the raw target when there is none. For a tag, the text without `#`. For a markdown link, the link text. For an image embed, the alt text. |
| `from_document_id` | UUID of the document holding the reference                                 |
| `to_document_id`   | The target's UUID when a wikilink or a markdown embed resolves in the index, otherwise the raw target. For a tag, the tag text. Assets keep the raw target. |

Body wikilinks that begin with a URL scheme (`https://`, `mailto:`, `ipfs://`)
are `URL`; every other body wikilink is `INTERNAL`. An inline `#tag` in the body
is `TAG`. Body embeds are their own labels: `![[target]]` and `![](destination)`
are `INTERNAL_ASSET` or `EXTERNAL_ASSET` by the same URL-scheme rule, and a
`![[target]]` with no extension or a `.md` extension is `EMBEDDED_MARKDOWN`.
The same shapes in front matter strings use the `FRONTMATTER_` labels, and an
inline markdown link is read only from front matter. Only documents with
a non-empty UUID produce edges. Section anchors (`[[target#section]]`) and
aliases (`[[target|alias]]`) are both understood.

## Checking the vault

`-validate` reports structural problems on stderr and exits non-zero, so it can
gate CI:

```sh
$ ./markdown-tools-cli -dir testdata/vault -validate

broken-frontmatter.md: broken front matter: parse front matter: yaml: line 1: did not find expected ',' or ']'
duplicate UUID 77679f2b-... -> duplicate-a.md, duplicate-b.md
error: vault check found 2 problem(s)
```

It reports front matter that fails to parse, UUIDs shared across documents, and
front matter missing the `-idkey`. Files with no front matter at all are
tolerated. When nothing is wrong it prints `vault check: ok` and exits zero.

`-check-duplicates` reports duplicate UUIDs while still writing the index. `-fix-duplicates`
goes further: it lists the colliding documents, asks for confirmation, and assigns
each one a fresh UUID, leaving the rest of every file's front matter untouched.
Anything but `y`/`yes` aborts without modifying a file.

## Broken front matter

By default the first file whose front matter fails to parse aborts the run and
names the file. Pass `-force` (or `-F`) to skip those files instead, logging one
line each and indexing the rest:

```sh
$ ./markdown-tools-cli -dir testdata/vault

error: vault/broken-frontmatter.md: parse front matter: yaml: line 1: did not find expected ',' or ']'

$ ./markdown-tools-cli -dir testdata/vault -force -include-unshared

skipping vault/broken-frontmatter.md: parse front matter: yaml: line 1: did not find expected ',' or ']'
```

`-force` also covers the output phase itself: a document that fails to serialize
is dropped and logged rather than aborting, so the JSON stays valid.

## Piping the JSON

Each skipped file is reported on stderr, so the JSON on stdout is already clean
and pipes into `jq` on its own. `-quiet` (or `-q`) drops those notices when you
would rather not see them:

```sh
$ markdown-tools-cli -dir . -include-unshared -force | jq '.[] | .name'
$ markdown-tools-cli -dir . -include-unshared -force -q | jq '.[] | .name'
```

`-quiet` suppresses progress notices only. A run that fails still says so on
stderr and still exits non-zero, and `-validate` still reports normally,
because there the report *is* the output.

## Examples

The commands below run against the two fixture trees in this repo:

- `testdata/vault/` — a realistic vault. Folders, people, projects, tags, both
  `INTERNAL` and `URL` wikilinks, a deliberately duplicated UUID pair
  (`duplicate-a.md`, `duplicate-b.md`), and one file with broken front matter
  (`broken-frontmatter.md`). Most examples use this, and pass `-F` to get past
  the broken file.
- `examples/` — four small files for the basics. Note it contains an
  intentional duplicate UUID, so `a/b/post.md` loses its key to
  `duplicate.md` and disappears from the index. Use `testdata/vault` when you
  want every file to appear.

Build the binary once and call it `markdown-tools-cli` below:

```sh
go build -o markdown-tools-cli .
```

### Basic index

```sh
# Every document, as JSON on stdout.
markdown-tools-cli -dir testdata/vault -F -include-unshared > vault.json

# Just the shared documents (the default).
markdown-tools-cli -dir testdata/vault -F > shared.json

# One file per pattern, relative to -dir.
markdown-tools-cli -dir testdata/vault -glob 'People/*.md' -F -include-unshared
```

### Jumping straight to jq

`-F` gets you past the broken file, `-q` keeps the notices off your terminal.
Together they give you clean JSON to work with:

```sh
# How many documents indexed.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq 'length'

# Every path.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r '.[].relativePath'

# Name, title, and tags as a table.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r '.[] | [.name, .metadata.title, (.metadata.tags|tostring)] | @tsv'
```

### `-F` on its own versus `-F -q`

The only difference is stderr. Both give identical stdout:

```sh
# You see: skipping vault/broken-frontmatter.md: parse front matter: ...
markdown-tools-cli -dir testdata/vault -F -include-unshared | jq 'length'

# You see nothing; stdout is the same 24 documents.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq 'length'
```

`-F` alone is right when you want to know a file was skipped. `-F -q` is right
when you are piping. To keep the notice but still capture it:

```sh
markdown-tools-cli -dir testdata/vault -F -include-unshared 2>skipped.txt | jq 'length'
```

### Querying front matter

Front matter arrives under `metadata`, as whatever the YAML was. Because vaults
disagree about shapes, defensive `jq` tends to pay off:

```sh
# Titles, sorted. The select drops files with no front matter at all, whose
# title comes through as null.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r '[.[] | select(.metadata.title) | .metadata.title] | sort | .[]'

# One document by title.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r 'to_entries[] | select(.value.metadata.title == "Ada Lovelace") | .key'

# Slug built from the title.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r '.[] | select(.metadata.title) | .metadata.title | ascii_downcase | gsub(" "; "-")'

# Drafts, and anything unpublished.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r '.[] | select(.metadata.draft == true) | .relativePath'
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r '.[] | select(.metadata.published == false) | .relativePath'

# Tags written as a YAML list and as a comma-separated string, folded together.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r '[.[] | select(.metadata.tags) | .metadata.tags | if type == "array" then .[] else (gsub(", *";"\n") | split("\n")) end] | flatten | map(select(length > 0)) | unique | sort | .[]'
```

`author` has the same problem: some files write a string, some a mapping.

```sh
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r '[.[] | select(.metadata.author) | .metadata.author | if type == "object" then .name else . end] | unique | sort | .[]'
```

### Finding what changed

Front matter is content addressed, so its CID changes exactly when the front
matter does. That makes the index a cheap "what did I touch" check:

```sh
# Front matter CID per document, truncated for reading.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r '.[] | [.relativePath, .frontmatter_cid[0:12]] | @tsv'

# Documents edited since a given unix timestamp.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r --argjson cutoff 1789000000 '.[] | select(.modifiedUnix > $cutoff) | .relativePath'
```

### Walking the wikilink graph

```sh
# Every edge as a readable line.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared -wikilinks | jq -r '.[] | "\(.from_document_id[0:8]) -> \(.to_document_id[0:8])  \(.title)"'

# Internal edges only; drop the URL ones.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared -wikilinks | jq -r '.[] | select(.label == "INTERNAL") | .title'

# Aliases that resolve: the title is the pipe alias, not the target.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared -wikilinks | jq -r '.[] | select(.title | test("index"; "i")) | .title'

# The most-linked-to documents, as a backlink count.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared -wikilinks | jq -r '[.[] | select(.label == "INTERNAL") | .to_document_id] | group_by(.) | map({id: .[0], n: length}) | sort_by(-.n) | .[0:5][] | "\(.n)\t\(.id)"'

# Documents nothing links to.
markdown-tools-cli -dir testdata/vault -F -q -include-unshared -wikilinks > edges.json
markdown-tools-cli -dir testdata/vault -F -q -include-unshared | jq -r --slurpfile g edges.json '.[] | .key as $me | select([$g[0][] | select(.from_document_id == $me)] | length == 0) | .value.relativePath'
```

### Finding duplicates

`-check-duplicates` reports them after writing the index, and exits non-zero, so it
works in CI. Pass `-include-unshared`: it runs *after* the share filter, so without it the
collisions are filtered out before it ever sees them.

```sh
# Reports the deliberate duplicate-a/duplicate-b pair. Exits 1.
markdown-tools-cli -dir testdata/vault -F -check-duplicates -include-unshared > /dev/null

# Which UUIDs collide.
markdown-tools-cli -dir testdata/vault -F -check-duplicates -include-unshared 2>&1 >/dev/null | grep '^duplicate UUID'
```

`-fix-duplicates` is the opposite: it works on the whole vault regardless of the
filter, because a collision between two unshared documents is still a collision
worth fixing. It rewrites front matter in place, so run it on a copy.

```sh
cp -r testdata/vault /tmp/scratch
markdown-tools-cli -dir /tmp/scratch -fix-duplicates -F     # prompts; y/yes proceeds
markdown-tools-cli -dir /tmp/scratch -check-duplicates -F -include-unshared # now clean
```

### Checking the vault

`-q` changes nothing here, because the report goes to stderr and is the output:

```sh
markdown-tools-cli -dir testdata/vault -validate
echo "exit=$?"        # 1, the vault has problems

# In CI: fail the build on a broken vault.
markdown-tools-cli -dir testdata/vault -validate || exit 1

# Only ask whether the vault is clean.
markdown-tools-cli -dir testdata/vault -validate 2>&1 | grep -q 'vault check: ok' && echo clean
```

### Writing somewhere other than stdout

`-out` takes the JSON off your terminal, which is handy when a document body is
large:

```sh
markdown-tools-cli -dir testdata/vault -F -q -include-unshared -out vault.json

# Then query the file directly.
jq -r '.[].name' vault.json | sort
```

### A custom UUID key

Vaults that key documents on something other than `uuid`:

```sh
markdown-tools-cli -dir . -F -q -include-unshared -idkey id
markdown-tools-cli -dir . -F -q -include-unshared -idkey id | jq -r 'to_entries[] | "\(.key)  \(.value.relativePath)"'
```

### Memory footprint

Useful when a vault is large enough to wonder.

```sh
markdown-tools-cli -dir testdata/vault -F -include-unshared -memusage > /dev/null
```

## Library dependencies

```go
require (
	github.com/dentropy/markdown-indexer/libs/markdown-indexer v0.0.0
	github.com/dentropy/markdown-indexer/libs/markdown-tools v0.0.0
)
```

The read-only package is reached through `markdownindexer`:

```go
import "github.com/dentropy/markdown-indexer/libs/markdown-indexer"

idx, err := markdownindexer.Scan(dir, markdownindexer.Options{})
```

The write package is reached through `markdowntools`:

```go
import "github.com/dentropy/markdown-indexer/libs/markdown-tools"

changed, err := markdowntools.SetShare(idx, dir, "uuid", false)
```

Both sibling modules are resolved with local `replace` directives during
development. Those directives are ignored when a module is consumed as a
dependency.

## Test

This module depends on both sibling libraries, so build it in module mode from
its own directory:

```sh
go test ./...
```

Manual verification steps are in [MANUAL-TESTING.md](MANUAL-TESTING.md).

## Layout

```
main.go              flags, orchestration, prompts, share filtering, JSON encoding
main_test.go         the tests
examples/            four small files, including a deliberate duplicate UUID
testdata/            a realistic vault, used by the tests and most examples
MANUAL-TESTING.md    manual verification steps
```