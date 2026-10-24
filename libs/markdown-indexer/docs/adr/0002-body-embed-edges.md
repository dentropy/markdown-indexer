# Body embeds get their own labels

Embeds used to be invisible or misclassified: `![](assets/x.png)` produced no
edge, and `![[image.png]]` matched as a plain wikilink, yielding an `INTERNAL`
edge whose target was a file. Body embeds are now edges with their own labels:
`INTERNAL_ASSET` and `EXTERNAL_ASSET` for `![[asset]]` and `![alt](dest)`, and
`EMBEDDED_MARKDOWN` for `![[note]]`.

The label is part of the edge's content address (ADR 0001), so every existing
body edge keeps its address; the fix to `![[image.png]]` is the one deliberate
change, replacing a wrong `INTERNAL` edge with the right `INTERNAL_ASSET` one.

Two decisions shape the family:

- **Classification is the target's shape, not the vault.** A URL scheme is
  `EXTERNAL_ASSET`; a `![[target]]` with no extension or `.md` is
  `EMBEDDED_MARKDOWN`; anything else is `INTERNAL_ASSET`. Deterministic, no
  lookup, so `![[missing-note]]` is still a markdown embed. `EMBEDDED_MARKDOWN`
  resolves through the name map like a wikilink; assets keep the raw target
  because only `.md` files are indexed.
- **Front matter embeds are not scanned.** Front matter keeps the pre-embed
  behavior: `![[x]]` there is a plain wikilink (`FRONTMATTER_INTERNAL`) and
  `![](...)` is skipped. Body-only labels mean a body embed and a front matter
  hit on the same target stay two edges by construction, with no
  `FRONTMATTER_*_ASSET` variants to maintain.
