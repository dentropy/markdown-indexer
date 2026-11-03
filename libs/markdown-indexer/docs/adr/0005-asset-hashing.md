# Asset hashing is opt-in on Scan, and is not an edge field

Scan hashes non-markdown files only when `Options.HashAssets` is set. The
default walk stays documents and edges, so a caller that does not need binaries
does not read them. The hashes live on `Index.Assets`, not on `Edge`: putting
`sha256` on an asset edge would change that edge's content address. An
`INTERNAL_ASSET` target stays the raw path; `ResolveAsset` joins it to an
asset.

The hash is a hex sha256 of the file bytes, the same kind of hash as
`RawMarkdownHash`, not an IPLD CID. Front matter and edges stay CIDv1. The CLI
exposes the flag the same way it exposed backlinks: `-assets` prints the index
as JSON, `-assets-dir` writes the hashed files, and neither changes
`backlinks.schema.json`.
