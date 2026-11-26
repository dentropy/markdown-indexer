# JSON schema

`Document`, `Edge`, and `Manifest` are the values this package puts into JSON,
and their field names are fixed. The schemas are kept as files next to this one:

| File                                          | Root                                               | One entry          |
| --------------------------------------------- | -------------------------------------------------- | ------------------ |
| [documents.schema.json](documents.schema.json) | The document index, keyed by UUID                  | `$defs.document`   |
| [wikilinks.schema.json](wikilinks.schema.json) | The graph of wikilinks, inline tags, and front matter edges, keyed by content address | `$defs.wikilink`   |
| [backlinks.schema.json](backlinks.schema.json) | The incoming links for one document, published at `/backlinks/$UUID.json` | `$defs.backlink` |
| [manifest.schema.json](manifest.schema.json) | The single-file directory of the vault: every document's title, UUID, and path | `$defs.entry` |
| [assets.schema.json](assets.schema.json) | The hashed non-markdown files, keyed by relative path, when `HashAssets` is on | `$defs.asset` |
| [articles.schema.json](articles.schema.json) | The published per-document JSON at `/articles/$UUID.json` | the root object |

Each file validates both levels: the root is the envelope the CLI prints, and
`$defs` holds a single entry inside it.

```python
import json
from jsonschema import validate

schema = json.load(open("documents.schema.json"))
vault = json.load(open("vault.json"))

validate(vault, schema)                                            # every document
validate(next(iter(vault.values())), schema["$defs"]["document"])  # one document
```

The graph works the same way: `wikilinks.schema.json` at the root,
`$defs.wikilink` for one edge.

Backlinks are not part of the index this package produces; they are derived from
it. `backlinks.schema.json` describes the per-document file a consumer publishes:
every resolved `INTERNAL` edge whose target is a published document becomes one
entry on that target. Unresolved targets, `URL` edges, and self-links are
dropped, and a document with no incoming links still gets a file with an empty
`backlinks` array. The envelope is one document's file, and `$defs.backlink` is
one entry inside it.

The UUID is not a field of its own in the document index. `Document.ID` is
deliberately not serialized there: it is read out of `metadata` under
`Options.IDKey`, and it is what the `documents.schema.json` root is keyed by.
The `$defs.document` shape is therefore what a single entry under that key looks
like, not the envelope itself. The manifest is the exception: it names the UUID
as a field, because its job is to carry one.

The manifest is also derived, not part of the index. `manifest.schema.json`
describes a single JSON file that lists every document with a non-empty UUID by
`title`, `uuid`, and `path`, ordered by path. `title` is the display title
(front matter title when it is a non-empty string, otherwise the file name), and
`path` is the same value as `relativePath` in the document index. Documents
without a UUID are omitted, an empty vault yields `"documents": []`, and a
duplicated UUID appears on more than one entry because a manifest describes the
files that exist rather than collapsing them. It is the shape to publish at a
fixed path such as `site/directory.json` when a consumer needs to resolve an
identifier to a readable name and location without carrying the whole index.

`assets.schema.json` is the hashed non-markdown files keyed by relative path,
produced when `Options.HashAssets` is set. `articles.schema.json` is one
published document at `/articles/$UUID.json`: identity, path, front matter CID,
and body, without inlined metadata.

`label` is an enum of twelve values in three families. Body hits are
`INTERNAL`, `INTERNAL_MISSING`, `URL`, `TAG`, `INTERNAL_ASSET`,
`EXTERNAL_ASSET`, `EMBEDDED_MARKDOWN`, and `EMBEDDED_MARKDOWN_MISSING`. Front
matter strings produce `FRONTMATTER_INTERNAL`, `FRONTMATTER_INTERNAL_MISSING`,
`FRONTMATTER_URL`, and `FRONTMATTER_TAG` instead, so they do not share an
address with a body edge. `TAG` and `FRONTMATTER_TAG` store
the text without `#` in both `title` and `to_document_id`, and neither resolves
to a document UUID. The front matter `tags` field does not produce either label.
`title` and `to_document_id` are free-form on purpose: an unresolved target is
raw text, which may be a URL, a section anchor, a tag, or a typo.

How those fields are addressed is described in
[Content addressing](content-addressing.md).

Back to the [README](../README.md).
