# Problems

## Broken files

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

## Duplicate UUIDs

A duplicate UUID is **not** a scan problem. Both documents index cleanly, so
nothing lands in `idx.Problems`; ask the index instead:

```go
for _, id := range idx.DuplicateUUIDs() {
	fmt.Println(id, idx.PathsForID(id))
}
```

`idx.ByUUID()` keys the documents by UUID, and documents with no UUID are keyed
by the empty string, which is how you find them.

## Checking a vault

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

Back to the [README](../README.md).
