package markdownindexer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// ErrAssetNotFound reports a target that does not resolve to any indexed asset.
var ErrAssetNotFound = errors.New("asset not found")

// ErrAssetAmbiguous reports a target whose basename matches more than one
// indexed asset, so a unique file cannot be chosen.
var ErrAssetAmbiguous = errors.New("asset target matches more than one file")

// Asset is one non-markdown file in the vault as indexed. It is not a
// document: it has no UUID, no front matter, and no edges of its own. Its
// identity is the sha256 of its bytes; a rename leaves the same hash at a new
// RelativePath.
type Asset struct {
	// RelativePath is the path of the file relative to the vault root, always
	// using forward slashes regardless of OS.
	RelativePath string `json:"relativePath"`

	// Name is the file name, including its extension.
	Name string `json:"name"`

	// ModifiedUnix is the time the file was last edited, as a unix timestamp
	// in seconds.
	ModifiedUnix int64 `json:"modifiedUnix"`

	// SHA256 is the hex-encoded sha256 of the file's bytes. Identical files
	// share a hash. It is the same kind of hash as Document.RawMarkdownHash,
	// not an IPLD CID.
	SHA256 string `json:"sha256"`
}

// ByPath returns the assets keyed by relative path. Paths are unique in a
// vault walk, so each key holds one asset.
func (ix *Index) ByPath() map[string]Asset {
	idx := make(map[string]Asset, len(ix.Assets))
	for _, a := range ix.Assets {
		idx[a.RelativePath] = a
	}
	return idx
}

// ResolveAsset maps a wikilink or embed target to an indexed asset.
//
// Lookup order: an exact vault-relative path, then a path relative to
// fromRelPath's directory, then a unique basename. Path comparison is
// case-insensitive after slash normalization. A URL-scheme target is not an
// asset. fromRelPath may be empty; the relative-to-source step is then
// skipped.
//
// A unique match returns the asset and a nil error. No match returns
// ErrAssetNotFound. Two or more basename hits and no earlier unique path match
// returns ErrAssetAmbiguous.
func (ix *Index) ResolveAsset(target, fromRelPath string) (Asset, error) {
	t := strings.TrimSpace(target)
	if t == "" || hasURLScheme(t) {
		return Asset{}, ErrAssetNotFound
	}
	t = path.Clean(strings.TrimPrefix(filepathToSlash(t), "./"))
	t = strings.TrimPrefix(t, "/")
	if t == "." || t == ".." {
		return Asset{}, ErrAssetNotFound
	}

	if !strings.HasPrefix(t, "../") {
		if a, ok := ix.lookupAssetPath(t); ok {
			return a, nil
		}
	}

	if fromRelPath != "" {
		fromDir := path.Dir(filepathToSlash(fromRelPath))
		rel := path.Clean(path.Join(fromDir, t))
		if rel != ".." && !strings.HasPrefix(rel, "../") {
			if a, ok := ix.lookupAssetPath(rel); ok {
				return a, nil
			}
		}
	}

	base := path.Base(t)
	var hits []Asset
	for _, a := range ix.Assets {
		if strings.EqualFold(a.Name, base) {
			hits = append(hits, a)
		}
	}
	switch len(hits) {
	case 0:
		return Asset{}, ErrAssetNotFound
	case 1:
		return hits[0], nil
	default:
		return Asset{}, ErrAssetAmbiguous
	}
}

func (ix *Index) lookupAssetPath(rel string) (Asset, bool) {
	for _, a := range ix.Assets {
		if strings.EqualFold(a.RelativePath, rel) {
			return a, true
		}
	}
	return Asset{}, false
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// indexAsset reads a non-markdown file off disk and hashes its bytes.
func indexAsset(absPath, relPath string) (Asset, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return Asset{}, fmt.Errorf("stat asset: %w", err)
	}
	sum, err := hashFile(absPath)
	if err != nil {
		return Asset{}, err
	}
	return Asset{
		RelativePath: relPath,
		Name:         path.Base(relPath),
		ModifiedUnix: info.ModTime().Unix(),
		SHA256:       sum,
	}, nil
}

func hashFile(absPath string) (string, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("read asset: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash asset: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func isMarkdownName(name string) bool {
	return strings.EqualFold(path.Ext(name), ".md")
}

func isHiddenName(name string) bool {
	return strings.HasPrefix(name, ".")
}

// isHiddenPath reports whether any path segment is a hidden name, so
// .obsidian/plugins/x.png is not an asset while .obsidian/note.md can still
// match the document pattern.
func isHiddenPath(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if isHiddenName(seg) {
			return true
		}
	}
	return false
}

func isRegularFile(d fs.DirEntry) bool {
	return d.Type().IsRegular()
}
