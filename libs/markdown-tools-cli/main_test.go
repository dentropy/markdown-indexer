package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
)

func TestRunWithCheckDups(t *testing.T) {
	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", "testdata", "-glob", "*.md", "-check-duplicates"}, &stdout, &stderr, strings.NewReader(""))
	if err == nil {
		t.Fatal("want error when duplicate UUIDs are present")
	}
	if !strings.Contains(stderr.String(), "123e4567-e89b-12d3-a456-426614174000") {
		t.Errorf("stderr missing duplicate id: %s", stderr.String())
	}
	if !strings.Contains(err.Error(), "123e4567-e89b-12d3-a456-426614174000") {
		t.Errorf("error should mention the duplicate UUID, got: %s", err)
	}
	var docs map[string]markdownindexer.Document
	if json.Unmarshal([]byte(stdout.String()), &docs) != nil || len(docs) != 4 {
		t.Errorf("stdout should still contain the full index (testdata has 6 files; the default share filter loads 5, which dedupe to 4 UUIDs): %s", stdout.String())
	}
}

func TestExamplesDuplicateUUIDs(t *testing.T) {
	const dupUUID = "123e4567-e89b-12d3-a456-426614174000"
	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", "examples", "-check-duplicates"}, &stdout, &stderr, strings.NewReader(""))

	if err == nil {
		t.Fatal("want error since examples/duplicate.md duplicates a UUID")
	}
	if !strings.Contains(err.Error(), dupUUID) {
		t.Errorf("error should list the duplicated UUID, got: %s", err)
	}
	if !strings.Contains(stderr.String(), "duplicate.md, a/b/post.md") &&
		!strings.Contains(stderr.String(), "a/b/post.md, duplicate.md") {
		t.Errorf("stderr should name both colliding files: %s", stderr.String())
	}

	var docs map[string]markdownindexer.Document
	if json.Unmarshal([]byte(stdout.String()), &docs) != nil {
		t.Fatalf("stdout should be a valid index: %s", stdout.String())
	}
	if len(docs) != 2 {
		t.Errorf("examples has 4 markdown files but the default share filter loads 3 (no-frontmatter.md is excluded) with 2 unique UUIDs, got %d entries: %s", len(docs), stdout.String())
	}
	if _, ok := docs[dupUUID]; !ok {
		t.Errorf("the duplicated UUID should still appear as a map key: %s", stdout.String())
	}
}

func writeTestMD(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func parseTestDoc(t *testing.T, dir, name, idKey string) (markdownindexer.Document, string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := markdownindexer.Parse(data, name, time.Time{}, idKey)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return doc, string(data)
}

func TestInitFrontMatter(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "no-frontmatter.md", "Body only.\n")
	writeTestMD(t, dir, "missing-keys.md", "---\ntitle: Missing\n---\n\nBody.\n")
	writeTestMD(t, dir, "already-shared.md", "---\nuuid: keep-me\nshare: true\n---\n\nBody.\n")
	writeTestMD(t, dir, "nested/note.md", "---\nuuid: nested-id\nshare: false\n---\n\nNested.\n")

	var stdout, stderr strings.Builder
	if err := runWith([]string{"-dir", dir, "-initfrontmatter"}, &stdout, &stderr, strings.NewReader("")); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}

	doc, raw := parseTestDoc(t, dir, "no-frontmatter.md", "uuid")
	if doc.ID == "" {
		t.Errorf("no-frontmatter.md should get a UUID: %s", raw)
	}
	if _, ok := doc.FrontMatter()["share"]; !ok {
		t.Errorf("no-frontmatter.md should get a share key: %s", raw)
	}
	if markdownindexer.SharedMetadata(doc.Metadata) {
		t.Errorf("no-frontmatter.md share should default to false: %s", raw)
	}
	if !strings.Contains(raw, "Body only.") {
		t.Errorf("no-frontmatter.md lost its body: %s", raw)
	}

	doc, raw = parseTestDoc(t, dir, "missing-keys.md", "uuid")
	if doc.ID == "" {
		t.Errorf("missing-keys.md should get a UUID: %s", raw)
	}
	if _, ok := doc.FrontMatter()["share"]; !ok {
		t.Errorf("missing-keys.md should get a share key: %s", raw)
	}
	if markdownindexer.SharedMetadata(doc.Metadata) {
		t.Errorf("missing-keys.md share should default to false: %s", raw)
	}
	if !strings.Contains(raw, "title: Missing") {
		t.Errorf("missing-keys.md lost its front matter: %s", raw)
	}

	doc, raw = parseTestDoc(t, dir, "already-shared.md", "uuid")
	if doc.ID != "keep-me" {
		t.Errorf("already-shared.md should keep its UUID, got %q: %s", doc.ID, raw)
	}
	if !markdownindexer.SharedMetadata(doc.Metadata) {
		t.Errorf("already-shared.md should keep share: true: %s", raw)
	}

	doc, raw = parseTestDoc(t, dir, "nested/note.md", "uuid")
	if doc.ID != "nested-id" {
		t.Errorf("nested/note.md should keep its UUID, got %q: %s", doc.ID, raw)
	}
	if markdownindexer.SharedMetadata(doc.Metadata) {
		t.Errorf("nested/note.md should keep share: false: %s", raw)
	}
}

