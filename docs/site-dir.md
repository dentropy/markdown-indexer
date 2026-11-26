# Site directory specification

This document is the contract for the directory written by:

```sh
markdown-tools-cli -dir <vault> -site-dir <site>
```

## Install markdown-tools-cli

The tool is the `main` package of [github.com/dentropy/markdown-indexer](https://github.com/dentropy/markdown-indexer), in [`libs/markdown-tools-cli`](https://github.com/dentropy/markdown-indexer/tree/main/libs/markdown-tools-cli). It requires Go 1.27.

```sh
go install github.com/dentropy/markdown-indexer/libs/markdown-tools-cli@latest
```

`go install` places the `markdown-tools-cli` binary in `$(go env GOPATH)/bin`. That directory must be on `PATH`.

From a clone of the same repository:

```sh
git clone https://github.com/dentropy/markdown-indexer.git
cd markdown-indexer/libs/markdown-tools-cli
go build -o markdown-tools-cli .
```

Run the built binary as `./markdown-tools-cli`, or copy it onto `PATH`. Then publish a vault:

```sh
markdown-tools-cli -dir <vault> -site-dir <site>
```

A frontend reads `<site>` over HTTP (or from disk) and renders it. It does not read the vault, and it does not need the Go source. The normative file shapes are also in:

- [`libs/markdown-indexer/docs/manifest.schema.json`](../libs/markdown-indexer/docs/manifest.schema.json)
- [`libs/markdown-indexer/docs/articles.schema.json`](../libs/markdown-indexer/docs/articles.schema.json)
- [`libs/markdown-indexer/docs/backlinks.schema.json`](../libs/markdown-indexer/docs/backlinks.schema.json)

Front matter objects are open JSON. There is no schema for a single asset byte file. The asset *index* schema (`assets.schema.json`) describes a different CLI mode (`-assets`). That index is not written into `-site-dir`.

The layout decision is recorded in [ADR 0006](../libs/markdown-indexer/docs/adr/0006-static-site-layout.md). This document is the rendering contract that follows from it.

## What the tree is

`<site>` is a static file tree. One vault scan produces it. There is no `index.html`, no router, and no bundled JavaScript. The frontend is a separate application that fetches these files.

```
<site>/
  directory.json
  articles/<uuid>.json
  frontmatter/<frontmatter_cid>.json
  assets/<sha256><ext>
  backlinks/<uuid>.json
```

`<uuid>` is the document identity, read from front matter. `<frontmatter_cid>` is a CIDv1 (dag-json, sha2-256) of that front matter. `<sha256>` is 64 lowercase hex characters, the sha256 of the asset bytes. `<ext>` is the original file extension, including the dot, lowercased. A file with no extension is published as the bare hash, with no dot.

Every JSON file is UTF-8, encoded with two-space indentation and a trailing newline. Parse it as JSON. The encoder escapes `<`, `>`, and `&` as `\u003c`, `\u003e`, and `\u0026`. A JSON parser yields the original characters. The same string may also contain `\n`, `\r`, and `\"`.

Paths inside JSON always use `/`, including on Windows.

## How a renderer loads one document

Treat `directory.json` as the catalog. Every published document is one object in `documents`. Render a document with four reads:

1. `directory.json`, to list documents and to build the name index below.
2. `articles/<uuid>.json`, for the body and the front matter address.
3. `frontmatter/<frontmatter_cid>.json`, for title and the rest of the metadata. The CID comes from the article. Cache this response forever: the filename is the content address, and a change produces a new CID and a new file.
4. `backlinks/<uuid>.json`, for incoming links. The file exists for every published document. `backlinks` is `[]` when nothing links to it.

Asset bytes are a fifth read, and only when the body (or the renderer) asks for one: `assets/<sha256><ext>`, taken from a markdown destination that starts with `assets/`. That URL is relative to `<site>/`, the directory that contains `directory.json`. It is not relative to `articles/`. Fetching `articles/assets/...` is wrong.

`raw_markdown_hash` and `modifiedUnix` are informational. They are not cache keys for the published article. `raw_markdown_hash` is the sha256 of the vault body *before* asset links were rewritten, so hashing the published `raw_markdown` disagrees with it whenever a rewrite happened. Front matter files and asset files are the ones that are safe to cache immutably. The hex in an asset filename is the sha256 of those bytes, and a client may check it. The front matter CID is *not* the hash of the pretty-printed file. Do not recompute it from the file.

## Generation

The CLI scans the vault, filters documents, rewrites asset links in memory, then writes the tree. `-site-dir` is exclusive with `-wikilinks`, `-backlinks`, `-manifest`, `-assets`, and `-out`.

### Scan

`markdown-indexer` walks `<vault>`:

- A markdown file matching `-glob` (default `**/*.md`) becomes a document. The default identity key is the front matter field `uuid` (`-idkey` changes the source field). The published JSON field is still called `uuid`.
- The YAML block between the opening `---` and the closing `---` is parsed and removed from the body. Leading spaces, tabs, CR, and LF after the closing delimiter are stripped. The rest of the file, including its original newlines, is `raw_markdown`. A file with no front matter has an empty metadata object.
- `frontmatter_cid` is the CIDv1 of that metadata encoded as DAG-JSON. Key order in the YAML does not change the CID. Identical metadata shares one CID.
- `raw_markdown_hash` is the hex sha256 of the stripped body.
- `name` is the file name with its final extension removed (`Notes/obsidian-link-syntax.md` → `obsidian-link-syntax`). `relativePath` is the vault-relative path.
- A file whose front matter does not parse is not a document. The default CLI aborts on the first such file. `-force` (or `-F`) skips it and publishes the rest.
- With `-site-dir`, every other regular file is hashed (sha256 of the bytes) unless a path segment starts with `.`, the file is markdown, or it is not a regular file. Symlinks are skipped. Hidden markdown can still be a document when it matches the glob. This hash is not an IPLD CID.

### Which documents are published

A document is published only when its identity string is non-empty. Documents with no `uuid` (and files with no front matter) are omitted from `directory.json`, `articles/`, `frontmatter/`, and `backlinks/`.

By default the CLI then keeps only documents whose front matter `share` value is true. A boolean `true` counts, as does a string `strconv.ParseBool` accepts: `1`, `t`, `T`, `TRUE`, `true`, `True`. Anything else, including a missing `share` key, `false`, `no`, or `yes`, is unpublished. `-include-unshared` keeps every identified document.

The share filter does not drop hashed files from the scan. It drops them from the site indirectly: an asset is copied only when a *published* body resolves a link to it. A binary referenced only by an unpublished note is not in `assets/`.

### Article identity when UUIDs collide

Documents are ordered by `relativePath`, byte-wise.

| Output | Duplicate UUIDs |
| --- | --- |
| `articles/<uuid>.json` | One file. The greatest `relativePath` wins. |
| `directory.json` | One entry per file. Both rows are kept, so a UUID can appear twice. |
| `backlinks/<uuid>.json` | One file. Source and target identity use the greatest path, matching the article. |
| `frontmatter/<cid>.json` | One file per distinct CID among every identified document, including a duplicate that lost the article slot. |

The catalog a renderer should trust for "what can I open" is `directory.json`. The body for a UUID is the article file. If a UUID is listed twice, both rows share that one body. Orphan front matter files (a CID no article points at) can exist in that collision case. Load front matter through `article.frontmatter_cid`. Do not treat a directory listing of `frontmatter/` as the document list.

### Body rewrite

`BuildArticles` copies the vault body. `RewriteAssetLinks` then rewrites that copy. The hash fields are not updated. Front matter bytes are not rewritten, because changing them would make `frontmatter_cid` a lie. Asset paths written only in front matter are not copied unless the body also resolves them.

The rewriter runs three passes, in this order, on the body only. It does not skip fenced code blocks or inline code. A reference inside a fence that resolves is rewritten and the asset is published.

**Pass 1. Wikilink embeds** `![[target]]`, `![[target#section]]`, `![[target|alias]]`, `![[target#section|alias]]`.

The pattern is `!\[\[([^\]\|#]+)(?:#([^\]\|]+))?(?:\|([^\]]+))?\]\]`. `target` and `alias` are trimmed. When `target` resolves to an asset, the span becomes `![alias](assets/<file>)`. An empty alias stays empty, so `![[diagram.png]]` becomes `![](assets/<sha256>.png)`. The `#section` group is discarded.

**Pass 2. Markdown images** `![alt](destination)`, where `destination` has no whitespace or closing parenthesis. The match is `\[([^\]\n]*)\]\(([^)\s]+)\)` and the character immediately before the `[` must be `!`. The `!` is kept. `alt` is copied as written, without trimming. When `destination` resolves, the image becomes `![alt](assets/<file>)`. A destination that does not resolve, including every URL, stays as written. A title after the URL (`![alt](file.png "title")`) does not match, because of the space, and is left as written. That file is not published by this reference.

**Pass 3. Wikilinks** `[[target]]`, `[[target#section]]`, `[[target|alias]]`, `[[target#section|alias]]`, same groups as the embed, without the leading `!`.

When `target` resolves to an asset, the span becomes a markdown *link*, not an image: `[title](assets/<file>)`. `title` is the alias, or the raw target when there is no alias. `[[manual.pdf|the manual]]` becomes `[the manual](assets/<sha256>.pdf)`. The section is discarded. A target that does not resolve stays as the original `[[...]]`, section and alias included.

Already-rewritten `assets/<sha256>.<ext>` destinations do not resolve a second time. The hash filename is not a vault path and is not a vault basename.

### Asset resolution

A target resolves to an asset, or it is left unchanged. There is no "best guess" file.

1. Trim. An empty target, or a target with a URL scheme, does not resolve.
2. Convert `\` to `/`, strip a leading `./` and a leading `/`, then apply path cleaning (`..` and `.` segments). A result of `.` or `..` does not resolve.
3. If the result does not start with `../`, and some indexed asset's vault-relative path equals it case-insensitively, that asset is the match.
4. Otherwise join the result onto the directory of the source document's `relativePath`. If that path stays inside the vault and matches an asset path case-insensitively, that asset is the match.
5. Otherwise take the basename. If exactly one asset has that basename, case-insensitively, it is the match. Zero matches, or two or more, do not resolve. An ambiguous basename is not published by that reference.

A URL scheme is the CommonMark autolink rule: 2 to 32 characters, an ASCII letter, then ASCII letters, digits, `+`, `-`, or `.`, then `:`, and the next byte is not a space. `https://example.com/a.png`, `mailto:ada@example.com`, and `ipfs://...` are schemes. `www.example.com`, `C:/Users/x`, and `Design: the indexer` are not. A space later in the string does not cancel the scheme (`https://example.com/a b` is still a URL).

The published file name is `<sha256>` plus `path.Ext` of the original file name, lowercased. `Photo.PNG` is published as `<sha256>.png`. `archive.tar.gz` is published as `<sha256>.gz` (the final extension only). Two vault files with the same bytes and the same extension share one site file. The same bytes with different extensions are two files.

### Backlinks

Backlinks are derived from the vault bodies. The asset rewrite changes only the article copy, so a resolved asset wikilink disappears from `raw_markdown` and still never becomes a backlink.

A backlink is a body `[[...]]` whose target resolved to another published document's UUID. The renderer does not recompute this. It reads `backlinks/<uuid>.json`.

Included:

- Body wikilinks only. The label in the file is the string `INTERNAL`.
- Both the source and the target are published documents with a UUID.
- The link target matched a document `name` exactly (see resolution below). The stored display text is the pipe alias, or the raw target when there is no alias.

Excluded:

- Self-links.
- Unresolved targets, including links to unpublished (unshared, or missing-UUID) documents.
- URL targets.
- Embeds (`![[...]]`), markdown images, and ordinary markdown links (`[text](dest)`).
- Anything that occurs only in front matter, including a `[[link]]` inside a front matter string.
- Inline `#tags`.

The same link written twice in one document is one backlink. Two aliases to the same target are two backlinks. Entries are sorted by `from_name`, then `title`, then `from_document_id`, compared as raw bytes, not with a locale.

`from_name` is the source document's display title: front matter `title` when that value is a non-empty string after trimming, otherwise the source `name`.

### Directory

`directory.json` is `{"documents":[...]}`. Each entry is `title`, `uuid`, and `path`, in that order. `path` is `relativePath`. `title` is the same display title as `from_name`. Entries are sorted by `path`, then `uuid`, as raw bytes. The array is present and empty when nothing is published. It is never `null`.

Duplicate UUIDs stay as separate rows. `directory.json` describes the files that were scanned, after the share filter.

### Write mechanics

The writer creates `<site>/`, `articles/`, `frontmatter/`, `assets/`, and `backlinks/` even when a directory ends up empty. Each file is written by a temp file in the same directory followed by a rename, so a reader does not see a partial JSON file. A file whose bytes already match is left untouched.

`-prune` defaults to true. It deletes regular files in `articles/`, `frontmatter/`, and `assets/` that are not part of this run, and it deletes regular `*.json` files in `backlinks/` whose stem is not a published UUID. It does not delete subdirectories, symlinks, or files that sit next to `directory.json`. `-prune=false` leaves stale files in place. A renderer should follow `directory.json` and the article CIDs, not a raw directory listing.

## File formats

### `directory.json`

```json
{
  "documents": [
    {
      "title": "First Post",
      "uuid": "123e4567-e89b-12d3-a456-426614174000",
      "path": "a/b/post.md"
    }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `title` | Display title. Non-empty front matter `title` string, otherwise the file name without `.md`. |
| `uuid` | Document identity. Also the article and backlinks file name. |
| `path` | Vault-relative path of the source file. Equals `relativePath` on the article. |

No other fields are present.

### `articles/<uuid>.json`

The article is the document without its metadata object. The body may already contain rewritten asset links.

```json
{
  "uuid": "123e4567-e89b-12d3-a456-426614174000",
  "relativePath": "a/b/post.md",
  "name": "post",
  "modifiedUnix": 1789506535,
  "frontmatter_cid": "baguqeerazmpneno3adsosk63nrbgcabb42h2xeyhgphmwx46skr4ms52natq",
  "raw_markdown": "See ![](assets/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png) and [[other]].\n",
  "raw_markdown_hash": "afb00ef5983c7babc20d3eb45871aa371ab15814e96a9fdc2bf978d9944ba527"
}
```

| Field | Meaning |
| --- | --- |
| `uuid` | Same as the file stem. |
| `relativePath` | Vault path, `/`-separated. |
| `name` | File name without the final extension. This is the wikilink lookup key. It is not the display title. |
| `modifiedUnix` | Source file mtime, unix seconds. Not milliseconds. |
| `frontmatter_cid` | Join to `frontmatter/<frontmatter_cid>.json`. Matches `^b[a-z2-7]+$`. |
| `raw_markdown` | Body to render, after the asset rewrite. |
| `raw_markdown_hash` | 64 hex chars. Sha256 of the vault body, not of this string, when any asset link was rewritten. |

No other fields are present. There is no `metadata`, `title`, `share`, or `tags` field on the article. Those live in the front matter file.

### `frontmatter/<frontmatter_cid>.json`

The parsed YAML mapping, re-encoded as a JSON object. Keys are sorted by raw bytes. Nested mappings become objects, sequences become arrays, booleans stay booleans, null stays null, and numbers stay JSON numbers. A YAML timestamp becomes an RFC 3339 string. Two documents with the same metadata share this file.

```json
{
  "share": true,
  "tags": ["go", "cli"],
  "title": "First Post",
  "uuid": "123e4567-e89b-12d3-a456-426614174000"
}
```

The set of keys is whatever the vault author wrote, plus keys the publisher's vault tools may have added (`uuid`, `share`). The renderer must tolerate unknown keys. Display title resolution is specified above; a non-string `title` (a number, a list) falls back to `article.name`.

`tags` in this object is the author's front matter field. It is not derived from inline `#tags` in the body, and the publisher does not read it as a tag index. Inline `#tags` are not a field here.

A wikilink or path stored in a front matter string is still the vault text. It was not rewritten to `assets/...`.

### `backlinks/<uuid>.json`

```json
{
  "document_id": "123e4567-e89b-12d3-a456-426614174000",
  "backlinks": [
    {
      "from_document_id": "223e4567-e89b-12d3-a456-426614174000",
      "from_name": "Other Note",
      "from_relative_path": "notes/other.md",
      "title": "the post",
      "label": "INTERNAL"
    }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `document_id` | Target UUID. Same as the file stem and as `articles/<uuid>.json`. |
| `from_document_id` | Source UUID. Open `articles/<from_document_id>.json`. |
| `from_name` | Source display title. |
| `from_relative_path` | Source vault path. |
| `title` | The link text the author wrote: pipe alias, otherwise the raw target. |
| `label` | Always `INTERNAL`. |

`backlinks` is always an array.

### `assets/<sha256><ext>`

The original file bytes. No JSON wrapper, no sidecar, no MIME map. The renderer infers the media type from `<ext>`. Typical values are `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`, `.svg`, `.pdf`, and audio or video extensions. Any non-markdown regular file can appear. Serve and render unknown extensions as downloads.

## Rendering `raw_markdown`

Render the body as Markdown. The publisher does not choose a flavor. Headings, lists, tables, emphasis, autolinks, and fenced code are the renderer's Markdown implementation.

On top of that, interpret the constructs the publisher either rewrote or deliberately left in place. Build a name index first.

### Name index

Walk `directory.json`'s `documents` in order (it is already sorted by path). For each row, derive `name` as the last path segment with its final extension removed (`a/b/post.md` → `post`). Map `name` → `uuid`, overwriting. The last row wins, which is the greatest path. That matches the publisher's resolution.

Also keep `path` → `uuid` if the renderer wants to recognize author-written markdown links. The publisher does not require that. It does require name resolution for the wikilinks it left behind.

Document names are case-sensitive. `[[Post]]` does not match `post`.

### What a rewritten body looks like

Vault body:

```markdown
See ![[diagram.png]] and ![alt](assets/diagram.png) and [[manual.pdf|the manual]] and [[other-note]] and ![[other-note]] and ![](https://example.com/a.png).
```

Published `raw_markdown`, given a unique `diagram.png` and `manual.pdf` in the vault:

```markdown
See ![](assets/<png-sha256>.png) and ![alt](assets/<png-sha256>.png) and [the manual](assets/<pdf-sha256>.pdf) and [[other-note]] and ![[other-note]] and ![](https://example.com/a.png).
```

`[[other-note]]` and `![[other-note]]` remain because those targets are documents, or missing. The remote image remains. A missing or ambiguous asset wikilink also remains, for example `![[gone.png]]`.

### Destinations the renderer must handle

Apply these rules to the published string.

**Site asset image.** `![alt](assets/<file>)`. Request `<site>/assets/<file>`. `alt` is the accessible name. This is how a resolved `![[image]]` and a resolved `![alt](vault-path)` both appear.

**Site asset link.** `[text](assets/<file>)`. Request the same way. This is how a resolved `[[file.pdf]]` or `[[file.pdf|text]]` appears. Render it as a link, not as an embedded image, unless the renderer chooses to inline a type it understands. The publisher used a link because the original syntax was a wikilink rather than an embed.

**Remote image or link.** A destination with a URL scheme, such as `![](https://example.com/a.png)` or `[text](mailto:ada@example.com)`. Use it as an absolute URL. Do not prefix `<site>/`.

**Document wikilink.** `[[target]]`, `[[target#section]]`, `[[target|alias]]`, `[[target#section|alias]]`.

- Trim `target` and `alias`.
- If `target` has a URL scheme, it is an external link to `target`. Display `alias` if present, otherwise `target`.
- Otherwise look `target` up in the name index. Do not strip a trailing `.md` on this form. `[[note.md]]` matches a document only when some file's `name` is the literal string `note.md`.
- On a hit, link to that UUID. Display `alias` if present, otherwise `target`.
- `#section` is the author's heading text, not a slug the publisher computed. The publisher has no heading-id algorithm. Scrolling to a heading is the renderer's choice. Dropping the section and opening the document is a correct rendering.
- On a miss, show the display text and do not invent a route. The target was unpublished, missing, or ambiguous at publish time. Unpublished targets are indistinguishable from typos in the output.

**Document embed.** `![[target]]`, `![[target#section]]`, `![[target|alias]]`, `![[target#section|alias]]`, still present in the body.

- If `target` has a URL scheme, treat it as a remote resource. The publisher left it unchanged.
- If `target` has no extension, or its final extension is `.md` in any letter case, it is a transclusion of another document. Strip one trailing `.md` (any case) and look that string up in the name index. `![[other-note]]` and `![[other-note.md]]` both look up `other-note`.
- On a hit, the publisher did not inline the other article. The renderer may fetch `articles/<uuid>.json` and inline that body, or render a link. Inlining is optional. If it inlines, use the same rules for the nested body, and guard cycles.
- If the target has any other extension, it is an asset reference that did not resolve (missing or ambiguous). There is no file to fetch. Show the alias or the target.
- `#section` and `alias` mean the same thing as on a document wikilink. `alias` is display text, not a file name.

**Ordinary markdown link.** `[text](destination)` with any other destination, including a vault path (`notes/other.md`), an absolute path (`/books/ikigai`), or a fragment. The publisher stored it as written and did not attach a UUID. Render it as a normal markdown link. Mapping `destination` onto a `path` in `directory.json` is an optional extra, not part of the contract.

**Inline tag.** `#name` in running text. The publisher does not emit a tag directory. Leaving the hash as text is a correct rendering. A renderer that builds a tag index must scan bodies itself. Do not treat the front matter `tags` field as that same index. The publisher's own notion of an inline tag, if the renderer wants to match it: not inside a fence or inline code, not a heading (`# Heading` has a space), not an all-digit body (`#1984`), and the `#` is not a continuation of a word, a wikilink section, or a URL fragment. Case is preserved. Matching this exactly is optional.

### Display title

For chrome (the window title, the catalog row, the backlink label), use this order:

1. Front matter `title`, when it is a string and is not empty after trimming.
2. `article.name`.

That is the string `directory.json` already stores as `title`, and the string backlinks store as `from_name`. Prefer the `directory.json` value for catalog rows so the renderer does not fetch every front matter file to paint the index.

## Worked join

```
GET <site>/directory.json
  → documents[].uuid = 123e4567-e89b-12d3-a456-426614174000
  → documents[].title = "First Post"
  → documents[].path = "a/b/post.md"

GET <site>/articles/123e4567-e89b-12d3-a456-426614174000.json
  → frontmatter_cid = baguqeera...
  → name = "post"
  → raw_markdown contains ![](assets/<sha>.png) and [[other-note]]

GET <site>/frontmatter/baguqeera....json
  → { "title": "First Post", "share": true, "uuid": "...", ... }

GET <site>/assets/<sha>.png
  → image bytes, named from the markdown destination

GET <site>/backlinks/123e4567-e89b-12d3-a456-426614174000.json
  → backlinks[].from_document_id opens another article
```

`[[other-note]]` opens whichever `directory.json` row, after last-path-wins, has the derived name `other-note`.

## What the tree does not contain

The renderer cannot fetch these, because the publisher does not write them:

- The edge graph (`-wikilinks` output). Outgoing document links are recovered by parsing `raw_markdown`.
- An asset index JSON. Asset URLs appear only inside published bodies.
- Per-document metadata inside the article file.
- A tag index, a search index, or an RSS/Atom feed.
- HTML, a checksum manifest, cache-control metadata, or CORS headers.
- Inlined transclusions. `![[other-note]]` is still that text.
- Rewritten document wikilinks. `[[other-note]]` is still that text.
- Vault files that failed resolution, and binaries referenced only from front matter or only from unpublished documents.

`from_document_id` in the backlinks schema comment mentions a route `/#/<uuid>`. That is an example from an older static app. This tree does not define a URL router. The renderer picks its own routes. The stable id is the UUID.

## Hosting

Serve `<site>` as static files. JSON responses should use `application/json`. Asset responses should use a media type inferred from the extension when the server can do that.

If the frontend's origin differs from the site's origin, the site host must send `Access-Control-Allow-Origin`. The publisher does not write that header.

A complete published document always has `articles/<uuid>.json` and `backlinks/<uuid>.json`. A fetch of either that 404s is an incomplete publish or a stale client catalog. Front matter 404s when the CID is wrong or the file was pruned. An asset 404s when the markdown destination was not produced by this publisher (the author wrote `assets/...` by hand and it did not match a published file).

## Flags that change the tree

| Flag | Effect on `<site>` |
| --- | --- |
| `-dir` | Vault root. Relative paths in the JSON are relative to this directory. |
| `-site-dir` | Output root. |
| `-glob` | Which markdown files are documents. Default `**/*.md`. |
| `-idkey` | Front matter field read as the UUID. Default `uuid`. Published field names stay `uuid`. |
| `-include-unshared` | Publish documents whose `share` is not true. Default is `share: true` only. |
| `-force`, `-F` | Skip unreadable front matter instead of aborting. Those files are absent. |
| `-prune` | Default true. Drop regular files in the four subdirectories that this run did not write. |

The renderer does not need to know which flags were used. `directory.json` is the published set either way. `share: false` inside a front matter file means the publisher was run with `-include-unshared` (or the field was true at publish time and the renderer is looking at a stale note). Do not hide a `directory.json` entry because `share` is false. The publisher already decided.
