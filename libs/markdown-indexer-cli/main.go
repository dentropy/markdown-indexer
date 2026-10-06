package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/dentropy/markdown-indexer/libs/markdown-index"
	"github.com/google/uuid"
)

const frontMatterDelimiter = "---"

type options struct {
	dir        string
	out        string
	pattern    string
	all        bool
	idKey      string
	checkDup   bool
	stripDup   bool
	memUsage   bool
	wikilinks  bool
	force      bool
	vaultCheck bool
	quiet      bool
}

// filterSharedDocs narrows the document set to the ones the CLI is allowed to
// hold: by default only documents whose front matter sets share: true, or the
// whole vault under -all.
func filterSharedDocs(docs []markdownindexer.Document, opts options) []markdownindexer.Document {
	if opts.all {
		return docs
	}
	keep := make([]markdownindexer.Document, 0, len(docs))
	for _, d := range docs {
		if markdownindexer.SharedMetadata(d.Metadata) {
			keep = append(keep, d)
		}
	}
	return keep
}

func main() {
	if err := runWith(os.Args[1:], os.Stdout, os.Stderr, os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	return runWith(args, stdout, io.Discard, strings.NewReader(""))
}

func runWith(args []string, stdout, stderr io.Writer, stdin io.Reader) error {
	opts, err := parseFlags(args)
	if err != nil {
		return err
	}

	if opts.vaultCheck {
		return vaultCheck(opts, stderr)
	}

	// -quiet silences the progress notices that share a stream with the index,
	// so stdout can be piped straight into jq. It never silences a fatal error:
	// those are returned and printed by main, and -vaultcheck reports on stderr
	// because that report is its output.
	notices := stderr
	if opts.quiet {
		notices = io.Discard
	}

	ix, err := scanVault(opts, notices)
	if err != nil {
		return err
	}
	docs := ix.Documents

	// Duplicate-UUID repair runs on the whole vault: a collision between two
	// unshared documents is still a collision worth fixing.
	if opts.stripDup {
		if err := stripDuplicateFrontMatter(docs, opts.dir, opts.idKey, stdin, stderr); err != nil {
			return err
		}
		ix, err = scanVault(opts, notices)
		if err != nil {
			return err
		}
		docs = ix.Documents
	}

	// By default only documents whose front matter sets share: true are loaded
	// in; -all widens the index to the whole vault.
	docs = filterSharedDocs(docs, opts)

	out := stdout
	if opts.out != "" {
		f, err := os.Create(opts.out)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")

	if opts.wikilinks {
		graph := buildWikilinkGraph(docs)
		if err := enc.Encode(graph); err != nil {
			return fmt.Errorf("encode wikilink graph: %w", err)
		}
	} else {
		if opts.force {
			docs = filterMarshalable(docs, func(d markdownindexer.Document) string {
				return d.RelativePath
			}, notices)
		}
		if err := enc.Encode(documentsIndex(docs)); err != nil {
			return fmt.Errorf("encode documents: %w", err)
		}
	}

	if opts.memUsage {
		bytesUsed := documentsMemoryUsage(docs)
		fmt.Fprintf(notices, "documents memory usage: %d bytes (%s)\n", bytesUsed, formatBytes(bytesUsed))
	}

	if opts.checkDup {
		ix := &markdownindexer.Index{Documents: docs}
		dups := ix.DuplicateUUIDs()
		for _, id := range dups {
			fmt.Fprintf(notices, "duplicate UUID %s -> %s\n", id, strings.Join(ix.PathsForID(id), ", "))
		}
		if len(dups) > 0 {
			return fmt.Errorf("found %d duplicate UUID(s): %s", len(dups), strings.Join(dups, ", "))
		}
	}
	return nil
}

// scanVault indexes every markdown file in the vault, reporting each file it
// could not read. Without -force the first broken file aborts the run, matching
// the default behaviour of failing loudly on a malformed vault. The share filter
// is not applied here: callers decide what to do with the whole vault.
func scanVault(opts options, stderr io.Writer) (*markdownindexer.Index, error) {
	ix, err := markdownindexer.Scan(opts.dir, markdownindexer.Options{Pattern: opts.pattern, IDKey: opts.idKey})
	if err != nil {
		return nil, err
	}
	if len(ix.Problems) == 0 {
		return ix, nil
	}
	if !opts.force {
		p := ix.Problems[0]
		return nil, fmt.Errorf("%s: %w", p.Path(), p.Err)
	}
	for _, p := range ix.Problems {
		fmt.Fprintf(stderr, "skipping %s: %v\n", p.Path(), p.Err)
	}
	return ix, nil
}

// filterMarshalable keeps every element of items that serializes to JSON
// without error, logging the ones that do not to stderr. With -force a single
// bad element (e.g. corrupt front matter that already slipped into metadata)
// would otherwise abort the entire output, so it is dropped instead.
func filterMarshalable[T any](items []T, name func(T) string, stderr io.Writer) []T {
	keep := items[:0]
	for _, it := range items {
		if _, err := json.Marshal(it); err != nil {
			fmt.Fprintf(stderr, "skipping %s: %v\n", name(it), err)
			continue
		}
		keep = append(keep, it)
	}
	return keep
}

// documentsIndex keys every document by its UUID, turning the JSON document
// output into an object instead of an array. Documents are visited in
// relative-path order, so a duplicated UUID deterministically resolves to the
// same entry on every run.
func documentsIndex(docs []markdownindexer.Document) map[string]markdownindexer.Document {
	return (&markdownindexer.Index{Documents: docs}).ByUUID()
}

func parseFlags(args []string) (options, error) {
	fs := flag.NewFlagSet("markdown-indexer-cli", flag.ContinueOnError)
	var opts options
	fs.StringVar(&opts.dir, "dir", ".", "directory to scan for markdown files")
	fs.StringVar(&opts.out, "out", "", "output file (defaults to stdout)")
	fs.StringVar(&opts.pattern, "pattern", "**/*.md", "glob pattern, relative to -dir")
	fs.BoolVar(&opts.all, "all", false, "load every document; by default only documents whose front matter sets `share: true` are loaded")
	fs.StringVar(&opts.idKey, "idkey", "uuid", "YAML front matter key holding the document UUID")
	fs.BoolVar(&opts.checkDup, "checkdups", false, "report documents that share the same UUID")
	fs.BoolVar(&opts.stripDup, "stripdups", false, "replace the UUID of every document with a duplicate UUID with a fresh one, keeping the rest of its front matter (prompts for confirmation)")
	fs.BoolVar(&opts.memUsage, "memusage", false, "print an estimate of the memory used by the documents struct")
	fs.BoolVar(&opts.wikilinks, "wikilinks", false, "output a JSON graph of wikilinks instead of the document index")
	fs.BoolVar(&opts.force, "force", false, "skip documents with broken front matter and index the rest instead of aborting")
	fs.BoolVar(&opts.force, "F", false, "shorthand for -force")
	fs.BoolVar(&opts.quiet, "quiet", false, "print only the JSON, suppressing progress notices on stderr")
	fs.BoolVar(&opts.quiet, "q", false, "shorthand for -quiet")
	fs.BoolVar(&opts.vaultCheck, "vaultcheck", false, "check the vault for broken front matter and duplicate UUIDs, then exit non-zero if any are found")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if opts.dir == "" {
		return options{}, errors.New("dir must not be empty")
	}
	if opts.pattern == "" {
		return options{}, errors.New("pattern must not be empty")
	}
	return opts, nil
}

// documentsMemoryUsage returns an estimate of the bytes held by the documents
// slice: the slice header, the contiguous backing array, and every string or
// byte slice its elements reference.
func documentsMemoryUsage(docs []markdownindexer.Document) uintptr {
	used := uintptr(24) // slice header
	used += uintptr(cap(docs)) * reflect.TypeOf(markdownindexer.Document{}).Size()
	for i := range docs {
		used += memrefs(reflect.ValueOf(&docs[i]).Elem())
	}
	return used
}

// memrefs estimates the heap memory referenced by a value on top of its
// container. String and slice payloads contribute their bytes; structs, maps,
// pointers and interfaces recurse into their fields. The value's own inline
// footprint is the caller's responsibility (e.g. the slice backing array).
func memrefs(v reflect.Value) uintptr {
	switch v.Kind() {
	case reflect.Struct:
		n := uintptr(0)
		for i := 0; i < v.NumField(); i++ {
			n += memrefs(v.Field(i))
		}
		return n
	case reflect.String:
		return uintptr(v.Len())
	case reflect.Slice:
		if v.IsNil() {
			return 0
		}
		n := uintptr(v.Cap()) * v.Type().Elem().Size()
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return n // []byte: no further references
		}
		for i := 0; i < v.Len(); i++ {
			n += memrefs(v.Index(i))
		}
		return n
	case reflect.Array:
		n := uintptr(0)
		for i := 0; i < v.Len(); i++ {
			n += memrefs(v.Index(i))
		}
		return n
	case reflect.Map:
		if v.IsNil() {
			return 0
		}
		n := uintptr(0)
		iter := v.MapRange()
		for iter.Next() {
			n += 8 + iter.Key().Type().Size() + iter.Value().Type().Size() +
				memrefs(iter.Key()) + memrefs(iter.Value())
		}
		return n
	case reflect.Interface:
		if v.IsNil() {
			return 0
		}
		return memrefs(v.Elem())
	case reflect.Pointer:
		if v.IsNil() {
			return 0
		}
		return v.Type().Elem().Size() + memrefs(v.Elem())
	default:
		return 0
	}
}

// formatBytes renders a byte count in a human readable unit.
func formatBytes(n uintptr) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := uint64(n) / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// vaultCheck scans the vault and reports structural problems to stderr:
// documents whose front matter fails to parse, duplicate UUIDs, and documents
// with front matter that is missing the UUID key. It returns an error carrying
// the problem count so a non-zero exit can gate CI.
func vaultCheck(opts options, stderr io.Writer) error {
	problems, err := markdownindexer.Check(opts.dir, markdownindexer.Options{Pattern: opts.pattern, IDKey: opts.idKey})
	if err != nil {
		return err
	}
	for _, p := range problems {
		switch err := p.Err.(type) {
		case *markdownindexer.DuplicateUUIDError:
			fmt.Fprintf(stderr, "duplicate UUID %s -> %s\n", err.UUID, strings.Join(err.Paths, ", "))
		case *markdownindexer.BrokenFrontMatterError:
			fmt.Fprintf(stderr, "%s: broken front matter: %v\n", p.Path(), err.Err)
		default:
			fmt.Fprintf(stderr, "%s: %v\n", p.Path(), p.Err)
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("vault check found %d problem(s)", len(problems))
	}
	fmt.Fprintln(stderr, "vault check: ok")
	return nil
}

// collidingDoc pairs a document's relative path with the duplicate UUID it
// owns.
type collidingDoc struct {
	Path string
	ID   string
}

// documentsWithDuplicateID returns every document that owns one of the given
// duplicated UUIDs, sorted by relative path.
func documentsWithDuplicateID(dups []string, docs []markdownindexer.Document) []collidingDoc {
	ix := &markdownindexer.Index{Documents: docs}
	out := make([]collidingDoc, 0)
	for _, id := range dups {
		for _, p := range ix.PathsForID(id) {
			out = append(out, collidingDoc{Path: p, ID: id})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// stripDuplicateFrontMatter lists every document sharing a duplicate UUID,
// asks for confirmation, and then replaces the UUID of each of them with a
// fresh one so the conflict is resolved while the rest of every document's
// front matter is left intact.
func stripDuplicateFrontMatter(docs []markdownindexer.Document, dir, idKey string, stdin io.Reader, stderr io.Writer) error {
	ix := &markdownindexer.Index{Documents: docs}
	dups := ix.DuplicateUUIDs()
	if len(dups) == 0 {
		fmt.Fprintln(stderr, "no duplicate UUIDs found; nothing to fix")
		return nil
	}
	colliding := documentsWithDuplicateID(dups, docs)

	fmt.Fprintf(stderr, "The following %d document(s) have a duplicate UUID. Each one will get a fresh UUID:\n", len(colliding))
	for _, c := range colliding {
		fmt.Fprintf(stderr, "  %s (uuid %s)\n", c.Path, c.ID)
	}
	ok, err := promptConfirm(stderr, stdin)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted by user; no files were modified")
	}
	for _, c := range colliding {
		abs := filepath.Join(dir, filepath.FromSlash(c.Path))
		if err := replaceFrontMatterUUID(abs, idKey); err != nil {
			return fmt.Errorf("fix uuid in %s: %w", c.Path, err)
		}
		fmt.Fprintf(stderr, "reassigned uuid: %s\n", c.Path)
	}
	return nil
}

// promptConfirm asks for a y/N answer on stderr and reads the reply from
// stdin. Only an explicit "y" or "yes" counts as confirmation.
func promptConfirm(stderr io.Writer, stdin io.Reader) (bool, error) {
	fmt.Fprint(stderr, "Continue? [y/N] ")
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

// replaceFrontMatterUUID assigns a fresh UUID to the given key in a file's
// front matter on disk, leaving the rest of the file byte for byte untouched.
func replaceFrontMatterUUID(abs, idKey string) error {
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	cleaned := setFrontMatterUUID(data, idKey, uuid.New().String())
	if bytes.Equal(cleaned, data) {
		return nil
	}
	return os.WriteFile(abs, cleaned, 0o644)
}

// setFrontMatterUUID returns the file content with the value of the top-level
// YAML key `key` replaced by `value`, preserving all other bytes of the file.
// If the front matter has no such key, the key is appended to it. Files
// without front matter are returned unchanged.
func setFrontMatterUUID(data []byte, key, value string) []byte {
	if !hasFrontMatter(data) {
		return data
	}

	body := data[len(frontMatterDelimiter):]
	yamlStart := len(frontMatterDelimiter)
	trimmed := bytes.TrimLeft(body, " \t")
	yamlStart += len(body) - len(trimmed)
	if len(trimmed) > 0 && (trimmed[0] == '\n' || trimmed[0] == '\r') {
		n := 1
		if trimmed[0] == '\r' && len(trimmed) > 1 && trimmed[1] == '\n' {
			n = 2
		}
		yamlStart += n
		trimmed = trimmed[n:]
	}

	end := findClosingDelimiter(trimmed)
	if end < 0 {
		return data
	}
	yamlEnd := yamlStart + end

	// The YAML block ends with the closing --- delimiter, so every line before
	// the last one is front-matter content.
	lines := bytes.Split(data[yamlStart:yamlEnd], []byte("\n"))
	if len(lines) == 0 {
		return data
	}
	content, closing := lines[:len(lines)-1], lines[len(lines)-1]

	keyBytes := []byte(key)
	replaced := false
	var block bytes.Buffer
	for _, line := range content {
		if !replaced {
			if valStart, ok := matchingKeyValueStart(line, keyBytes); ok {
				block.Write(line[:valStart])
				block.WriteString(value)
				block.WriteByte('\n')
				replaced = true
				continue
			}
		}
		block.Write(line)
		block.WriteByte('\n')
	}
	if !replaced {
		block.WriteString(key)
		block.WriteString(": ")
		block.WriteString(value)
		block.WriteByte('\n')
	}
	block.Write(closing)

	var out bytes.Buffer
	out.Write(data[:yamlStart])
	out.Write(block.Bytes())
	out.Write(data[yamlEnd:])
	return out.Bytes()
}

// hasFrontMatter reports whether the file content opens with a --- delimiter
// line.
func hasFrontMatter(data []byte) bool {
	if !bytes.HasPrefix(data, []byte(frontMatterDelimiter)) {
		return false
	}
	rest := data[len(frontMatterDelimiter):]
	if len(rest) == 0 {
		return false
	}
	c := rest[0]
	return c == '\n' || c == '\r' || c == ' ' || c == '\t'
}

// findClosingDelimiter returns the offset just past the closing --- line within
// body, or -1 when the block is never closed.
func findClosingDelimiter(body []byte) int {
	for _, line := range bytes.Split(body, []byte("\n")) {
		trimmed := bytes.TrimRight(line, " \t\r")
		if bytes.Equal(trimmed, []byte(frontMatterDelimiter)) {
			return bytes.Index(body, line) + len(line)
		}
	}
	return -1
}

// matchingKeyValueStart reports whether line is a top-level "<key>: value"
// front-matter entry (wrapping quotes around the key are tolerated) and
// returns the byte offset at which the value begins.
func matchingKeyValueStart(line, key []byte) (int, bool) {
	if len(line) == 0 || line[0] == ' ' || line[0] == '\t' || line[0] == '\r' || line[0] == '#' {
		return 0, false
	}
	rest := line
	quoted := false
	if rest[0] == '\'' || rest[0] == '"' {
		quoted = true
		rest = rest[1:]
	}
	if !bytes.HasPrefix(rest, key) {
		return 0, false
	}
	rest = rest[len(key):]
	if quoted {
		if len(rest) == 0 || (rest[0] != '\'' && rest[0] != '"') {
			return 0, false
		}
		rest = rest[1:]
	}
	i := 0
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t') {
		i++
	}
	if i >= len(rest) || rest[i] != ':' {
		return 0, false
	}
	rest = rest[i+1:]
	i = 0
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t') {
		i++
	}
	return len(line) - len(rest) + i, true
}

// buildWikilinkGraph scans every document for [[wikilinks]] and returns the
// graph as a map keyed by each edge's content address.
func buildWikilinkGraph(docs []markdownindexer.Document) map[string]markdownindexer.Edge {
	return markdownindexer.BuildEdges(docs)
}
