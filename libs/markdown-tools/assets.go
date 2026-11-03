package markdowntools

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
)

// WriteOptions controls pruning for a published directory of files.
type WriteOptions struct {
	// Prune removes regular files in the target directory that do not belong
	// to the set. Subdirectories, symlinks, and other file types are left
	// alone.
	Prune bool
}

// AssetFileName is the published name of an asset: its sha256, plus the
// original extension lowercased.
func AssetFileName(a markdownindexer.Asset) string {
	return a.SHA256 + strings.ToLower(path.Ext(a.Name))
}

// WriteAssets copies each asset from vault into dir, named AssetFileName.
// Identical hashes with the same extension share a file. Files are written
// atomically and only when their bytes change. With Prune, regular files in
// dir that do not belong to the set are removed after the writes.
//
// It does not hash: SHA256 on each asset is authoritative. An empty or
// non-hex hash, or a name that cannot be a single file name, fails the whole
// call and leaves the directory untouched.
func WriteAssets(assets []markdownindexer.Asset, vault, dir string, opts WriteOptions) (written, removed []string, err error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil, errors.New("assets directory must not be empty")
	}
	if strings.TrimSpace(vault) == "" {
		return nil, nil, errors.New("vault directory must not be empty")
	}

	keep := make(map[string]markdownindexer.Asset, len(assets))
	names := make([]string, 0, len(assets))
	var invalid []string
	for _, a := range assets {
		name := AssetFileName(a)
		if err := checkAssetFileName(a, name); err != nil {
			invalid = append(invalid, fmt.Sprintf("%q (%v)", a.RelativePath, err))
			continue
		}
		if _, ok := keep[name]; !ok {
			names = append(names, name)
		}
		keep[name] = a
	}
	if len(invalid) > 0 {
		return nil, nil, fmt.Errorf("invalid asset id(s): %s", strings.Join(invalid, ", "))
	}
	sort.Strings(names)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create assets directory: %w", err)
	}

	outAbs, err := filepath.Abs(dir)
	if err != nil {
		return nil, nil, err
	}

	for _, name := range names {
		a := keep[name]
		src := filepath.Join(vault, filepath.FromSlash(a.RelativePath))
		srcAbs, err := filepath.Abs(src)
		if err != nil {
			return written, nil, err
		}
		if withinPath(outAbs, srcAbs) {
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return written, nil, fmt.Errorf("read asset %s: %w", a.RelativePath, err)
		}
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
	removed, err = pruneNamed(dir, keepNames(names))
	return written, removed, err
}

func checkAssetFileName(a markdownindexer.Asset, name string) error {
	if err := checkBacklinkID(name); err != nil {
		return err
	}
	raw, err := hex.DecodeString(a.SHA256)
	if err != nil || len(raw) != sha256Size {
		return errors.New("sha256 must be 64 hex characters")
	}
	return nil
}

const sha256Size = 32

func keepNames(names []string) map[string]struct{} {
	keep := make(map[string]struct{}, len(names))
	for _, n := range names {
		keep[n] = struct{}{}
	}
	return keep
}

// pruneNamed removes regular files in dir whose names are not in keep.
func pruneNamed(dir string, keep map[string]struct{}) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}
	var removed []string
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		name := entry.Name()
		if _, ok := keep[name]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return removed, fmt.Errorf("remove stale %s: %w", name, err)
		}
		removed = append(removed, name)
	}
	sort.Strings(removed)
	return removed, nil
}

func withinPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
