# Backlinks are derived, body-only, and symmetric with the published set

A document's backlinks are the resolved body `INTERNAL` edges arriving at it.
They are computed from the index on demand (`Index.Backlinks` or
`BuildBacklinks`) rather than stored, so the graph keeps one representation of
a relationship and a backlink can never disagree with its edge.

The published shape is `backlinks/<uuid>.json`, described by
[`backlinks.schema.json`](../backlinks.schema.json). Every document with a
non-empty UUID gets a file, with `"backlinks": []` when nothing links to it, so
a publisher can serve the path unconditionally.

Three decisions shape it:

- **Body `INTERNAL` only.** The schema pins `label` to `INTERNAL`, so
  `FRONTMATTER_INTERNAL`, `FRONTMATTER_INTERNAL_MISSING`, `URL`, `TAG`,
  `INTERNAL_ASSET`, `EXTERNAL_ASSET`, `EMBEDDED_MARKDOWN`,
  `EMBEDDED_MARKDOWN_MISSING`, `INTERNAL_MISSING`, and `FRONTMATTER_URL` are
  dropped. A `[[link]]` in front matter is not a backlink. Resolvedness is
  checked by the target being an indexed UUID, which also rejects the empty
  ID a UUID-less document resolves to.
- **Self-links are dropped.** A document linking to itself describes its own
  body, not an incoming reference, and would make every self-referencing note
  its own backlink.
- **Both ends come from the caller's document set.** The map is built from the
  documents handed in (or `Index.Documents` and its `Edges`), so
  `Options{Shared: true}` and the CLI's default share filter narrow sources and
  targets together. An unshared document never leaks its title or path into a
  published file, and its links never resolve.

Ordering is fixed rather than incidental: `from_name`, then `title`, then
`from_document_id`, byte-wise. `from_name` is the source's front matter
`title` when it is a non-empty string, otherwise its file name. A duplicated
target UUID keeps the `Index.ByUUID` rule: the document with the greatest
relative path wins.

Deduping is inherited from the graph: identical edges share a content address
(ADR [0001](0001-frontmatter-edge-labels.md)), so a link written twice in one
document yields one backlink. Two different aliases to the same target are two
edges and therefore two entries.
