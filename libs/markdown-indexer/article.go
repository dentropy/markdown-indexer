package markdownindexer

// Article is the published per-document JSON: identity, path, the front matter
// CID, and the markdown body, without the inlined front matter object. The
// body of a published article may be rewritten by a caller; RawMarkdownHash
// is always the indexer's hash of the vault body.
type Article struct {
	// UUID is the document's identifier. It is a field here because a
	// published file is self-describing, matching backlinks.document_id.
	UUID string `json:"uuid"`

	// RelativePath is Document.RelativePath.
	RelativePath string `json:"relativePath"`

	// Name is Document.Name, the file name without the .md extension.
	Name string `json:"name"`

	// ModifiedUnix is Document.ModifiedUnix.
	ModifiedUnix int64 `json:"modifiedUnix"`

	// FrontmatterCID is Document.FrontmatterCID, the pointer to the published
	// front matter object.
	FrontmatterCID string `json:"frontmatter_cid"`

	// RawMarkdown is the markdown body. BuildArticles copies the vault body;
	// a publisher may overwrite this with a rewritten body.
	RawMarkdown string `json:"raw_markdown"`

	// RawMarkdownHash is the hex sha256 of the vault body, even when
	// RawMarkdown has been rewritten.
	RawMarkdownHash string `json:"raw_markdown_hash"`
}

// Articles derives one Article per identified document in the index, keyed by
// UUID. Duplicate UUIDs keep the greatest relative path, matching ByUUID.
func (ix *Index) Articles() map[string]Article {
	return BuildArticles(ix.Documents)
}

// BuildArticles lists the documents as articles, the same way Index.Articles
// does without a vault walk.
func BuildArticles(docs []Document) map[string]Article {
	out := make(map[string]Article, len(docs))
	for _, d := range docs {
		if d.ID == "" {
			continue
		}
		out[d.ID] = Article{
			UUID:            d.ID,
			RelativePath:    d.RelativePath,
			Name:            d.Name,
			ModifiedUnix:    d.ModifiedUnix,
			FrontmatterCID:  d.FrontmatterCID,
			RawMarkdown:     d.RawMarkdown,
			RawMarkdownHash: d.RawMarkdownHash,
		}
	}
	return out
}
