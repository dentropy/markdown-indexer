# Backlinks: derived per-document incoming links across the three modules

Implement the `backlinks.schema.json` artifact end to end:

- `markdown-indexer` computes backlinks from the index (read-only, no I/O).
- `markdown-tools` publishes them as one JSON file per document (writes only).
- `markdown-tools-cli` exposes `-backlinks` / `-backlinks-dir` on top of both.

The published shape is fixed by `libs/markdown-indexer/docs/backlinks.schema.json`:
`{"document_id": "...", "backlinks": [...]}`, one file per published document at
`backlinks/<uuid>.json`, with an empty array when nothing links to it.

## Semantics (single source of truth)

A backlink is **one resolved body `INTERNAL` edge arriving at a document**:

| Keep | Drop |
|---|---|
| `label == "INTERNAL"` and `to_document_id` is a UUID of an indexed document | `INTERNAL_MISSING`, `URL`, `TAG`, `EMBEDDED_MARKDOWN`, `*_ASSET`, all `FRONTMATTER_*` |
| source and target both in the caller's document set (the "published set") | self-links (`from == to`) |
| one entry per distinct edge (existing CID dedupe) | edges with an empty `from`/`to` UUID |

Derived fields:

- `from_document_id` — `Edge.FromDocumentID`.
- `from_name` — source document's front matter `title` when it is a non-empty
  string, otherwise its `Name` (file name without `.md`).
- `from_relative_path` — source document's `RelativePath`.
- `title` — `Edge.Title` (pipe alias, else raw target).
- `label` — literally `"INTERNAL"`.

Ordering is fixed: `from_name`, then `title`, then `from_document_id`, all
byte-wise string comparison (`sort.SliceStable`). Every document with a non-empty
UUID gets an entry, with a non-nil empty slice so JSON emits `[]`, never `null`.

Privacy: both ends come from the same filtered document set the CLI already
builds, so an unshared document never appears as a source (no leaked title or
path) and its links never resolve. `-include-unshared` widens both ends.

## 1. `libs/markdown-indexer` (compute)

### New file `backlinks.go`

```go
type Backlink struct {
    FromDocumentID   string `json:"from_document_id"`
    FromName         string `json:"from_name"`
    FromRelativePath string `json:"from_relative_path"`
    Title            string `json:"title"`
    Label            string `json:"label"` // always labelInternal
}

type Backlinks struct {
    DocumentID string     `json:"document_id"`
    Backlinks  []Backlink `json:"backlinks"` // never nil
}

// Backlinks derives the incoming links for every document in the index, keyed
// by target UUID. Uses Index.Edges; call BuildBacklinks for documents held
// elsewhere.
func (ix *Index) Backlinks() map[string]Backlinks

// BuildBacklinks derives backlinks from documents alone, mirroring BuildEdges.
func BuildBacklinks(docs []Document) map[string]Backlinks
```

Implementation notes:

- `buildBacklinks(docs, edges)` shared by both entry points.
- Index documents deterministically with `ByUUID` semantics: iterate the
  relative-path-sorted slice, later overwrite wins, so duplicate UUIDs resolve
  to the greatest path (same rule as `Index.ByUUID`).
- Seed one `Backlinks{DocumentID: id, Backlinks: []Backlink{}}` per unique
  non-empty UUID, then append matching edges, then sort.
- Skip an edge whose source is missing from the map (defensive; should not
  happen when `Edges` was built from `Documents`).
- `ix.Edges` nil (zero-value `Index`) means no edges; `BuildBacklinks` is the
  path for callers holding documents from a database.

### `document.go`: display name

```go
// DisplayName returns the front matter title when it is a non-empty string,
// otherwise Name.
func (d Document) DisplayName() string
```

Top-level `title` only; a non-string or whitespace-only value falls back to
`Name`. `from_name` must never be empty for an indexed document.

### Tests: `backlinks_test.go`

- `[[Target]]` with and without an alias → one entry each, `title` is alias /
  raw target; `from_name` is front matter title, else file name.
- Multiple sources → sorted by `from_name`, then `title`, then UUID; assert
  exact order with deliberately out-of-order inputs and edge-map iteration
  (build `Edges` from a map literal to prove determinism).
