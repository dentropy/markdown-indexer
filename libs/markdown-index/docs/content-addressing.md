# Content addressing

Front matter and edges are hashed the same way: a CIDv1 over DAG-JSON, with a
sha2-256 multihash. An edge is *keyed* by its address in `Index.Edges`; front
matter *carries* its address in `Document.FrontmatterCID`.

Identical input always yields the same address, and DAG-JSON canonicalizes map
ordering, so two front matter blocks that differ only in key order share an
address. `RawMarkdownHash` is a plain hex sha256 of the body, not an IPLD
address — it identifies content, but it is not a CID and is not used as a key.

The same addresses show up as keys and fields when the index leaves this package
as JSON; the shapes are in [JSON schema](json-schema.md).

Back to the [README](../README.md).
