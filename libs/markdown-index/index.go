package markdownindexer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Defaults applied to the zero value of Options.
const (
	// DefaultPattern matches every markdown file at any depth.
	DefaultPattern = "**/*.md"

	// DefaultIDKey is the front matter key read by Options.IDKey when unset.
	DefaultIDKey = "uuid"
)

// Options controls what a scan reads. Its zero value loads every markdown file
// at any depth, keyed by the "uuid" front matter field.
type Options struct {
	// Pattern is a glob relative to the vault root, supporting ** recursion.
	// Defaults to DefaultPattern.
	Pattern string

	// IDKey is the YAML front matter key holding the document UUID. Defaults to
	// DefaultIDKey.
	IDKey string

	// Shared narrows the index to documents whose front matter sets share: true.
	// It is off by default: an index is the whole vault unless the caller asks
	// for less.
	Shared bool
}

func (o Options) pattern() string {
	if o.Pattern == "" {
		return DefaultPattern
	}
	return filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(o.Pattern, "./"), "./"))
}

func (o Options) idKey() string {
	if o.IDKey == "" {
		return DefaultIDKey
	}
	return o.IDKey
}

// Problem is one structural fault in a vault, together with why. A scan
// collects problems instead of aborting on the first bad file, so a caller
// learns about every fault in one pass.
type Problem struct {
	// Paths are the documents the problem concerns, in relative-path order. A
	// fault in one document names it; a duplicate UUID names every document
	// that collides.
	Paths []string

	// Err is what went wrong.
	Err error
}

// Path is the first document the problem concerns, or the empty string when it
// concerns none.
func (p Problem) Path() string {
	if len(p.Paths) == 0 {
		return ""
	}
	return p.Paths[0]
}

func (p Problem) Error() string { return strings.Join(p.Paths, ", ") + ": " + p.Err.Error() }

func (p Problem) Unwrap() error { return p.Err }

// ErrMissingUUID reports front matter that parsed but carries no value under
// the configured ID key. The document is still indexed, with an empty ID.
var ErrMissingUUID = errors.New("missing uuid in front matter")

// ErrBrokenFrontMatter reports a file whose YAML front matter did not parse.
// Match it with errors.Is.
var ErrBrokenFrontMatter = errors.New("broken front matter")

// BrokenFrontMatterError is a file whose front matter did not parse. Its
// message is the underlying cause (a YAML error or a missing closing ---); the
// broken-front-matter framing is what errors.Is matches on, so it is not
// repeated in the text.
type BrokenFrontMatterError struct {
	Err error
}

func (e *BrokenFrontMatterError) Error() string { return e.Err.Error() }

func (e *BrokenFrontMatterError) Unwrap() error { return e.Err }

func (e *BrokenFrontMatterError) Is(target error) bool { return target == ErrBrokenFrontMatter }

// DuplicateUUIDError reports documents that share one UUID.
type DuplicateUUIDError struct {
	// UUID is the contested identifier.
	UUID string

	// Paths are the documents holding it.
	Paths []string
}

func (e *DuplicateUUIDError) Error() string {
	return "duplicate UUID " + e.UUID
}

// Index is the in-memory collection of documents and edges derived from a
// vault: the derived, queryable form of what the vault contains.
type Index struct {
	// Documents holds every indexed document, ordered by relative path.
	Documents []Document

	// Edges holds the resolved wikilink graph, keyed by content address.
	Edges map[string]Edge

	// Problems holds the files that could not be indexed. A non-empty slice
	// does not invalidate the index; it means the vault has broken files.
	Problems []Problem
}

// ByUUID returns the documents keyed by UUID. Documents without a UUID are
// keyed by the empty string, which is how a caller can find them, and a
// duplicated UUID resolves deterministically to the document with the
// greatest relative path.
func (ix *Index) ByUUID() map[string]Document {
	idx := make(map[string]Document, len(ix.Documents))
	for _, d := range ix.Documents {
		idx[d.ID] = d
	}
	return idx
}

// DuplicateUUIDs returns the sorted UUIDs held by more than one document. UUIDs
// that are empty are ignored: a document with no UUID is not colliding with
// every other document that has none.
func (ix *Index) DuplicateUUIDs() []string {
	counts := make(map[string]int)
	for _, d := range ix.Documents {
		if d.ID != "" {
			counts[d.ID]++
		}
	}
	var dups []string
	for id, n := range counts {
		if n > 1 {
			dups = append(dups, id)
		}
	}
	sort.Strings(dups)
	return dups
}

