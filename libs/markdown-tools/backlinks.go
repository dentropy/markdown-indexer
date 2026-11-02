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

// BacklinksOptions controls how WriteBacklinks publishes a backlinks tree.
type BacklinksOptions struct {
	// Prune removes regular *.json files in the target directory that do not
	// belong to the set. The directory is treated as owned by the caller:
	// anything else in it, and every subdirectory, is left alone.
	Prune bool
}

// WriteBacklinks writes one JSON file per document into dir, named
// <document_id>.json. The map key is authoritative: it becomes both the file
// name and the document_id in the file, and a nil Backlinks slice is written
// as an empty array.
//
// Files are written atomically and only when their bytes change, so repeating
// the call leaves mtimes alone. With Prune, regular *.json files in dir that
// do not belong to the set are removed after the writes succeed; a directory
// with an empty set therefore ends up empty.
//
// Every document ID is validated before anything is written. An ID that cannot
// be a single file name (empty, ".", "..", a path separator, a drive colon, or
// a control character) fails the whole call and leaves the directory
// untouched. IDs come from front matter, so a vault must not be able to steer
// the writer outside dir.
//
// It returns the file names written and removed, both sorted.
func WriteBacklinks(backlinks map[string]markdownindexer.Backlinks, dir string, opts BacklinksOptions) (written, removed []string, err error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil, errors.New("backlinks directory must not be empty")
	}

	ids := make([]string, 0, len(backlinks))
	for id := range backlinks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var invalid []string
	for _, id := range ids {
		if err := checkBacklinkID(id); err != nil {
			invalid = append(invalid, fmt.Sprintf("%q (%v)", id, err))
		}
	}
	if len(invalid) > 0 {
		return nil, nil, fmt.Errorf("invalid backlink document id(s): %s", strings.Join(invalid, ", "))
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create backlinks directory: %w", err)
	}

	for _, id := range ids {
		entry := backlinks[id]
		entry.DocumentID = id
		if entry.Backlinks == nil {
			entry.Backlinks = []markdownindexer.Backlink{}
		}

		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return written, nil, fmt.Errorf("encode backlinks for %s: %w", id, err)
		}
		data = append(data, '\n')

		path := filepath.Join(dir, id+".json")
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
		written = append(written, id+".json")
	}

	if !opts.Prune {
		return written, nil, nil
	}
	removed, err = pruneBacklinks(dir, backlinks)
	if err != nil {
		return written, removed, err
	}
	return written, removed, nil
}

// checkBacklinkID rejects an ID that cannot be one file name inside a single
// directory.
func checkBacklinkID(id string) error {
	switch id {
	case "":
		return errors.New("empty")
	case ".", "..":
		return errors.New("not a file name")
	}
	if strings.ContainsAny(id, `/\:`) {
		return errors.New("contains a path separator or colon")
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return errors.New("contains a control character")
		}
	}
	if filepath.Base(id) != id {
		return errors.New("contains a path separator")
	}
	return nil
}

// writeFileAtomic writes data to a temporary file in path's directory and
// renames it into place, so a reader never sees a partial file.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".backlinks-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

// pruneBacklinks removes regular *.json files in dir whose stem is not a key
// in keep. Subdirectories, symlinks, and other file types are ignored.
func pruneBacklinks(dir string, keep map[string]markdownindexer.Backlinks) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read backlinks directory: %w", err)
	}

	var removed []string
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if _, ok := keep[strings.TrimSuffix(name, ".json")]; ok {
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