func TestInitFrontMatterShareTrue(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "missing-share.md", "---\nuuid: one\n---\n\nBody.\n")
	writeTestMD(t, dir, "existing-false.md", "---\nuuid: two\nshare: false\n---\n\nBody.\n")

	var stdout, stderr strings.Builder
	if err := runWith([]string{"-dir", dir, "-initfrontmatter", "-share=true"}, &stdout, &stderr, strings.NewReader("")); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}

	doc, raw := parseTestDoc(t, dir, "missing-share.md", "uuid")
	if !markdownindexer.SharedMetadata(doc.Metadata) {
		t.Errorf("missing-share.md should get share: true: %s", raw)
	}
	doc, raw = parseTestDoc(t, dir, "existing-false.md", "uuid")
	if markdownindexer.SharedMetadata(doc.Metadata) {
		t.Errorf("existing-false.md should keep share: false: %s", raw)
	}
}

func TestInitFrontMatterCustomIDKey(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "note.md", "---\ntitle: Custom\n---\n\nBody.\n")

	var stdout, stderr strings.Builder
	if err := runWith([]string{"-dir", dir, "-initfrontmatter", "-idkey", "id"}, &stdout, &stderr, strings.NewReader("")); err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}

	doc, raw := parseTestDoc(t, dir, "note.md", "id")
	if doc.ID == "" {
		t.Errorf("note.md should get an id key: %s", raw)
	}
	if strings.Contains(raw, "uuid:") {
		t.Errorf("note.md should use the configured id key, not uuid: %s", raw)
	}
}

func TestSetShareRecursive(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "no-frontmatter.md", "Body only.\n")
	writeTestMD(t, dir, "shared.md", "---\nuuid: keep-a\nshare: true\n---\n\nShared body.\n")
	writeTestMD(t, dir, "nested/private.md", "---\nuuid: keep-b\nshare: false\n---\n\nPrivate body.\n")
	writeTestMD(t, dir, "nested/deep/upper.md", "---\nuuid: keep-c\nshare: TRUE\n---\n\nUpper body.\n")

	run := func(share string) {
		t.Helper()
		var stdout, stderr strings.Builder
		if err := runWith([]string{"-dir", dir, "-setshare", "-share=" + share}, &stdout, &stderr, strings.NewReader("")); err != nil {
			t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
		}
	}

	run("false")
	for _, name := range []string{"no-frontmatter.md", "shared.md", "nested/private.md", "nested/deep/upper.md"} {
		doc, raw := parseTestDoc(t, dir, name, "uuid")
		if doc.ID == "" {
			t.Errorf("%s should have a UUID: %s", name, raw)
		}
		if _, ok := doc.FrontMatter()["share"]; !ok {
			t.Errorf("%s should have a share key: %s", name, raw)
		}
		if markdownindexer.SharedMetadata(doc.Metadata) {
			t.Errorf("%s should be share: false: %s", name, raw)
		}
	}
	for name, want := range map[string]string{
		"shared.md":            "keep-a",
		"nested/private.md":    "keep-b",
		"nested/deep/upper.md": "keep-c",
	} {
		doc, raw := parseTestDoc(t, dir, name, "uuid")
		if doc.ID != want {
			t.Errorf("%s should keep UUID %q, got %q: %s", name, want, doc.ID, raw)
		}
	}
	for name, want := range map[string]string{
		"shared.md":            "Shared body.",
		"nested/private.md":    "Private body.",
		"nested/deep/upper.md": "Upper body.",
	} {
		_, raw := parseTestDoc(t, dir, name, "uuid")
		if !strings.Contains(raw, want) {
			t.Errorf("%s lost its body: %s", name, raw)
		}
	}

	before := make(map[string]string)
	for _, name := range []string{"no-frontmatter.md", "shared.md", "nested/private.md", "nested/deep/upper.md"} {
		_, raw := parseTestDoc(t, dir, name, "uuid")
		before[name] = raw
	}
	run("false")
	for name, want := range before {
		_, raw := parseTestDoc(t, dir, name, "uuid")
		if raw != want {
			t.Errorf("%s changed on a repeated -setshare run:\nbefore %q\nafter  %q", name, want, raw)
		}
	}

	run("true")
	for _, name := range []string{"no-frontmatter.md", "shared.md", "nested/private.md", "nested/deep/upper.md"} {
		doc, raw := parseTestDoc(t, dir, name, "uuid")
		if !markdownindexer.SharedMetadata(doc.Metadata) {
			t.Errorf("%s should be share: true: %s", name, raw)
		}
	}
}