// PathsForID returns the relative paths of every document holding the given
// UUID.
func (ix *Index) PathsForID(id string) []string {
	var paths []string
	for _, d := range ix.Documents {
		if d.ID == id {
			paths = append(paths, d.RelativePath)
		}
	}
	return paths
}

// Err reports the scan problems as one error, or nil when the vault indexed
// cleanly.
func (ix *Index) Err() error {
	if len(ix.Problems) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(ix.Problems))
	for _, p := range ix.Problems {
		msgs = append(msgs, p.Error())
	}
	return fmt.Errorf("%d problem(s): %s", len(ix.Problems), strings.Join(msgs, "; "))
}

// Scan walks the vault at dir and indexes every markdown file matching
// opts.Pattern. Files whose front matter does not parse become Problems on the
// returned index rather than aborting the scan; a non-nil error means the
// vault itself could not be read.
func Scan(dir string, opts Options) (*Index, error) {
	ix := &Index{Edges: map[string]Edge{}}
	pattern := opts.pattern()

	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if !MatchGlob(pattern, name) {
			return nil
		}

		doc, err := indexFile(p, name, opts.idKey())
		if err != nil {
			ix.Problems = append(ix.Problems, Problem{
				Paths: []string{name},
				Err:   &BrokenFrontMatterError{Err: err},
			})
			return nil
		}
		ix.Documents = append(ix.Documents, doc)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(ix.Documents, func(i, j int) bool {
		return ix.Documents[i].RelativePath < ix.Documents[j].RelativePath
	})

	if opts.Shared {
		ix.Documents = filterSharedDocs(ix.Documents)
	}
	ix.Edges = buildGraph(ix.Documents)
	return ix, nil
}

// Check scans the vault and reports its structural problems: files whose front
// matter does not parse, documents sharing a UUID, and documents with front
// matter that is missing the ID key. It returns one Problem per issue; an empty
// slice means the vault is clean. Files with no front matter at all are
// tolerated and not reported.
func Check(dir string, opts Options) ([]Problem, error) {
	ix, err := Scan(dir, opts)
	if err != nil {
		return nil, err
	}

	problems := append([]Problem(nil), ix.Problems...)
	for _, id := range ix.DuplicateUUIDs() {
		problems = append(problems, Problem{
			Paths: ix.PathsForID(id),
			Err:   &DuplicateUUIDError{UUID: id, Paths: ix.PathsForID(id)},
		})
	}
	for _, d := range ix.Documents {
		if d.ID == "" && string(d.Metadata) != "{}" {
			problems = append(problems, Problem{
				Paths: []string{d.RelativePath},
				Err:   fmt.Errorf("%w: %s", ErrMissingUUID, opts.idKey()),
			})
		}
	}
	return problems, nil
}

// indexFile reads a markdown file off disk and indexes it.
func indexFile(absPath, relPath, idKey string) (Document, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return Document{}, fmt.Errorf("read file: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return Document{}, fmt.Errorf("stat file: %w", err)
	}
	return Parse(data, relPath, info.ModTime(), idKey)
}

// filterSharedDocs narrows the document set to the ones whose front matter
// marks them shareable with share: true. The value may be a JSON boolean or a
// string strconv.ParseBool accepts, so YAML spellings such as `share: true`,
// `share: TRUE`, and quoted `share: "true"` all count.
func filterSharedDocs(docs []Document) []Document {
	keep := make([]Document, 0, len(docs))
	for _, d := range docs {
		if SharedMetadata(d.Metadata) {
			keep = append(keep, d)
		}
	}
	return keep
}

// SharedMetadata reports whether the front matter JSON marks the document as
// shareable with a share: true entry. The value may be a JSON boolean or a
// string strconv.ParseBool accepts, so YAML spellings such as `share: true`,
// `share: TRUE`, and quoted `share: "true"` all count.
func SharedMetadata(metadata json.RawMessage) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(metadata, &m); err != nil {
		return false
	}
	raw, ok := m["share"]
	if !ok {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		v, _ := strconv.ParseBool(s)
		return v
	}
	return false
}
