package markdowntools

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
	"github.com/google/uuid"
)

// InitFrontMatter adds missing front matter to every document in ix. A
// document without a UUID gets one under idKey, and a document without a
// share key gets the value pointed to by share, or false when share is nil.
// Existing UUIDs and share values are left alone.
//
// It returns the relative paths of the files it rewrote.
func InitFrontMatter(ix *markdownindexer.Index, dir, idKey string, share *bool) ([]string, error) {
	shareValue := false
	if share != nil {
		shareValue = *share
	}

	var changed []string
	for _, doc := range ix.Documents {
		frontMatter := doc.FrontMatter()
		_, hasShare := frontMatter["share"]
		ok, err := rewriteDocument(dir, doc, func(data []byte) ([]byte, bool, error) {
			rewritten := false
			if doc.ID == "" {
				next, err := SetFrontMatterValue(data, idKey, uuid.New().String())
				if err != nil {
					return nil, false, err
				}
				data = next
				rewritten = true
			}
			if !hasShare {
				next, err := SetFrontMatterValue(data, "share", strconv.FormatBool(shareValue))
				if err != nil {
					return nil, false, err
				}
				data = next
				rewritten = true
			}
			return data, rewritten, nil
		})
		if err != nil {
			return changed, err
		}
		if ok {
			changed = append(changed, doc.RelativePath)
		}
	}
	return changed, nil
}

// SetShare sets the share key on every document in ix to share and gives every
// document without a UUID a fresh one under idKey. Existing UUIDs and all
// other front matter are preserved. A value that already means the requested
// permission is left byte-for-byte alone.
//
// It returns the relative paths of the files it rewrote.
func SetShare(ix *markdownindexer.Index, dir, idKey string, share bool) ([]string, error) {
	var changed []string
	for _, doc := range ix.Documents {
		frontMatter := doc.FrontMatter()
		_, hasShare := frontMatter["share"]
		needsShare := !hasShare || markdownindexer.SharedMetadata(doc.Metadata) != share

		ok, err := rewriteDocument(dir, doc, func(data []byte) ([]byte, bool, error) {
			rewritten := false
			if doc.ID == "" {
				next, err := SetFrontMatterValue(data, idKey, uuid.New().String())
				if err != nil {
					return nil, false, err
				}
				data = next
				rewritten = true
			}
			if needsShare {
				next, err := SetFrontMatterValue(data, "share", strconv.FormatBool(share))
				if err != nil {
					return nil, false, err
				}
				data = next
				rewritten = true
			}
			return data, rewritten, nil
		})
		if err != nil {
			return changed, err
		}
		if ok {
			changed = append(changed, doc.RelativePath)
		}
	}
	return changed, nil
}

// ReplaceDuplicateUUIDs gives every document sharing a UUID a fresh one under
// idKey. Documents with unique UUIDs are not touched, and the rest of each
// rewritten file's front matter is preserved.
//
// It returns the relative paths of the files it rewrote.
func ReplaceDuplicateUUIDs(ix *markdownindexer.Index, dir, idKey string) ([]string, error) {
	duplicates := make(map[string]bool)
	for _, id := range ix.DuplicateUUIDs() {
		duplicates[id] = true
	}

	var changed []string
	for _, doc := range ix.Documents {
		if !duplicates[doc.ID] {
			continue
		}
		ok, err := rewriteDocument(dir, doc, func(data []byte) ([]byte, bool, error) {
			next, err := SetFrontMatterValue(data, idKey, uuid.New().String())
			if err != nil {
				return nil, false, err
			}
			return next, true, nil
		})
		if err != nil {
			return changed, err
		}
		if ok {
			changed = append(changed, doc.RelativePath)
		}
	}
	return changed, nil
}

// rewriteDocument reads one document, applies fn, and writes the result back
// only when fn reports a change and the bytes actually differ.
func rewriteDocument(dir string, doc markdownindexer.Document, fn func([]byte) ([]byte, bool, error)) (bool, error) {
	abs := filepath.Join(dir, filepath.FromSlash(doc.RelativePath))
	data, err := os.ReadFile(abs)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", doc.RelativePath, err)
	}
	out, changed, err := fn(data)
	if err != nil {
		return false, fmt.Errorf("%s: %w", doc.RelativePath, err)
	}
	if !changed || bytes.Equal(out, data) {
		return false, nil
	}
	if err := os.WriteFile(abs, out, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", doc.RelativePath, err)
	}
	return true, nil
}
