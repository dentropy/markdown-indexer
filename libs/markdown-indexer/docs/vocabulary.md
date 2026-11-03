# Vocabulary

| Term                | Meaning                                                                                                             |
| ------------------- | ------------------------------------------------------------------------------------------------------------------- |
| **Vault**           | A directory tree of markdown files, and the non-markdown files they may reference. The source of truth for what exists. |
| **Document**        | One markdown file as indexed: relative path, name, modification time, front matter (as JSON), front matter content address, raw markdown body, and UUID. |
| **Front matter**    | The YAML block at the top of a document, held in the index as JSON.                                                |
| **UUID**            | A document's identifier, read from front matter. Its identity, not its path.                                        |
| **Content address** | The IPLD hash (CIDv1, dag-json codec, sha2-256) of front matter or an edge.                                          |
| **Wikilink**        | A `[[target]]` reference in a document body or in a front matter string.                                            |
| **URL**             | A link target that starts with a scheme (`https:`, `mailto:`, `ipfs:`). Kept as written. Not a document name or a path. |
| **Asset**           | A non-markdown file in the vault, indexed by relative path and the sha256 of its bytes when hashing is on. Not a document. An embed target is kept as written and resolved against this index. |
| **Markdown embed**  | A `![[note]]` body embed of another document's content. Not a wikilink and not an asset.                            |
| **Tag**             | A `#name` label in a document's markdown body. Not a document, and not a wikilink. A `#name` in a front matter string is a front matter edge. The front matter `tags` field is not a tag. |
| **Front matter edge** | A wikilink, markdown link, or `#name` found in a front matter string. Not a body wikilink, not a body embed, and not a tag. |
| **Edge**            | One relationship leaving a document: a body wikilink, a body embed, a body tag, or a front matter edge. Keyed by content address. |
| **Backlink**        | A resolved body `INTERNAL` edge arriving at a document. One entry in that document's published backlinks file. Unlike an edge, it is derived rather than stored in the index. |
| **Article**         | The published per-document record: UUID, path, front matter content address, and markdown body, without the inlined front matter object. |
| **Manifest**        | The vault's file directory as one value: every document with a UUID, ordered by path, carrying only its title, UUID, and path. Derived, not stored. |
| **Index**           | The in-memory collection of documents, edges, and assets derived from the vault.                                      |
| **Problem**         | One structural fault found while indexing or checking: the paths it concerns, and the error that caused it.            |

A document's identity is its UUID, not its path: a rename leaves the same
document in place with a new `RelativePath`.

Back to the [README](../README.md).
