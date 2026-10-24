# markdown-indexer

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
A `[[target]]` reference in a document body or in a front matter string.
_Avoid_: link, internal link

**URL**:
A link target that starts with a scheme (`https:`, `mailto:`, `ipfs:`). It is kept as written and is not a document name or a path.
_Avoid_: website, external link, URI

**Asset**:
An embedded non-markdown target, `![[image.png]]` or `![](assets/x.png)`. Kept as written; not a document, and not indexed.
_Avoid_: image, attachment, binary

**Markdown embed**:
A `![[note]]` body embed of another document's content. Classified by the target's shape, not the vault: no file extension or `.md` means markdown. Not a wikilink and not an asset.
_Avoid_: note embed, transclusion, embedded wikilink

**Tag**:
A `#name` label in a document's markdown body. Not a document, and not a wikilink. A `#name` in a front matter string is a front matter edge. The front matter `tags` field is not a tag.
_Avoid_: hashtag, topic, front matter tag

**Front matter edge**:
A wikilink, markdown link, or `#name` found in a front matter string. Not a body wikilink, not a body embed, and not a tag. An embed in front matter is not scanned: `![[x]]` is a plain wikilink there and `![](...)` is skipped.
_Avoid_: front matter tag, metadata link

**Edge**:
One relationship leaving a document: a body wikilink, a body embed, a body tag, or a front matter edge. Keyed by its content address, so identical edges dedupe.
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