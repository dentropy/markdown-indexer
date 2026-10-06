package markdownindexer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeMD(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExtractFrontMatter(t *testing.T) {
	data := []byte("---\nuuid: 123e4567-e89b-12d3-a456-426614174000\ntags:\n  - go\n---\n\nbody")
	fm, err := extractFrontMatter(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm["uuid"] != "123e4567-e89b-12d3-a456-426614174000" {
		t.Errorf("uuid = %v, want document uuid", fm["uuid"])
	}
	b, err := json.Marshal(fm)
	if err != nil {
		t.Fatal(err)
	}
	if got := uuidFrom(b, "uuid"); got != "123e4567-e89b-12d3-a456-426614174000" {
		t.Errorf("uuidFrom = %q", got)
	}
	if got := uuidFrom(b, "id"); got != "" {
		t.Errorf("uuidFrom should not fall back to the id key, got %q", got)
	}
}

func TestExtractFrontMatterMissing(t *testing.T) {
	fm, err := extractFrontMatter([]byte("no front matter"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("want empty map, got %v", fm)
	}
}

func TestExtractFrontMatterUnterminated(t *testing.T) {
	_, err := extractFrontMatter([]byte("---\nid: nope"))
	if err == nil {
		t.Fatal("want error for unterminated front matter")
	}
	if err != ErrUnterminatedFrontMatter {
		t.Errorf("want ErrUnterminatedFrontMatter, got %v", err)
	}
}

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"**/*.md", "a/b/c.md", true},
		{"**/*.md", "top.md", true},
		{"**/*.md", "a/b/c.txt", false},
		{"*.md", "top.md", true},
		{"*.md", "a/top.md", false},
		{"a/**", "a/x/y.md", true},
		{"a/**", "b/x.md", false},
	}
	for _, c := range cases {
		if got := MatchGlob(c.pattern, c.name); got != c.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

func TestParse(t *testing.T) {
	dir := t.TempDir()
	p := writeMD(t, dir, "a/b/post.md", "---\nuuid: 123e4567-e89b-12d3-a456-426614174000\ntitle: First Post\ntags:\n  - go\n  - cli\ndraft: true\nshare: true\n---\n\n# First Post\n\nHello from the first markdown file.")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	doc, err := Parse(data, "a/b/post.md", info.ModTime(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.RelativePath != "a/b/post.md" {
		t.Errorf("path = %q", doc.RelativePath)
	}
	if doc.Name != "post" {
		t.Errorf("name = %q, want %q", doc.Name, "post")
	}
	if doc.ModifiedUnix != info.ModTime().Unix() {
		t.Errorf("modifiedUnix = %d, want %d", doc.ModifiedUnix, info.ModTime().Unix())
	}
	want := "123e4567-e89b-12d3-a456-426614174000"
	if !strings.Contains(string(doc.Metadata), want) {
		t.Errorf("metadata missing uuid: %s", doc.Metadata)
	}
	if doc.ID != want {
		t.Errorf("id = %q, want %q", doc.ID, want)
	}
	if doc.FrontmatterCID != "baguqeerat7vkerbf6ngyuswkheivqhtnym45uvk6ekwqn5bzdnizdwbq2adq" {
		t.Errorf("frontmatterCID = %q", doc.FrontmatterCID)
	}
	if doc.RawMarkdown != "# First Post\n\nHello from the first markdown file." {
		t.Errorf("rawMarkdown = %q", doc.RawMarkdown)
	}
	if doc.RawMarkdownHash != "afb00ef5983c7babc20d3eb45871aa371ab15814e96a9fdc2bf978d9944ba527" {
		t.Errorf("rawMarkdownHash = %q", doc.RawMarkdownHash)
	}
}

func TestParseCustomIDKey(t *testing.T) {
	doc, err := Parse([]byte("---\nid: abc123\n---\n\nbody"), "x.md", time.Unix(0, 0), "id")
	if err != nil {
		t.Fatal(err)
	}
	if doc.ID != "abc123" {
		t.Errorf("id = %q, want abc123", doc.ID)
	}
	other, err := Parse([]byte("---\nid: abc123\n---\n\nbody"), "x.md", time.Unix(0, 0), "")
	if err != nil {
		t.Fatal(err)
	}
	if other.ID != "" {
		t.Errorf("the default ID key must not pick up `id`, got %q", other.ID)
	}
}

func TestParseBrokenFrontMatter(t *testing.T) {
	if _, err := Parse([]byte("---\ntitle: [unclosed\n---\n\nBroken."), "broken.md", time.Unix(0, 0), ""); err == nil {
		t.Fatal("want error for broken front matter")
	}
}

func TestFrontMatterCID(t *testing.T) {
	cases := []struct {
		name, content, want string
	}{
		{
			name:    "post",
			content: "---\nuuid: 123e4567-e89b-12d3-a456-426614174000\ntitle: First Post\ntags:\n  - go\n  - cli\ndraft: true\nshare: true\n---\n\nbody",
			want:    "baguqeerat7vkerbf6ngyuswkheivqhtnym45uvk6ekwqn5bzdnizdwbq2adq",
		},
		{
			name:    "empty",
			content: "no front matter",
			want:    "baguqeeraiqjw7i2vwntyuekgvulpp2det2kpwt6cd7tx5ayqybqpmhfk76fa",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm, err := extractFrontMatter([]byte(c.content))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, err := FrontMatterCID(fm)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("FrontMatterCID = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFrontMatterCIDCanonical(t *testing.T) {
	a := "title: First Post\nuuid: 123e4567-e89b-12d3-a456-426614174000\ndraft: true\ntags:\n  - go\n  - cli\nshare: true\n"
	b := "draft: true\ntags:\n  - go\n  - cli\ntitle: First Post\nshare: true\nuuid: 123e4567-e89b-12d3-a456-426614174000\n"
	fa, err := extractFrontMatter([]byte("---\n" + a + "---\n\nbody"))
	if err != nil {
		t.Fatal(err)
	}
	fb, err := extractFrontMatter([]byte("---\n" + b + "---\n\nbody"))
	if err != nil {
		t.Fatal(err)
	}
	ca, err := FrontMatterCID(fa)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := FrontMatterCID(fb)
	if err != nil {
		t.Fatal(err)
	}
	if ca != cb {
		t.Errorf("key order must not affect the CID: %q != %q", ca, cb)
	}
	if ca != "baguqeerat7vkerbf6ngyuswkheivqhtnym45uvk6ekwqn5bzdnizdwbq2adq" {
		t.Errorf("canonical CID = %q", ca)
	}
}

func TestStripFrontMatter(t *testing.T) {
	in := []byte("---\nuuid: 123e4567-e89b-12d3-a456-426614174000\ntitle: Keep Body\n---\n\nBody text here.\n")
	if got := string(stripFrontMatter(in)); got != "Body text here.\n" {
		t.Errorf("stripFrontMatter = %q", got)
	}
	noFM := []byte("just body\n")
	if string(stripFrontMatter(noFM)) != string(noFM) {
		t.Errorf("files without front matter must be unchanged")
	}
}

func TestDuplicateUUIDs(t *testing.T) {
	ix := &Index{Documents: []Document{
		{RelativePath: "a.md", ID: "11111111-1111-1111-1111-111111111111"},
		{RelativePath: "b.md", ID: "11111111-1111-1111-1111-111111111111"},
		{RelativePath: "c.md", ID: "22222222-2222-2222-2222-222222222222"},
		{RelativePath: "d.md", ID: "22222222-2222-2222-2222-222222222222"},
		{RelativePath: "e.md", ID: "33333333-3333-3333-3333-333333333333"},
		{RelativePath: "f.md", ID: ""},
	}}
	want := []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"}
	if got := ix.DuplicateUUIDs(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("DuplicateUUIDs = %v, want %v", got, want)
	}
	if got := ix.PathsForID("11111111-1111-1111-1111-111111111111"); strings.Join(got, ",") != "a.md,b.md" {
		t.Errorf("PathsForID = %v", got)
	}
	if got := (&Index{Documents: []Document{{RelativePath: "a.md"}}}).DuplicateUUIDs(); len(got) != 0 {
		t.Errorf("empty ids must not be reported, got %v", got)
	}
}

func TestByUUID(t *testing.T) {
	ix := &Index{Documents: []Document{
		{RelativePath: "a.md", ID: "1"},
		{RelativePath: "b.md", ID: ""},
	}}
	got := ix.ByUUID()
	if len(got) != 2 {
		t.Fatalf("a document with no UUID must still be reachable, got %d entries", len(got))
	}
	if got["1"].RelativePath != "a.md" || got[""].RelativePath != "b.md" {
		t.Errorf("ByUUID = %v", got)
	}
}

func TestExtractWikilinks(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{"single basic link", "See [[note]] for more.", []string{"note"}},
		{"single aliased link", "See [[note|the note]] for more.", []string{"note"}},
		{"single section link", "Jump to [[note#installation]].", []string{"note"}},
		{"section and alias", "See [[note#setup|the setup]] here.", []string{"note"}},
		{"multiple links", "Links: [[post]] and [[note]] and [[dup]].", []string{"post", "note", "dup"}},
		{"external url as wikilink", "Visit [[https://example.com]] for info.", []string{"https://example.com"}},
		{"url with alias", "Visit [[https://example.com|Example]] for info.", []string{"https://example.com"}},
		{"path style link", "Link to [[a/b/post]].", []string{"a/b/post"}},
		{"mixed content", "Text [[post]] and [[https://example.com|Ex]] and [[note#section|Label]].", []string{"post", "https://example.com", "note"}},
		{"no wikilinks", "Just plain markdown with [links](http://example.com).", []string{}},
		{"empty content", "", []string{}},
		{"adjacent links", "[[post]][[note]]", []string{"post", "note"}},
		{"whitespace in target", "See [[my note]] here.", []string{"my note"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExtractWikilinks(c.content)
			if len(got) != len(c.want) {
				t.Fatalf("ExtractWikilinks() returned %d links, want %d: %v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("link[%d] = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestBuildGraph(t *testing.T) {
	docs := []Document{
		{ID: "11111111-1111-1111-1111-111111111111", Name: "post", RawMarkdown: "# Post\n\nSee [[note]] and [[https://example.com|Example]]."},
		{ID: "22222222-2222-2222-2222-222222222222", Name: "note", RawMarkdown: "# Note\n\nBack to [[post]]."},
		{ID: "", Name: "no-id", RawMarkdown: "# No ID\n\nLinks to [[post]]."},
	}

	graph := buildGraph(docs)
	if len(graph) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(graph))
	}

	find := func(from, to, label, title string) {
		t.Helper()
		for cid, e := range graph {
			if cid == "" {
				t.Errorf("every edge must be keyed by a non-empty CID")
				return
			}
			if e.FromDocumentID == from && e.ToDocumentID == to && e.Label == label && e.Title == title {
				return
			}
		}
		t.Errorf("missing edge %s -> %s (%s / %s)", from, to, label, title)
	}

	find("11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "INTERNAL", "note")
	find("11111111-1111-1111-1111-111111111111", "https://example.com", "WEBSITE", "Example")
	find("22222222-2222-2222-2222-222222222222", "11111111-1111-1111-1111-111111111111", "INTERNAL", "post")
}

func TestBuildGraphUnresolved(t *testing.T) {
	graph := buildGraph([]Document{
		{ID: "11111111-1111-1111-1111-111111111111", Name: "post", RawMarkdown: "Link to [[nonexistent]]."},
	})
	if len(graph) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph))
	}
	for _, e := range graph {
		if e.ToDocumentID != "nonexistent" {
			t.Errorf("unresolved link ToDocumentID = %q, want nonexistent", e.ToDocumentID)
		}
		if e.Label != "INTERNAL" {
			t.Errorf("unresolved link Label = %q, want INTERNAL", e.Label)
		}
	}
}

func TestBuildGraphAliasedTitle(t *testing.T) {
	graph := buildGraph([]Document{
		{ID: "11111111-1111-1111-1111-111111111111", Name: "post", RawMarkdown: "See [[note|the note]] and [[note#setup|Setup Guide]]."},
		{ID: "22222222-2222-2222-2222-222222222222", Name: "note", RawMarkdown: "# Note"},
	})
	if len(graph) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(graph))
	}
	titles := make(map[string]bool, len(graph))
	for _, e := range graph {
		titles[e.Title] = true
	}
	if !titles["the note"] {
		t.Errorf("expected title 'the note', got %v", titles)
	}
	if !titles["Setup Guide"] {
		t.Errorf("expected title 'Setup Guide', got %v", titles)
	}
}

func TestBuildGraphEmpty(t *testing.T) {
	if graph := buildGraph([]Document{{ID: "1", Name: "post", RawMarkdown: "No links here."}}); len(graph) != 0 {
		t.Errorf("expected 0 edges, got %d", len(graph))
	}
}

func TestBuildGraphSkipsDocumentsWithoutID(t *testing.T) {
	if graph := buildGraph([]Document{{ID: "", Name: "no-id", RawMarkdown: "Links to [[post]]."}}); len(graph) != 0 {
		t.Errorf("documents without an ID must be skipped, got %d edges", len(graph))
	}
}

func TestBuildGraphDistinctAddresses(t *testing.T) {
	graph := buildGraph([]Document{
		{ID: "11111111-1111-1111-1111-111111111111", Name: "post", RawMarkdown: "[[note]] and [[dup]]."},
		{ID: "22222222-2222-2222-2222-222222222222", Name: "note", RawMarkdown: "# Note"},
		{ID: "33333333-3333-3333-3333-333333333333", Name: "dup", RawMarkdown: "# Dup"},
	})
	if len(graph) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(graph))
	}
	toIDs := make(map[string]bool, len(graph))
	for _, e := range graph {
		if e.ToDocumentID == "" {
			t.Errorf("edge with empty ToDocumentID: %+v", e)
		}
		toIDs[e.ToDocumentID] = true
	}
	if !toIDs["22222222-2222-2222-2222-222222222222"] || !toIDs["33333333-3333-3333-3333-333333333333"] {
		t.Errorf("distinct edges must hash to distinct keys, got targets %v", toIDs)
	}
}

func TestSharedMetadata(t *testing.T) {
	cases := []struct {
		metadata string
		want     bool
	}{
		{`{"share":true}`, true},
		{`{"share":"true"}`, true},
		{`{"share":"TRUE"}`, true},
		{`{"share":false}`, false},
		{`{"share":"no"}`, false},
		{`{"title":"A"}`, false},
		{`{}`, false},
		{`not json`, false},
	}
	for _, c := range cases {
		if got := SharedMetadata(json.RawMessage(c.metadata)); got != c.want {
			t.Errorf("SharedMetadata(%s) = %v, want %v", c.metadata, got, c.want)
		}
	}
}

func TestFilterSharedDocsIdempotent(t *testing.T) {
	docs := []Document{
		{RelativePath: "a.md", Metadata: json.RawMessage(`{"share":true}`)},
		{RelativePath: "b.md", Metadata: json.RawMessage(`{"share":false}`)},
		{RelativePath: "c.md", Metadata: json.RawMessage(`{"title":"A"}`)},
	}
	once := filterSharedDocs(docs)
	if len(once) != 1 || once[0].RelativePath != "a.md" {
		t.Errorf("filterSharedDocs should keep only the shared document, got %+v", once)
	}
	if twice := filterSharedDocs(once); len(twice) != 1 {
		t.Errorf("filtering an already filtered set must not shrink it, got %d", len(twice))
	}
}
