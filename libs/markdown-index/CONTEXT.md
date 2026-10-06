# markdown-index

Indexes a markdown vault into content-addressed documents and a wikilink edge graph. This is the domain model of the `markdownindexer` package; the terms below are the ones its API uses.

## Vault and documents

**Vault**:
A directory tree of markdown files; the source of truth for what exists.
_Avoid_: folder, notes directory

**Document**:
One markdown file as indexed: relative path, name, modification time, front matter (as JSON), front matter content address, raw markdown body, and a UUID. Its identity is the UUID, not the path.
_Avoid_: note, file

**Front matter**:
The YAML block at the top of a document, held in the index as JSON.
_Avoid_: metadata, header

**UUID**:
A document's identifier, read from front matter. A document without one is still indexed, with an empty id; callers that need one per document are expected to reject it.
_Avoid_: id (ambiguous), doc id, document key

**Content address (CID)**:
The IPLD hash (CIDv1, dag-json codec, sha2-256) of front matter or of an edge; identical input always yields the same address.
_Avoid_: hash, checksum, fingerprint

## The graph

**Wikilink**:
A `[[target]]` reference in a document body.
_Avoid_: link, internal link

**Edge**:
One resolved wikilink graph edge (from UUID, to UUID, label, title). Keyed by its content address, so identical edges dedupe.
_Avoid_: link object, relationship

## The index

**Index**:
The in-memory collection of documents and edges derived from the vault. It is derived and read-only: the vault is always true, and this package never writes back.
_Avoid_: snapshot, dataset

**Scan**:
One recompute of the whole index. Files that cannot be parsed do not abort the scan; each becomes a problem on the index, so one pass surfaces every fault in the vault.
_Avoid_: index, run, sweep

**Problem**:
One fault found while indexing or checking: the paths it concerns and the error that caused it.
_Avoid_: issue, error log

**Check**:
The scan for faults alone, without keeping documents or edges.
_Avoid_: validate, lint