# Body embed edges: INTERNAL_ASSET, EXTERNAL_ASSET, EMBEDDED_MARKDOWN

Body-only embed edges for `![[target]]` and `![alt](dest)`, classified by URL
scheme / file extension. Front matter embeds are deliberately unchanged.

## Classification (body only)

| Syntax | Target shape | Label |
|---|---|---|
| `![[t]]`, `![[t#s]]`, `![[t\|alias]]` | URL scheme | `EXTERNAL_ASSET` |
| same | no extension or `.md` (case-insensitive) | `EMBEDDED_MARKDOWN` |
| same | any other extension | `INTERNAL_ASSET` |
| `![alt](dest)` | URL scheme | `EXTERNAL_ASSET` |
| same | anything else | `INTERNAL_ASSET` |

Markdown image syntax never yields `EMBEDDED_MARKDOWN` — it is not a
transclusion. Fields follow existing conventions: Title is the alias / alt text
or the target; FromDocumentID only for docs with a UUID; `EMBEDDED_MARKDOWN`
resolves `ToDocumentID` via the name→ID map (trailing `.md` trimmed first,
since document Names are filenames without `.md`); assets keep the raw target
as written — Scan only indexes `**/*.md`, so assets are never documents.

## Changes

### `libs/markdown-index/edge.go`

1. Add constants next to the existing six: `labelInternalAsset =
   "INTERNAL_ASSET"`, `labelExternalAsset = "EXTERNAL_ASSET"`,
   `labelEmbeddedMarkdown = "EMBEDDED_MARKDOWN"`. Update the Edge struct doc
   comments (Label/Title/ToDocumentID) to cover the new shapes.
2. Add `wikilinkEmbedRegex` = `!\[\[([^\]\|#]+)(?:#([^\]\|]+))?(?:\|([^\]]+))?\]\]`
   (same capture groups as `wikilinkRegex`, with a leading `!`) and
   `wikilinkEmbedMaskRegex` = `!\[\[[^\]]*\]\]`.
3. Add `isMarkdownEmbedTarget(target string) bool` — `path.Ext` empty or
   EqualFold `.md` (add `path/filepath` import).
4. Add `appendWikilinkEmbeds(edges, content, fromID, nameToID)` — classify per
   the table; resolve EMBEDDED_MARKDOWN through `nameToID` after trimming a
   trailing `.md`; assets keep the raw target.
5. Add `appendMarkdownImageEmbeds(edges, content, fromID)` — reuse
   `markdownLinkRegex` with `FindAllStringSubmatchIndex` and the leading-`!`
   byte check already used in `appendMarkdownLinks` (inverted: instead of
   skipping, emit an asset edge).
6. Add `maskWikilinkEmbeds(content string) string` — replaces each `![[...]]`
   span with spaces so the plain wikilink pass does not re-match the inner
   `[[...]]` (fixes `![[image.png]]` currently also producing an `INTERNAL`
   edge).
7. `collectEdges` body section becomes:
   ```go
   edges = appendWikilinkEmbeds(edges, doc.RawMarkdown, doc.ID, nameToID)
   edges = appendMarkdownImageEmbeds(edges, doc.RawMarkdown, doc.ID)
   edges = appendWikilinks(edges, maskWikilinkEmbeds(doc.RawMarkdown), doc.ID, nameToID, labelInternal, labelURL)
   ```
   Front matter section untouched: `![[banner.png]]` in front matter stays
   `FRONTMATTER_INTERNAL` (TestBuildGraphFrontMatterWikilink stays green),
   `![](x)` in front matter stays skipped — per user decision (body only).

### `libs/markdown-index/document_test.go` — new tests

- Wikilink embed asset: body `![[image.png]]` → exactly one edge,
  `INTERNAL_ASSET` (regression: must not also emit `INTERNAL`).
- Wikilink embed markdown: `![[note]]` with a `note` doc → `EMBEDDED_MARKDOWN`
  resolving to the UUID; `![[gone]]` (missing) → `EMBEDDED_MARKDOWN` with raw
  target; `![[note.md]]` → also resolves.
- Wikilink embed URL: `![[https://example.com/a.png]]` → `EXTERNAL_ASSET`.
- Markdown image embeds: `![](assets/x.png)` → `INTERNAL_ASSET`;
  `![alt](assets/x.png)` → Title `alt`; `![](https://example.com/a.png)` →
  `EXTERNAL_ASSET`; a plain body `[text](url)` still produces no edge.
- Same `![[banner.png]]` in body and front matter → two edges
  (`INTERNAL_ASSET` + `FRONTMATTER_INTERNAL`), two distinct CIDs.

### Docs

- `docs/wikilinks.schema.json`: add the three enum values; extend the `label`
  description (and `title`/`to_document_id` where embeds differ).
- `docs/usage.md`: replace "Image embeds (`![](...)`), reference-style links,
  bare URL fields..." in the front matter paragraph — front matter embeds are
  still not edges — and add a short body paragraph describing the three labels
  and the extension rule.
- `docs/vocabulary.md` + `CONTEXT.md`: add **Asset** (an embedded
  non-markdown target; kept as written, not a document) and **Markdown embed**
  (`![[note]]`, a body embed of another document's content); update the **Edge**
  entry to list body embed edges.
- `docs/adr/0002-body-embed-edges.md`: record the decision — body embeds get
  their own labels; extension-based classification (deterministic, no vault
  lookup); front matter left unchanged; existing edge content addresses stay
  stable because the label is part of the CID preimage (ADR 0001 pattern).

### Explicitly not changed

- `ExtractWikilinks` still matches the inner target of an embed
  (`![[img.png]]` → `img.png`) — public helper, out of scope.
- Front matter embeds (body-only labels per user decision).
- Body plain markdown links `[text](url)` still produce no edges.

## Verification

```sh
go test ./... && go vet ./...   # in libs/markdown-index
go test ./...                   # in libs/markdown-indexer-cli (fixture vault
                                # exercises the new syntax; tests assert no
                                # exact full-vault edge counts, so none break)
```