func TestWriteFlagValidation(t *testing.T) {
	cases := [][]string{
		{"-dir", ".", "-setshare"},
		{"-dir", ".", "-initfrontmatter", "-setshare", "-share=true"},
		{"-dir", ".", "-fix-duplicates", "-initfrontmatter"},
		{"-dir", ".", "-share=true"},
		{"-dir", ".", "-initfrontmatter", "-share=maybe"},
	}
	for _, args := range cases {
		if _, err := parseFlags(args); err == nil {
			t.Errorf("parseFlags(%v) should fail", args)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   uintptr
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.00 KiB"},
		{1728, "1.69 KiB"},
		{1048576, "1.00 MiB"},
	}
	for _, c := range cases {
		if got := formatBytes(c.in); got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDocumentsMemoryUsage(t *testing.T) {
	docs := []markdownindexer.Document{
		{RelativePath: "a.md", Name: "a", RawMarkdown: strings.Repeat("x", 100), RawMarkdownHash: strings.Repeat("0", 64)},
		{RelativePath: "b.md", Name: "b", RawMarkdown: "", FrontmatterCID: "baguq"},
	}
	used := documentsMemoryUsage(docs)
	if used < 100+64 {
		t.Errorf("memory usage %d should at least cover the payloads", used)
	}
	if used <= 24+2*reflect.TypeOf(markdownindexer.Document{}).Size() {
		t.Errorf("memory usage %d should cover the slice header and backing array", used)
	}
}

func TestRunWithMemUsage(t *testing.T) {
	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", "testdata", "-glob", "*.md", "-memusage"}, &stdout, &stderr, strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "documents memory usage:") {
		t.Errorf("stderr missing memory usage line: %q", stderr.String())
	}
	var docs map[string]markdownindexer.Document
	if json.Unmarshal([]byte(stdout.String()), &docs) != nil {
		t.Fatalf("stdout should still be a valid index: %s", stdout.String())
	}
	if len(docs) != 4 {
		t.Errorf("testdata has 6 markdown files; the default share filter loads 5, which dedupe to 4 UUIDs, got %d", len(docs))
	}
}

func TestStripDuplicateFrontMatter(t *testing.T) {
	dir := t.TempDir()
	oneContent := "---\nuuid: 11111111-1111-1111-1111-111111111111\nshare: true\ntitle: Uno\n---\n\nOne.\n"
	twoContent := "---\nuuid: 11111111-1111-1111-1111-111111111111\nshare: true\ntitle: Deux\n---\n\nTwo.\n"
	writeTestMD(t, dir, "one.md", oneContent)
	writeTestMD(t, dir, "two.md", twoContent)
	writeTestMD(t, dir, "three.md", "---\nuuid: 22222222-2222-2222-2222-222222222222\nshare: true\ntitle: Tres\n---\n\nThree.\n")

	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", dir, "-fix-duplicates", "-check-duplicates"}, &stdout, &stderr, strings.NewReader("y\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "one.md (uuid 11111111-1111-1111-1111-111111111111)") ||
		!strings.Contains(stderr.String(), "two.md (uuid 11111111-1111-1111-1111-111111111111)") {
		t.Errorf("stderr should list each path with its UUID before the prompt: %s", stderr.String())
	}

	newUUIDs := make(map[string]string)
	for _, name := range []string{"one.md", "two.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "11111111-1111-1111-1111-111111111111") {
			t.Errorf("%s should have its duplicate UUID replaced: %s", name, data)
		}
		if !strings.Contains(string(data), "title:") {
			t.Errorf("%s should keep its front matter: %s", name, data)
		}
		id := uuidFromFrontMatter(t, data)
		if id == "" || id == "11111111-1111-1111-1111-111111111111" {
			t.Errorf("%s should have a fresh UUID, got %q: %s", name, id, data)
		}
		if prev, ok := newUUIDs[id]; ok {
			t.Errorf("%s collides with new uuid %s (also on %s)", name, id, prev)
		}
		newUUIDs[id] = name
	}
	three, err := os.ReadFile(filepath.Join(dir, "three.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(three), "22222222-2222-2222-2222-222222222222") {
		t.Errorf("three.md is unique and must keep its UUID: %s", three)
	}

	var docs map[string]markdownindexer.Document
	if json.Unmarshal([]byte(stdout.String()), &docs) != nil {
		t.Fatalf("stdout should be a valid index: %s", stdout.String())
	}
	if len(docs) != 3 {
		t.Errorf("stdout should hold one entry per unique UUID, got %d: %s", len(docs), stdout.String())
	}
	if _, ok := docs["22222222-2222-2222-2222-222222222222"]; !ok {
		t.Errorf("three.md should be keyed by its UUID: %s", stdout.String())
	}
	for key, d := range docs {
		if key == "" {
			t.Errorf("unexpected empty map key for %s", d.RelativePath)
		}
		if d.RelativePath == "one.md" || d.RelativePath == "two.md" {
			if key == "" || key == "11111111-1111-1111-1111-111111111111" {
				t.Errorf("%s should have a fresh unique id, got map key %q", d.RelativePath, key)
			}
			if prev, ok := newUUIDs[key]; ok {
				if prev != d.RelativePath {
					t.Errorf("%s and %s share the new id %s", prev, d.RelativePath, key)
				}
			}
		}
	}
}

// uuidFromFrontMatter parses the uuid key out of a raw markdown file for the
// tests above.
func uuidFromFrontMatter(t *testing.T, data []byte) string {
	t.Helper()
	doc, err := markdownindexer.Parse(data, "test.md", time.Time{}, "uuid")
	if err != nil {
		t.Fatalf("parse front matter: %v", err)
	}
	return doc.ID
}

func TestStripDuplicateFrontMatterAborts(t *testing.T) {
	dir := t.TempDir()
	content := "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nHello.\n"
	writeTestMD(t, dir, "one.md", content)
	writeTestMD(t, dir, "two.md", content)

	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", dir, "-fix-duplicates"}, &stdout, &stderr, strings.NewReader("n\n"))
	if err == nil {
		t.Fatal("want error when the user declines")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("error should mention the abort, got: %s", err)
	}
	for _, name := range []string{"one.md", "two.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != content {
			t.Errorf("%s must be unchanged after abort: %q", name, data)
		}
	}
}

