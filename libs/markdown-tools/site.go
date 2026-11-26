package markdowntools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
)

// SiteResult reports what WriteSite wrote and pruned in each tree.
type SiteResult struct {
	ManifestWritten    bool
	ArticlesWritten    []string
	ArticlesRemoved    []string
	FrontMatterWritten []string
	FrontMatterRemoved []string
	AssetsWritten      []string
	AssetsRemoved      []string
	BacklinksWritten   []string
	BacklinksRemoved   []string
}

// WriteManifestFile writes manifest as indented JSON at path, skipping the
// write when the bytes already match.
func WriteManifestFile(manifest markdownindexer.Manifest, path string) (wrote bool, err error) {
	if strings.TrimSpace(path) == "" {
		return false, errors.New("manifest path must not be empty")
	}
	if manifest.Documents == nil {
		manifest.Documents = []markdownindexer.ManifestEntry{}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create manifest directory: %w", err)
	}
	existing, err := os.ReadFile(path)
	switch {
	case err == nil && bytes.Equal(existing, data):
		return false, nil
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if err := writeFileAtomic(path, data); err != nil {
		return false, err
	}
	return true, nil
}

// WriteArticles writes one article JSON file per UUID into dir, named
// <uuid>.json. The map key is authoritative for uuid. With Prune, leftover
// *.json files are removed.
func WriteArticles(articles map[string]markdownindexer.Article, dir string, opts WriteOptions) (written, removed []string, err error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil, errors.New("articles directory must not be empty")
	}
	ids := make([]string, 0, len(articles))
	var invalid []string
	for id := range articles {
		ids = append(ids, id)
		if err := checkBacklinkID(id); err != nil {
			invalid = append(invalid, fmt.Sprintf("%q (%v)", id, err))
		}
	}
	if len(invalid) > 0 {
		return nil, nil, fmt.Errorf("invalid article id(s): %s", strings.Join(invalid, ", "))
	}
	sort.Strings(ids)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create articles directory: %w", err)
	}
	keep := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		entry := articles[id]
		entry.UUID = id
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return written, nil, fmt.Errorf("encode article %s: %w", id, err)
		}
		data = append(data, '\n')
		name := id + ".json"
		keep[name] = struct{}{}
		path := filepath.Join(dir, name)
		existing, err := os.ReadFile(path)
		switch {
		case err == nil && bytes.Equal(existing, data):
			continue
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return written, nil, fmt.Errorf("read %s: %w", path, err)
		}
		if err := writeFileAtomic(path, data); err != nil {
			return written, nil, err
		}
		written = append(written, name)
	}
	if !opts.Prune {
		return written, nil, nil
	}
	removed, err = pruneNamed(dir, keep)
	return written, removed, err
}

// WriteFrontMatter writes one pretty-printed JSON file per front matter CID
// into dir. Duplicate CIDs share a file.
func WriteFrontMatter(docs []markdownindexer.Document, dir string, opts WriteOptions) (written, removed []string, err error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil, errors.New("frontmatter directory must not be empty")
	}
	byCID := map[string]json.RawMessage{}
	var invalid []string
	for _, d := range docs {
		if d.ID == "" {
			continue
		}
		cid := d.FrontmatterCID
		if err := checkBacklinkID(cid); err != nil {
			invalid = append(invalid, fmt.Sprintf("%q (%v)", cid, err))
			continue
		}
		byCID[cid] = d.Metadata
	}
	if len(invalid) > 0 {
		return nil, nil, fmt.Errorf("invalid frontmatter id(s): %s", strings.Join(invalid, ", "))
	}
	cids := make([]string, 0, len(byCID))
	for cid := range byCID {
		cids = append(cids, cid)
	}
	sort.Strings(cids)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create frontmatter directory: %w", err)
	}
	keep := make(map[string]struct{}, len(cids))
	for _, cid := range cids {
		var v any
		if len(byCID[cid]) == 0 {
			v = map[string]any{}
		} else if err := json.Unmarshal(byCID[cid], &v); err != nil {
			return written, nil, fmt.Errorf("decode front matter %s: %w", cid, err)
		}
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return written, nil, fmt.Errorf("encode front matter %s: %w", cid, err)
		}
		data = append(data, '\n')
		name := cid + ".json"
		keep[name] = struct{}{}
		path := filepath.Join(dir, name)
		existing, err := os.ReadFile(path)
		switch {
		case err == nil && bytes.Equal(existing, data):
			continue
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return written, nil, fmt.Errorf("read %s: %w", path, err)
		}
		if err := writeFileAtomic(path, data); err != nil {
			return written, nil, err
		}
		written = append(written, name)
	}
	if !opts.Prune {
		return written, nil, nil
	}
	removed, err = pruneNamed(dir, keep)
	return written, removed, err
}

// WriteSite publishes docs and ix.Assets into dir as the static JSON tree:
// directory.json, articles/, frontmatter/, assets/, backlinks/. Asset links in
// published article bodies are rewritten. Only assets that resolve from a
// published document are copied.
func WriteSite(docs []markdownindexer.Document, assets []markdownindexer.Asset, vault, dir string, opts WriteOptions) (SiteResult, error) {
	var res SiteResult
	if strings.TrimSpace(dir) == "" {
		return res, errors.New("site directory must not be empty")
	}
	ix := &markdownindexer.Index{Documents: docs, Assets: assets}

	articles := markdownindexer.BuildArticles(docs)
	used := map[string]markdownindexer.Asset{}
	for id, a := range articles {
		a.RawMarkdown = RewriteAssetLinks(a.RawMarkdown, a.RelativePath, ix, used)
		articles[id] = a
	}
	usedList := make([]markdownindexer.Asset, 0, len(used))
	for _, a := range used {
		usedList = append(usedList, a)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, fmt.Errorf("create site directory: %w", err)
	}

	wrote, err := WriteManifestFile(markdownindexer.BuildManifest(docs), filepath.Join(dir, "directory.json"))
	if err != nil {
		return res, err
	}
	res.ManifestWritten = wrote

	wopts := opts
	res.ArticlesWritten, res.ArticlesRemoved, err = WriteArticles(articles, filepath.Join(dir, "articles"), wopts)
	if err != nil {
		return res, err
	}
	res.FrontMatterWritten, res.FrontMatterRemoved, err = WriteFrontMatter(docs, filepath.Join(dir, "frontmatter"), wopts)
	if err != nil {
		return res, err
	}
	res.AssetsWritten, res.AssetsRemoved, err = WriteAssets(usedList, vault, filepath.Join(dir, "assets"), wopts)
	if err != nil {
		return res, err
	}
	res.BacklinksWritten, res.BacklinksRemoved, err = WriteBacklinks(
		markdownindexer.BuildBacklinks(docs),
		filepath.Join(dir, "backlinks"),
		BacklinksOptions{Prune: opts.Prune},
	)
	return res, err
}
