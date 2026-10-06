# Vocabulary

| Term                | Meaning                                                                                                             |
| ------------------- | ------------------------------------------------------------------------------------------------------------------- |
| **Vault**           | A directory tree of markdown files. The source of truth for what exists.                                            |
| **Document**        | One markdown file as indexed: relative path, name, modification time, front matter (as JSON), front matter content address, raw markdown body, and UUID. |
| **Front matter**    | The YAML block at the top of a document, held in the index as JSON.                                                |
| **UUID**            | A document's identifier, read from front matter. Its identity, not its path.                                        |
| **Content address** | The IPLD hash (CIDv1, dag-json codec, sha2-256) of front matter or an edge.                                          |
| **Wikilink**        | A `[[target]]` reference in a document body.                                                                        |
| **Edge**            | One resolved wikilink: from UUID, to UUID, label, title. Keyed by content address.                                   |
| **Index**           | The in-memory collection of documents and edges derived from the vault.                                              |
| **Problem**         | One structural fault found while indexing or checking: the paths it concerns, and the error that caused it.            |

A document's identity is its UUID, not its path: a rename leaves the same
document in place with a new `RelativePath`.

Back to the [README](../README.md).
