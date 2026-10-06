package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dentropy/markdown-indexer/libs/markdown-index"
)

func TestRunWithCheckDups(t *testing.T) {
	var stdout, stderr strings.Builder
	err := runWith([]string{"-dir", "testdata", "-pattern", "*.md", "-checkdups"}, &stdout, &stderr, strings.NewReader(""))
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
	err := runWith([]string{"-dir", "examples", "-checkdups"}, &stdout, &stderr, strings.NewReader(""))

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

func TestSetFrontMatterUUID(t *testing.T) {
	newID := "99999999-8888-7777-6666-555555555555"
	cases := []struct {
		name    string
		in      string
		wantIn  string
		wantOut string
	}{
		{
			name:    "replaces value, keeps rest of front matter",
			in:      "---\nuuid: old-id\ntitle: Keep\n---\n\nBody.\n",
			wantIn:  "old-id",
			wantOut: "---\nuuid: " + newID + "\ntitle: Keep\n---\n\nBody.\n",
		},
		{
			name:    "preserves quoted key",
			in:      "---\n\"uuid\": old-id\nother: 1\n---\n\nBody.\n",
			wantIn:  "old-id",
			wantOut: "---\n\"uuid\": " + newID + "\nother: 1\n---\n\nBody.\n",
		},
		{
			name:    "ignores nested key, appends top-level one",
			in:      "---\nuuid-old: no\nuuid-meta:\n  uuid: old-id\nother: 1\n---\n\nBody.\n",
			wantIn:  "uuid-meta",
			wantOut: "---\nuuid-old: no\nuuid-meta:\n  uuid: old-id\nother: 1\nuuid: " + newID + "\n---\n\nBody.\n",
		},
		{
			name:    "no front matter is unchanged",
			in:      "Just a body.\n",
			wantIn:  "Just a body.",
			wantOut: "Just a body.\n",
		},
		{
			name:    "missing key appends to front matter",
			in:      "---\ntitle: X\n---\n\nBody.\n",
			wantIn:  "title",
			wantOut: "---\ntitle: X\nuuid: " + newID + "\n---\n\nBody.\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(c.in, c.wantIn) {
				t.Fatalf("bad test setup: %q does not contain %q", c.in, c.wantIn)
			}
			if got := string(setFrontMatterUUID([]byte(c.in), "uuid", newID)); got != c.wantOut {
				t.Errorf("setFrontMatterUUID =\n%s\nwant\n%s", got, c.wantOut)
			}
		})
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
	err := runWith([]string{"-dir", "testdata", "-pattern", "*.md", "-memusage"}, &stdout, &stderr, strings.NewReader(""))
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
	err := runWith([]string{"-dir", dir, "-stripdups", "-checkdups"}, &stdout, &stderr, strings.NewReader("y\n"))
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
	err := runWith([]string{"-dir", dir, "-stripdups"}, &stdout, &stderr, strings.NewReader("n\n"))
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
	err := runWith([]string{"-dir", "testdata", "-pattern", "*.md", "-wikilinks"}, &stdout, &stderr, strings.NewReader(""))
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
	err := runWith([]string{"-dir", dir, "-vaultcheck", "-quiet"}, &stdout, &stderr, strings.NewReader(""))
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
	err := runWith([]string{"-dir", dir, "-vaultcheck"}, &stdout, &stderr, strings.NewReader(""))
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
	err := runWith([]string{"-dir", dir, "-vaultcheck"}, &stdout, &stderr, strings.NewReader(""))
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
	if err := runWith([]string{"-all", "-dir", dir}, &stdout, &stderr, strings.NewReader("")); err != nil {
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
	if err := runWith([]string{"-all", "-dir", dir, "-wikilinks"}, &stdout, &stderr, strings.NewReader("")); err != nil {
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