func TestRunWithWikilinks(t *testing.T) {
	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", "testdata", "-glob", "*.md", "-wikilinks"}, &stdout, &stderr, strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var graph map[string]markdownindexer.Edge
	if err := json.Unmarshal([]byte(stdout.String()), &graph); err != nil {
		t.Fatalf("stdout should be a valid JSON object of markdownindexer.Edge: %v\nstdout: %s", err, stdout.String())
	}

	if len(graph) == 0 {
		t.Fatal("expected at least one wikilink from testdata")
	}

	for hash, link := range graph {
		if hash == "" {
			t.Errorf("edge with empty IPLD hash: %+v", link)
		}
		if link.Label == "" {
			t.Errorf("edge %s has empty Label", hash)
		}
		if link.Title == "" {
			t.Errorf("edge %s has empty Title", hash)
		}
		if link.FromDocumentID == "" {
			t.Errorf("edge %s has empty FromDocumentID", hash)
		}
		if link.ToDocumentID == "" {
			t.Errorf("edge %s has empty ToDocumentID", hash)
		}
	}
}

func TestRunWithWikilinksOutput(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "a.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\nshare: true\n---\n\nLink to [[b]].\n")
	writeTestMD(t, dir, "b.md", "---\nuuid: 22222222-2222-2222-2222-222222222222\nshare: true\n---\n\nLink to [[a|Alpha]].\n")

	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", dir, "-wikilinks"}, &stdout, &stderr, strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var graph map[string]markdownindexer.Edge
	if err := json.Unmarshal([]byte(stdout.String()), &graph); err != nil {
		t.Fatalf("stdout should be valid JSON: %v\nstdout: %s", err, stdout.String())
	}

	if len(graph) != 2 {
		t.Fatalf("expected 2 wikilinks, got %d", len(graph))
	}

	// a -> b
	ab := false
	// b -> a (aliased)
	ba := false
	for _, l := range graph {
		if l.FromDocumentID == "11111111-1111-1111-1111-111111111111" &&
			l.ToDocumentID == "22222222-2222-2222-2222-222222222222" &&
			l.Label == "INTERNAL" && l.Title == "b" {
			ab = true
		}
		if l.FromDocumentID == "22222222-2222-2222-2222-222222222222" &&
			l.ToDocumentID == "11111111-1111-1111-1111-111111111111" &&
			l.Title == "Alpha" {
			ba = true
		}
	}
	if !ab {
		t.Errorf("missing a -> b edge in %s", stdout.String())
	}
	if !ba {
		t.Errorf("missing b -> a (aliased) edge in %s", stdout.String())
	}
}