- Self-link `[[Me]]` → dropped, target still gets `backlinks: []`.
- Missing target, `URL`, `TAG`, embed, and `FRONTMATTER_INTERNAL` → dropped.
- Every published document has an entry; `len(result[id].Backlinks) == 0` and
  `json.Marshal` yields `[]`, not `null`.
- Documents without a UUID are neither sources nor targets (also covers an
  `INTERNAL` edge that resolves to an empty ID).
- Duplicate UUID: deterministic source selection (greatest relative path).
- `BuildBacklinks(docs)` equals `(&Index{Documents: docs, Edges: BuildEdges(docs)}).Backlinks()`.
- Optional but recommended: test-only dependency `santhosh-tekuri/jsonschema/v6`
  validates each marshaled `Backlinks` against `docs/backlinks.schema.json`
  (read with a package-relative path), so the schema file cannot drift.

### Docs

- `doc.go` — one paragraph: backlinks are derived incoming edges, not edges.
- `docs/usage.md` — new "Backlinks" section: `Backlinks()`, `BuildBacklinks()`,
  the keep/drop table, sorting, empty-array guarantee, shared-set behavior.
- `docs/vocabulary.md`, `CONTEXT.md` — add **Backlink**: a resolved body
  `INTERNAL` edge arriving at a document; one entry in that document's
  backlinks file. Distinct from **Edge** (outgoing).
- `docs/adr/0003-backlinks-derivation.md` — record: derived on read, body
  `INTERNAL` only (schema `const`), both ends restricted to the caller's
  document set, self-links dropped, fixed sort, empty array per document.
- `docs/json-schema.md` — link the derivation section; drive-by fix the stale
  "enum of six values" sentence while in the file.
- `libs/markdown-indexer/README.md` — mention `Backlinks()` next to `BuildEdges`.

## 2. `libs/markdown-tools` (write)

### New file `backlinks.go`

```go
type BacklinksOptions struct {
    Prune bool // remove *.json in dir that are not in the set
}

// WriteBacklinks writes dir/<document_id>.json for every entry, atomically,
// skipping files whose bytes are unchanged. Returns files written and removed.
func WriteBacklinks(backlinks map[string]markdownindexer.Backlinks, dir string, opts BacklinksOptions) (written, removed []string, err error)
```

- Accept the computed map, not an `*Index`: the CLI builds the filtered set and
  the indexer owns derivation. (Keeps `markdown-tools` from re-deriving.)
- `os.MkdirAll(dir, 0o755)`; encode with two-space indent and a trailing newline;
  write via `CreateTemp` in `dir` → `Chmod(0o644)` → `Rename`; remove the temp on
  any error. Skip the rename when existing bytes are identical (stable mtimes).
