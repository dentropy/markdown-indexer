# The manifest is a flat, path-ordered list, not another UUID-keyed map

The vault manifest is a single JSON file, `{"documents": [...]}`, described by
[`manifest.schema.json`](../manifest.schema.json). Each entry is `title`,
`uuid`, and `path`, in that order, sorted by `path` and then `uuid`. It is
derived from the index on demand through `Index.Manifest` or `BuildManifest`,
exactly as backlinks are.

The document index already holds every one of these values. A separate shape is
worth having because the index is keyed by UUID and carries bodies, front matter,
and content addresses, so a consumer that only needs to resolve an identifier to
a readable name and location cannot use it without carrying the vault. The
manifest is the file to publish at a fixed path such as `site/directory.json`;
the index is not.

Four decisions shape it:

- **An envelope, not a bare array.** `{"documents": [...]}` leaves room for
  vault-level facts later without breaking a consumer that already reads
  `documents`, and it matches the object-at-the-root shape of the other schemas.
  A bare array cannot grow.
- **`uuid` is a field here.** The document index deliberately does not serialize
  `Document.ID`; it is the map key. The manifest's job is to carry identity, so
  it names it. `title` and `path` are the display name and `relativePath`
  values, not new derivations.
- **Only documents with a UUID.** An entry is a way to find a document by
  identity. A document with no UUID has nothing to list, and the empty key in
  the document index is a fault (`-check-duplicates`, `-validate`), not an
  identity. This also matches backlinks, which skip the empty ID.
- **Duplicate UUIDs are not collapsed.** The manifest describes the files that
  exist. Two documents claiming one UUID become two entries, in path order,
  which is visible in the published file. `Index.ByUUID` and backlinks collapse
  them deterministically for lookups; a directory listing must not hide a file.

Ordering is fixed rather than incidental: `path`, then `uuid` as a tie-break for
merged document sets. The list is never `null`; an empty vault publishes
`"documents": []`, so a consumer can iterate unconditionally.