func TestRunWithForceDocumentJSON(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "good.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\nshare: true\n---\n\nGood.\n")
	writeTestMD(t, dir, "broken.md", "---\ntitle: [unclosed\n---\n\nBroken.\n")

	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", dir, "-force"}, &stdout, &stderr, strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "skipping broken.md") {
		t.Errorf("stderr should log the skipped document: %q", stderr.String())
	}

	var docs map[string]markdownindexer.Document
	if err := json.Unmarshal([]byte(stdout.String()), &docs); err != nil {
		t.Fatalf("stdout should be a valid JSON object of Document: %v\nstdout: %s", err, stdout.String())
	}
	if len(docs) != 1 {
		t.Errorf("force should output only the healthy document, got %d documents", len(docs))
	}
	if got := docs["11111111-1111-1111-1111-111111111111"]; got.RelativePath != "good.md" {
		t.Errorf("force output should be keyed by the document UUID, got %+v", got)
	}
}

func TestRunWithQuietSuppressesNotices(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "good.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\nshare: true\n---\n\nGood.\n")
	writeTestMD(t, dir, "broken.md", "---\ntitle: [unclosed\n---\n\nBroken.\n")

	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", dir, "-force", "-quiet"}, &stdout, &stderr, strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("quiet should write no notices to stderr, got: %q", stderr.String())
	}

	// The point of the flag: stdout is a JSON document with nothing mixed in.
	var docs map[string]markdownindexer.Document
	if err := json.Unmarshal([]byte(stdout.String()), &docs); err != nil {
		t.Fatalf("quiet should still write valid JSON: %v\nstdout: %s", err, stdout.String())
	}
	if len(docs) != 1 {
		t.Errorf("quiet should not change what is indexed, got %d documents", len(docs))
	}
}

func TestRunWithQShorthandMatchesQuiet(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "broken.md", "---\ntitle: [unclosed\n---\n\nBroken.\n")

	var stdout, stderr strings.Builder
	if err := runWith([]string{"-dir", dir, "-force", "-q"}, &stdout, &stderr, strings.NewReader("")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("-q should behave like -quiet, got: %q", stderr.String())
	}
}

// Quiet must never hide why a run failed: a broken vault with no -force is a
// fatal error, returned and printed by main, not a notice.
func TestRunWithQuietKeepsFatalErrors(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "broken.md", "---\ntitle: [unclosed\n---\n\nBroken.\n")

	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", dir, "-quiet"}, &stdout, &stderr, strings.NewReader(""))
	if err == nil {
		t.Fatal("quiet should not turn a fatal error into a success")
	}
	if !strings.Contains(err.Error(), "broken.md") {
		t.Errorf("error should still name the broken file, got: %v", err)
	}
}

