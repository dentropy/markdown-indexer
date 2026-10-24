# Vocabulary

| Term                | Meaning                                                                                                             |
| ------------------- | ------------------------------------------------------------------------------------------------------------------- |
| **Vault**           | A directory tree of markdown files. The source of truth for what exists.                                            |
| **Document**        | One markdown file as indexed: relative path, name, modification time, front matter (as JSON), front matter content address, raw markdown body, and UUID. |
| **Front matter**    | The YAML block at the top of a document, held in the index as JSON.                                                |
| **UUID**            | A document's identifier, read from front matter. Its identity, not its path.                                        |
| **Content address** | The IPLD hash (CIDv1, dag-json codec, sha2-256) of front matter or an edge.                                          |
| **Wikilink**        | A `[[target]]` reference in a document body or in a front matter string.                                            |
| **URL**             | A link target that starts with a scheme (`https:`, `mailto:`, `ipfs:`). Kept as written. Not a document name or a path. |
| **Asset**           | An embedded non-markdown target, `![[image.png]]` or `![](assets/x.png)`. Kept as written. Not a document; assets are not indexed. |
| **Markdown embed**  | A `![[note]]` body embed of another document's content. Not a wikilink and not an asset.                            |
| **Tag**             | A `#name` label in a document's markdown body. Not a document, and not a wikilink. A `#name` in a front matter string is a front matter edge. The front matter `tags` field is not a tag. |
| **Front matter edge** | A wikilink, markdown link, or `#name` found in a front matter string. Not a body wikilink, not a body embed, and not a tag. |
| **Edge**            | One relationship leaving a document: a body wikilink, a body embed, a body tag, or a front matter edge. Keyed by content address. |
| **Index**           | The in-memory collection of documents and edges derived from the vault.                                              |
| **Problem**         | One structural fault found while indexing or checking: the paths it concerns, and the error that caused it.            |

A document's identity is its UUID, not its path: a rename leaves the same
document in place with a new `RelativePath`.

Back to the [README](../README.md).
