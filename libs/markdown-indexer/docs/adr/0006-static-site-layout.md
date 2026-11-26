# The published site is articles, frontmatter, assets, directory.json, and backlinks

A publisher writes one vault scan as a static JSON tree an app can fetch:

- `directory.json` is the existing path-ordered directory (ADR 0004).
- `articles/<uuid>.json` is the document without inlined front matter.
- `frontmatter/<frontmatter_cid>.json` is the parsed front matter, addressed by
  the indexer's CID, so identical front matter is one file. Those bytes are not
  rewritten: changing them would make the CID lie.
- `assets/<sha256>.<ext>` is the vault binary, named by the indexer hash.
- `backlinks/<uuid>.json` is the existing incoming-link file (ADR 0003).

Bodies stay on the article. Asset links in the published body are rewritten to
`assets/<sha256>.<ext>`; document wikilinks are not. Only assets referenced by
the published document set are copied, so the share filter applies to binaries
too. Flattening assets by basename (experiment 024) collides; content-addressed
names do not.