// -vaultcheck reports on stderr because that report is its output, so -quiet
// leaves it alone.
func TestRunWithQuietKeepsVaultCheckReport(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "a.md", "---\nuuid: 33333333-3333-3333-3333-333333333333\n---\n\nA.\n")
	writeTestMD(t, dir, "b.md", "---\nuuid: 33333333-3333-3333-3333-333333333333\n---\n\nB.\n")

	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", dir, "-validate", "-quiet"}, &stdout, &stderr, strings.NewReader(""))
	if err == nil {
		t.Fatal("want a non-nil error for a vault with duplicate UUIDs")
	}
	if !strings.Contains(stderr.String(), "duplicate UUID") {
		t.Errorf("vaultcheck report should survive quiet, got: %q", stderr.String())
	}
}

func TestRunWithQuietSuppressesMemUsage(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "good.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\nshare: true\n---\n\nGood.\n")

	var stdout, stderr strings.Builder
	if err := runWith([]string{"-dir", dir, "-memusage", "-quiet"}, &stdout, &stderr, strings.NewReader("")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("quiet should suppress the memory report too, got: %q", stderr.String())
	}
}

func TestRunWithDocumentJSONFailsWithoutForce(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "good.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nGood.\n")
	writeTestMD(t, dir, "broken.md", "---\ntitle: [unclosed\n---\n\nBroken.\n")

	var stdout, stderr strings.Builder
	if err := runWith([]string{"-dir", dir}, &stdout, &stderr, strings.NewReader("")); err == nil {
		t.Fatal("want error without force")
	}
}

func TestRunWithForceWikilinksJSON(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "good.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\nshare: true\n---\n\nLinks to [[good]] and [[broken]].\n")
	writeTestMD(t, dir, "broken.md", "---\ntitle: [unclosed\n---\n\nBroken.\n")

	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", dir, "-wikilinks", "-force"}, &stdout, &stderr, strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "skipping broken.md") {
		t.Errorf("stderr should log the skipped document: %q", stderr.String())
	}

	var graph map[string]markdownindexer.Edge
	if err := json.Unmarshal([]byte(stdout.String()), &graph); err != nil {
		t.Fatalf("stdout should be a valid JSON object of markdownindexer.Edge: %v\nstdout: %s", err, stdout.String())
	}
	for _, l := range graph {
		if l.FromDocumentID != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("edges must only come from the healthy document, got from_document_id %q", l.FromDocumentID)
		}
	}
}

func TestFilterMarshalableSkipsUnmarshalable(t *testing.T) {
	docs := []markdownindexer.Document{
		{RelativePath: "bad.md", Metadata: json.RawMessage("not json")},
		{RelativePath: "good.md", Metadata: json.RawMessage(`{"uuid":"11111111-1111-1111-1111-111111111111"}`)},
	}

	var stderr strings.Builder
	kept := filterMarshalable(docs, func(d markdownindexer.Document) string { return d.RelativePath }, &stderr)
	if len(kept) != 1 || kept[0].RelativePath != "good.md" {
		t.Fatalf("filterMarshalable should drop only the unmarshalable document, got %d kept", len(kept))
	}
	if !strings.Contains(stderr.String(), "skipping bad.md") {
		t.Errorf("stderr should log the skipped document: %q", stderr.String())
	}
}

