package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
	"github.com/dentropy/markdown-indexer/libs/markdown-tools"
)

type options struct {
	dir             string
	out             string
	pattern         string
	all             bool
	idKey           string
	checkDup        bool
	stripDup        bool
	initFrontMatter bool
	fixFrontMatter  bool
	setShare        bool
	share           string
	memUsage        bool
	manifest        bool
	wikilinks       bool
	backlinks       bool
	backlinksDir    string
	id              string
	prune           bool
	force           bool
	vaultCheck      bool
	quiet           bool
}

// shareValue parses -share into a boolean. The second return value reports
// whether the flag was set at all, so -initfrontmatter can tell "leave the
// default false" apart from an explicit request.
func (o options) shareValue() (bool, bool, error) {
	if o.share == "" {
		return false, false, nil
	}
	v, err := strconv.ParseBool(o.share)
	if err != nil {
		return false, false, fmt.Errorf("parse -share: %w", err)
	}
	return v, true, nil
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

	// Quoting repair walks the vault as raw bytes: a file whose front matter
	// does not parse never reaches an index, so it has to run before Scan.
	if opts.fixFrontMatter {
		if err := repairBrokenFrontMatter(opts, stdin, stderr); err != nil {
			return err
		}
	}

	ix, err := scanVault(opts, notices)
	if err != nil {
		return err
	}

	// Duplicate-UUID repair runs on the whole vault: a collision between two
	// unshared documents is still a collision worth fixing.
	if opts.stripDup {
		if err := stripDuplicateFrontMatter(ix, opts.dir, opts.idKey, stdin, stderr); err != nil {
			return err
		}
		if ix, err = scanVault(opts, notices); err != nil {
			return err
		}
	} else if opts.initFrontMatter || opts.setShare {
		changed, err := rewriteFrontMatter(ix, opts, notices)
		if err != nil {
			return err
		}
		if len(changed) > 0 {
			if ix, err = scanVault(opts, notices); err != nil {
				return err
			}
		}
	}
	docs := ix.Documents

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

	switch {
	case opts.backlinks && opts.backlinksDir != "":
		if err := publishBacklinks(docs, opts, notices); err != nil {
			return err
		}
	case opts.backlinks && opts.id != "":
		backlinks := markdownindexer.BuildBacklinks(docs)
		entry, ok := backlinks[opts.id]
		if !ok {
			return fmt.Errorf("unknown document id: %s", opts.id)
		}
		if err := enc.Encode(entry); err != nil {
			return fmt.Errorf("encode backlinks: %w", err)
		}
	case opts.backlinks:
		if err := enc.Encode(markdownindexer.BuildBacklinks(docs)); err != nil {
			return fmt.Errorf("encode backlinks: %w", err)
		}
	case opts.wikilinks:
		graph := buildWikilinkGraph(docs)
		if err := enc.Encode(graph); err != nil {
			return fmt.Errorf("encode wikilink graph: %w", err)
		}
	case opts.manifest:
		if err := enc.Encode(markdownindexer.BuildManifest(docs)); err != nil {
			return fmt.Errorf("encode manifest: %w", err)
		}
	default:
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

// rewriteFrontMatter applies -initfrontmatter or -setshare through
// markdowntools, reporting each rewritten file and the total.
func rewriteFrontMatter(ix *markdownindexer.Index, opts options, notices io.Writer) ([]string, error) {
	share, shareSet, err := opts.shareValue()
	if err != nil {
		return nil, err
	}

	var changed []string
	if opts.setShare {
		changed, err = markdowntools.SetShare(ix, opts.dir, opts.idKey, share)
	} else {
		var override *bool
		if shareSet {
			override = &share
		}
		changed, err = markdowntools.InitFrontMatter(ix, opts.dir, opts.idKey, override)
	}
	if err != nil {
		return changed, err
	}
	for _, p := range changed {
		fmt.Fprintf(notices, "updated front matter: %s\n", p)
	}
	fmt.Fprintf(notices, "%d file(s) updated\n", len(changed))
	return changed, nil
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
	fs := flag.NewFlagSet("markdown-tools-cli", flag.ContinueOnError)
	var opts options
	fs.StringVar(&opts.dir, "dir", ".", "directory to scan for markdown files")
	fs.StringVar(&opts.out, "out", "", "output file (defaults to stdout)")
	fs.StringVar(&opts.pattern, "glob", "**/*.md", "glob pattern, relative to -dir")
	fs.BoolVar(&opts.all, "include-unshared", false, "load every document; by default only documents whose front matter sets `share: true` are loaded")
	fs.StringVar(&opts.idKey, "idkey", "uuid", "YAML front matter key holding the document UUID")
	fs.BoolVar(&opts.checkDup, "check-duplicates", false, "report documents that share the same UUID")
	fs.BoolVar(&opts.stripDup, "fix-duplicates", false, "replace the UUID of every document with a duplicate UUID with a fresh one, keeping the rest of its front matter (prompts for confirmation)")
	fs.BoolVar(&opts.fixFrontMatter, "fix-frontmatter", false, "quote plain YAML scalars that contain a colon-space and close unbalanced quotes in flow sequences so the front matter parses (prompts for confirmation)")
	fs.BoolVar(&opts.initFrontMatter, "initfrontmatter", false, "add missing uuid and share front matter to every matched document (share defaults to false)")
	fs.BoolVar(&opts.setShare, "setshare", false, "set share on every matched document and add a uuid where one is missing; requires -share")
	fs.StringVar(&opts.share, "share", "", "share value for -initfrontmatter and -setshare (true or false)")
	fs.BoolVar(&opts.memUsage, "memusage", false, "print an estimate of the memory used by the documents struct")
	fs.BoolVar(&opts.manifest, "manifest", false, "output a single JSON file listing every processed document's title, uuid, and path")
	fs.BoolVar(&opts.wikilinks, "wikilinks", false, "output a JSON graph of wikilinks instead of the document index")
	fs.BoolVar(&opts.backlinks, "backlinks", false, "output each document's incoming links instead of the document index")
	fs.StringVar(&opts.backlinksDir, "backlinks-dir", "", "write one backlinks JSON file per document into this directory (implies -backlinks)")
	fs.StringVar(&opts.id, "id", "", "with -backlinks, output only this document's backlinks file")
	fs.BoolVar(&opts.prune, "prune", true, "with -backlinks-dir, remove *.json files for documents no longer published")
	fs.BoolVar(&opts.force, "force", false, "skip documents with broken front matter and index the rest instead of aborting")
	fs.BoolVar(&opts.force, "F", false, "shorthand for -force")
	fs.BoolVar(&opts.quiet, "quiet", false, "print only the JSON, suppressing progress notices on stderr")
	fs.BoolVar(&opts.quiet, "q", false, "shorthand for -quiet")
	fs.BoolVar(&opts.vaultCheck, "validate", false, "check the vault for broken front matter and duplicate UUIDs, then exit non-zero if any are found")
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
	if opts.backlinksDir != "" {
		opts.backlinks = true
	}
	outputModes := 0
	for _, on := range []bool{opts.wikilinks, opts.backlinks, opts.manifest} {
		if on {
			outputModes++
		}
	}
	if outputModes > 1 {
		return options{}, errors.New("-wikilinks, -backlinks, and -manifest are mutually exclusive")
	}
	if opts.backlinksDir != "" && opts.out != "" {
		return options{}, errors.New("-backlinks-dir cannot be combined with -out")
	}
	if opts.id != "" && !opts.backlinks {
		return options{}, errors.New("-id only applies with -backlinks")
	}
	if opts.id != "" && opts.backlinksDir != "" {
		return options{}, errors.New("-id cannot be combined with -backlinks-dir")
	}
	writeModes := 0
	for _, on := range []bool{opts.stripDup, opts.fixFrontMatter, opts.initFrontMatter, opts.setShare} {
		if on {
			writeModes++
		}
	}
	if writeModes > 1 {
		return options{}, errors.New("-fix-duplicates, -fix-frontmatter, -initfrontmatter, and -setshare are mutually exclusive")
	}
	if opts.setShare && opts.share == "" {
		return options{}, errors.New("-setshare requires -share=true or -share=false")
	}
	if opts.share != "" && !opts.initFrontMatter && !opts.setShare {
		return options{}, errors.New("-share only applies with -initfrontmatter or -setshare")
	}
	if _, _, err := opts.shareValue(); err != nil {
		return options{}, err
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

// repairBrokenFrontMatter quotes plain YAML scalars that contain a colon
// followed by a space, the usual reason a vault's front matter fails to parse.
// It walks the files directly, lists what it would change, asks for
// confirmation, and then rewrites. Files it cannot repair are reported and
// left untouched; Scan still sees them afterwards.
func repairBrokenFrontMatter(opts options, stdin io.Reader, stderr io.Writer) error {
	planned, err := markdowntools.RepairFrontMatter(opts.dir, markdowntools.RepairOptions{Pattern: opts.pattern})
	if err != nil {
		return err
	}
	for _, p := range planned.Unrepairable {
		fmt.Fprintf(stderr, "could not repair %s: %v\n", p.Path, p.Err)
	}
	if len(planned.Fixes) == 0 {
		if len(planned.Unrepairable) == 0 {
			fmt.Fprintln(stderr, "no repairable front matter found; nothing to fix")
		}
		return nil
	}

	fmt.Fprintf(stderr, "The following %d document(s) have invalid YAML scalars. Each one will be quoted in place:\n", len(planned.Fixes))
	for _, f := range planned.Fixes {
		fmt.Fprintf(stderr, "  %s (%s)\n", f.Path, strings.Join(f.Fields, ", "))
	}
	ok, err := promptConfirm(stderr, stdin)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted by user; no files were modified")
	}

	applied, err := markdowntools.RepairFrontMatter(opts.dir, markdowntools.RepairOptions{Pattern: opts.pattern, Apply: true})
	if err != nil {
		return err
	}
	for _, f := range applied.Fixes {
		fmt.Fprintf(stderr, "quoted front matter: %s (%s)\n", f.Path, strings.Join(f.Fields, ", "))
	}
	return nil
}

// stripDuplicateFrontMatter lists every document sharing a duplicate UUID,
// asks for confirmation, and then replaces the UUID of each of them with a
// fresh one so the conflict is resolved while the rest of every document's
// front matter is left intact.
func stripDuplicateFrontMatter(ix *markdownindexer.Index, dir, idKey string, stdin io.Reader, stderr io.Writer) error {
	dups := ix.DuplicateUUIDs()
	if len(dups) == 0 {
		fmt.Fprintln(stderr, "no duplicate UUIDs found; nothing to fix")
		return nil
	}
	colliding := documentsWithDuplicateID(dups, ix.Documents)

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

	changed, err := markdowntools.ReplaceDuplicateUUIDs(ix, dir, idKey)
	if err != nil {
		return err
	}
	for _, p := range changed {
		fmt.Fprintf(stderr, "reassigned uuid: %s\n", p)
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

// buildWikilinkGraph scans every document for [[wikilinks]] and returns the
// graph as a map keyed by each edge's content address.
func buildWikilinkGraph(docs []markdownindexer.Document) map[string]markdownindexer.Edge {
	return markdownindexer.BuildEdges(docs)
}

// publishBacklinks writes one backlinks file per document through
// markdowntools and reports what changed. -quiet silences the notices.
func publishBacklinks(docs []markdownindexer.Document, opts options, notices io.Writer) error {
	written, removed, err := markdowntools.WriteBacklinks(
		markdownindexer.BuildBacklinks(docs),
		opts.backlinksDir,
		markdowntools.BacklinksOptions{Prune: opts.prune},
	)
	if err != nil {
		return err
	}
	for _, name := range written {
		fmt.Fprintf(notices, "wrote backlinks: %s\n", filepath.Join(opts.backlinksDir, name))
	}
	for _, name := range removed {
		fmt.Fprintf(notices, "removed stale: %s\n", filepath.Join(opts.backlinksDir, name))
	}
	fmt.Fprintf(notices, "%d backlink file(s) written, %d removed\n", len(written), len(removed))
	return nil
}
