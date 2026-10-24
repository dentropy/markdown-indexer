# JSON schema

`Document` and `Edge` are the two values this package puts into JSON, and their
field names are fixed. The schemas are kept as files next to this one:

| File                                          | Root                                               | One entry          |
| --------------------------------------------- | -------------------------------------------------- | ------------------ |
| [documents.schema.json](documents.schema.json) | The document index, keyed by UUID                  | `$defs.document`   |
| [wikilinks.schema.json](wikilinks.schema.json) | The graph of wikilinks, inline tags, and front matter edges, keyed by content address | `$defs.wikilink`   |

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

The UUID is not a field of its own. `Document.ID` is deliberately not
serialized: it is read out of `metadata` under `Options.IDKey`, and it is what
the `documents.schema.json` root is keyed by. The `$defs.document` shape is
therefore what a single entry under that key looks like, not the envelope
itself.

`label` is an enum of six values. Body wikilinks are `INTERNAL` or `URL`.
`TAG` is an inline `#tag` in the body. Front matter strings produce
`FRONTMATTER_INTERNAL`, `FRONTMATTER_URL`, and `FRONTMATTER_TAG` instead, so
they do not share an address with a body edge. `TAG` and `FRONTMATTER_TAG` store
the text without `#` in both `title` and `to_document_id`, and neither resolves
to a document UUID. The front matter `tags` field does not produce either label.
`title` and `to_document_id` are free-form on purpose: an unresolved target is
raw text, which may be a URL, a section anchor, a tag, or a typo.

How those fields are addressed is described in
[Content addressing](content-addressing.md).

Back to the [README](../README.md).