func TestVaultCheckReportsIssues(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "one.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nOne.\n")
	writeTestMD(t, dir, "two.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nTwo.\n")
	writeTestMD(t, dir, "broken.md", "---\ntitle: [unclosed\n---\n\nBroken.\n")
	writeTestMD(t, dir, "missing-id.md", "---\ntitle: No UUID\n---\n\nBody.\n")
	writeTestMD(t, dir, "no-frontmatter.md", "Deliberately no front matter.\n")

	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", dir, "-validate"}, &stdout, &stderr, strings.NewReader(""))
	if err == nil {
		t.Fatal("want error when problems are found")
	}
	if !strings.Contains(err.Error(), "3 problem(s)") {
		t.Errorf("error should count problems, got: %v", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "broken.md") {
		t.Errorf("stderr should report the broken front matter: %q", out)
	}
	if !strings.Contains(out, "duplicate UUID") {
		t.Errorf("stderr should report the duplicate UUID: %q", out)
	}
	if !strings.Contains(out, "missing-id.md") {
		t.Errorf("stderr should report the missing uuid: %q", out)
	}
	if strings.Contains(out, "no-frontmatter.md") {
		t.Errorf("files without front matter must be tolerated, got: %q", out)
	}
	if stdout.String() != "" {
		t.Errorf("vault check should not write the index to stdout: %q", stdout.String())
	}
}

func TestVaultCheckClean(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "one.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nOne.\n")
	writeTestMD(t, dir, "no-frontmatter.md", "No front matter is fine here.\n")

	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", dir, "-validate"}, &stdout, &stderr, strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "vault check: ok") {
		t.Errorf("stderr should report a clean vault: %q", stderr.String())
	}
	if stdout.String() != "" {
		t.Errorf("vault check should not write to stdout: %q", stdout.String())
	}
}

func TestRunWithShareFilter(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "shared.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\nshare: true\n---\nShared.\n")
	writeTestMD(t, dir, "private.md", "---\nuuid: 22222222-2222-2222-2222-222222222222\nshare: false\n---\nPrivate.\n")
	writeTestMD(t, dir, "unsigned.md", "---\nuuid: 33333333-3333-3333-3333-333333333333\n---\nNo share key.\n")

	var stdout, stderr strings.Builder
	if err := runWith([]string{"-dir", dir}, &stdout, &stderr, strings.NewReader("")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var docs map[string]markdownindexer.Document
	if err := json.Unmarshal([]byte(stdout.String()), &docs); err != nil {
		t.Fatalf("stdout should be a valid index of Document: %v\n%s", err, stdout.String())
	}
	if len(docs) != 1 || docs["11111111-1111-1111-1111-111111111111"].RelativePath != "shared.md" {
		t.Errorf("the default run should load only the shared document, got %d entries: %s", len(docs), stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := runWith([]string{"-include-unshared", "-dir", dir}, &stdout, &stderr, strings.NewReader("")); err != nil {
		t.Fatalf("-all run: %v", err)
	}
	docs = map[string]markdownindexer.Document{}
	if err := json.Unmarshal([]byte(stdout.String()), &docs); err != nil {
		t.Fatalf("-all stdout should be a valid index of Document: %v\n%s", err, stdout.String())
	}
	if len(docs) != 3 {
		t.Errorf("-all should load every document, got %d entries: %s", len(docs), stdout.String())
	}
}

func TestRunWithShareWikilinks(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "shared.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\nshare: true\n---\nSee [[private]].\n")
	writeTestMD(t, dir, "private.md", "---\nuuid: 22222222-2222-2222-2222-222222222222\nshare: false\n---\nSee [[shared]].\n")

	var stdout, stderr strings.Builder
	if err := runWith([]string{"-dir", dir, "-wikilinks"}, &stdout, &stderr, strings.NewReader("")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var graph map[string]markdownindexer.Edge
	if err := json.Unmarshal([]byte(stdout.String()), &graph); err != nil {
		t.Fatalf("stdout should be a valid JSON object of markdownindexer.Edge: %v\n%s", err, stdout.String())
	}
	if len(graph) != 1 {
		t.Fatalf("the default wikilinks output should carry only edges from shared documents, got %d", len(graph))
	}
	for _, l := range graph {
		if l.FromDocumentID != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("the surviving edge must originate from the shared document, got %+v", l)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if err := runWith([]string{"-include-unshared", "-dir", dir, "-wikilinks"}, &stdout, &stderr, strings.NewReader("")); err != nil {
		t.Fatalf("-all run: %v", err)
	}
	graph = map[string]markdownindexer.Edge{}
	if err := json.Unmarshal([]byte(stdout.String()), &graph); err != nil {
		t.Fatalf("-all stdout should be a valid JSON object of markdownindexer.Edge: %v\n%s", err, stdout.String())
	}
	if len(graph) != 2 {
		t.Errorf("-all should include every edge, got %d", len(graph))
	}
}