- **Filename safety (required).** IDs come from front matter. Validate every ID
  before writing anything: non-empty, equals `filepath.Base(id)`, not `.`/`..`,
  no `/`, `\`, `:` or NUL. Fail closed with one error listing the bad IDs and
  write nothing. CLI `-force` may skip them instead.
- **Prune (opt-in by caller).** After all writes succeed, `os.ReadDir(dir)` and
  remove regular `*.json` files whose stem is not in the map. Never recurse and
  never touch non-`.json` files. This makes the directory mirror the published
  set; document that the tool owns the directory.
- Return sorted relative names so CLI notices and tests are deterministic.

### Tests: `backlinks_test.go`

- Writes one file per entry, including an empty `backlinks` array.
- Exact JSON bytes/shape for one document; re-run is idempotent and does not
  rewrite unchanged files.
- Prune removes stale `*.json`, leaves non-`.json` files and subdirectories.
- Unsafe IDs (`../x`, `a/b`, `..`, empty) → error, nothing written.
- Missing directory is created; unwritable directory returns an error.
- Empty map writes nothing and (with prune) removes stale files.

### Docs

- `doc.go` + `README.md` — widen from "writes front matter" to "writes derived
  output": front matter in the vault, backlinks files in a publish tree. Add a
  "Publish backlinks" section with the call and the file layout.
- Root `README.md` module table: mention backlinks publishing for
  `markdown-tools` and the CLI.

## 3. `libs/markdown-tools-cli` (expose)

### Flags

| Flag | Default | Behavior |
|---|---|---|
| `-backlinks` | `false` | select backlinks output instead of documents / `-wikilinks` |
| `-backlinks-dir` | `""` | write `<dir>/<uuid>.json`; implies `-backlinks` |
| `-id <uuid>` | `""` | with `-backlinks`, print one document's file (schema root) |
| `-prune` | `true` | with `-backlinks-dir`, remove stale `*.json`; `-prune=false` keeps them |

Validation:

- `-backlinks` and `-wikilinks` are mutually exclusive.
- `-backlinks-dir` with `-out` is an error (two output sinks).
- `-id` requires `-backlinks` and is incompatible with `-backlinks-dir`.
- Vault write modes (`-initfrontmatter`, `-setshare`, `-fix-duplicates`) may run
  first; the existing rescan then feeds backlinks, so publishing after a
  permission change reflects the new published set.

### Wiring

After `docs = filterSharedDocs(docs, opts)` (and the existing rescan):

```go
backlinks := markdownindexer.BuildBacklinks(docs)
```

- stdout: encode `map[string]Backlinks` keyed by UUID (a container, not the
  published file format). With `-id`, encode that single `Backlinks` value and
  error on an unknown ID.
- `-backlinks-dir`: call `markdowntools.WriteBacklinks(backlinks, dir,
  markdowntools.BacklinksOptions{Prune: opts.prune})`; print
  `wrote backlinks: <dir>/<id>.json`, `removed stale: ...`, and a total to
  stderr unless `-quiet`. With `-force`, skip and report unsafe IDs instead of
  aborting.
- Remove the `buildWikilinkGraph` wrapper if `BuildBacklinks` makes it the only
  caller pattern; keep `-wikilinks` behavior unchanged.

### Tests: `main_test.go`

- `-backlinks` stdout: map keyed by UUID, every filtered doc present, empty
  arrays for unlinked docs, deterministic order inside each array.
- `-backlinks -id`: one envelope, schema-shaped; unknown ID errors.
- `-backlinks-dir`: temp dir tree, one file per doc, stale file pruned, notices
  and `-quiet` behavior.
- `-include-unshared` off: unshared source/target absent; on: present.
- Flag validation: `-backlinks -wikilinks`, `-backlinks-dir -out`,
  `-id` without `-backlinks` → errors.
- `-force` skips an unsafe UUID, reports it, writes the rest.

### Docs

- `README.md` — the already-advertised `-backlinks` example becomes real; add
  the three flag rows, stdout vs file mode, the published file layout, a jq
  recipe (split the map into files), and the `-include-unshared` privacy note.
- `MANUAL-TESTING.md` — end-to-end steps against `testdata/vault`: publish to a
  temp dir, inspect one file, confirm empty arrays and pruning.

## 4. Integration and verification

```sh
cd libs/markdown-indexer   && gofmt -l . && go vet ./... && go test ./...
cd libs/markdown-tools     && gofmt -l . && go vet ./... && go test ./...
cd libs/markdown-tools-cli && gofmt -l . && go vet ./... && go test ./...

# End to end
go run . -dir testdata/vault -F -q -include-unshared -backlinks -backlinks-dir /tmp/backlinks
find /tmp/backlinks -name '*.json' | wc -l
cat /tmp/backlinks/$(ls /tmp/backlinks | head -1)
```

Acceptance:

- Every published document has exactly one file; unlinked documents contain
  `"backlinks": []`.
- No unresolved, URL, tag, embed, front matter, or self entries.
- Two runs produce byte-identical files; pruning removes stale ones.
- `-backlinks -id <uuid>` output validates against `backlinks.schema.json`.

## Sequencing (one commit per layer, each green)

1. `markdown-indexer`: types, `DisplayName`, `Backlinks`/`BuildBacklinks`,
   tests, docs, ADR.
2. `markdown-tools`: atomic writer + prune + tests + docs.
3. `markdown-tools-cli`: flags, wiring, tests, README, manual testing.
4. Integration test/fixture touch-ups, root README, final full test run.

## Decisions (confirmed)

1. **Front matter links:** Option A. Only body `INTERNAL` edges count; the
   schema's `const: "INTERNAL"` stays as-is.
2. **Display title:** the top-level `title` front matter key, falling back to
   the file name.
3. **Prune:** on by default for `-backlinks-dir`, `-prune=false` to keep stale
   files. An empty set prunes everything, because the directory is treated as
   owned by the tool.
4. **Unshared sources:** excluded. Both ends come from the loaded set.

Implementation note: an unsafe document ID fails the publish closed even under
`-force`; `-force` still only skips broken front matter.
