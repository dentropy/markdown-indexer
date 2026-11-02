package markdownindexer

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func backlinkDoc(t *testing.T, relPath, frontMatter, body string) Document {
	t.Helper()
	content := "---\n" + frontMatter + "---\n\n" + body
	doc, err := Parse([]byte(content), relPath, time.Time{}, "")
	if err != nil {
		t.Fatalf("parse %s: %v", relPath, err)
	}
	return doc
}

func TestBuildBacklinks(t *testing.T) {
	docs := []Document{
		backlinkDoc(t, "notes/alpha.md", "uuid: aaa\ntitle: Alpha\n", "See [[beta|the beta note]] and [[beta]]."),
		backlinkDoc(t, "notes/beta.md", "uuid: bbb\n", "Beta."),
	}

	got := BuildBacklinks(docs)

	beta, ok := got["bbb"]
	if !ok {
		t.Fatalf("beta has no entry: %+v", got)
	}
	if beta.DocumentID != "bbb" {
		t.Errorf("document_id = %q, want bbb", beta.DocumentID)
	}
	want := []Backlink{
		{
			FromDocumentID:   "aaa",
			FromName:         "Alpha",
			FromRelativePath: "notes/alpha.md",
			Title:            "beta",
			Label:            "INTERNAL",
		},
		{
			FromDocumentID:   "aaa",
			FromName:         "Alpha",
			FromRelativePath: "notes/alpha.md",
			Title:            "the beta note",
			Label:            "INTERNAL",
		},
	}
	if !reflect.DeepEqual(beta.Backlinks, want) {
		t.Errorf("beta backlinks = %+v, want %+v", beta.Backlinks, want)
	}

	alpha, ok := got["aaa"]
	if !ok {
		t.Fatalf("alpha has no entry: %+v", got)
	}
	if alpha.Backlinks == nil {
		t.Fatal("an unlinked document must publish an empty slice, not nil")
	}
	if len(alpha.Backlinks) != 0 {
		t.Errorf("alpha backlinks = %+v, want none", alpha.Backlinks)
	}
	data, err := json.Marshal(alpha)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"document_id":"aaa","backlinks":[]}` {
		t.Errorf("unlinked document JSON = %s, want an empty array", data)
	}
}

func TestBacklinksDropsWhatIsNotABodyLink(t *testing.T) {
	docs := []Document{
		backlinkDoc(t, "alpha.md", "uuid: aaa\nrelated: \"[[beta]]\"\nrelatedEmbed: \"![[beta]]\"\n",
			"Self [[alpha]]. Missing [[nope]]. Web [[https://example.com]]. Tag #topic. "+
				"Embed ![[beta]]. Markdown [beta](https://example.com). Real [[beta]]."),
		backlinkDoc(t, "beta.md", "uuid: bbb\n", "Beta."),
	}

	got := BuildBacklinks(docs)
	beta := got["bbb"]
	if len(beta.Backlinks) != 1 {
		t.Fatalf("only the resolved body wikilink is a backlink, got %+v", beta.Backlinks)
	}
	if beta.Backlinks[0].Title != "beta" {
		t.Errorf("title = %q, want beta", beta.Backlinks[0].Title)
	}
	if len(got["aaa"].Backlinks) != 0 {
		t.Errorf("a self-link must be dropped, got %+v", got["aaa"].Backlinks)
	}
}

func TestBacklinksSortedBySourceThenTitleThenID(t *testing.T) {
	// Deliberately out of order; source names put uppercase before lowercase.
	docs := []Document{
		backlinkDoc(t, "z.md", "uuid: zzz\ntitle: Zulu\n", "[[target|m]]"),
		backlinkDoc(t, "b.md", "uuid: bbb\ntitle: Alpha\n", "[[target|z]]"),
		backlinkDoc(t, "a.md", "uuid: aaa\n", "[[target|b]] [[target|a]]"),
		backlinkDoc(t, "target.md", "uuid: ttt\ntitle: Target\n", "Target."),
	}

	got := BuildBacklinks(docs)
	var order []string
	for _, b := range got["ttt"].Backlinks {
		order = append(order, b.FromName+"/"+b.Title)
	}
	want := []string{"Alpha/z", "Zulu/m", "a/a", "a/b"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestBacklinksSkipsDocumentsWithoutUUID(t *testing.T) {
	docs := []Document{
		backlinkDoc(t, "alpha.md", "uuid: aaa\n", "[[noid]] [[beta]]"),
		backlinkDoc(t, "beta.md", "uuid: bbb\n", "Beta."),
		backlinkDoc(t, "noid.md", "", "No UUID."),
	}

	got := BuildBacklinks(docs)
	if _, ok := got[""]; ok {
		t.Errorf("a document without a UUID must not get an entry: %+v", got)
	}
	if len(got) != 2 {
		t.Errorf("want entries for alpha and beta only, got %+v", got)
	}
	if len(got["bbb"].Backlinks) != 1 {
		t.Errorf("beta backlinks = %+v, want only alpha", got["bbb"].Backlinks)
	}
}

func TestBacklinksDuplicateUUIDIsDeterministic(t *testing.T) {
	// Scan orders documents by relative path; the last one wins the ID, and
	// both edges carry that UUID.
	docs := []Document{
		backlinkDoc(t, "a.md", "uuid: dup\n", "[[beta|a]]"),
		backlinkDoc(t, "z.md", "uuid: dup\n", "[[beta|z]]"),
		backlinkDoc(t, "beta.md", "uuid: bbb\n", "Beta."),
	}

	got := BuildBacklinks(docs)
	entries := got["bbb"].Backlinks
	if len(entries) != 2 {
		t.Fatalf("want two distinct titles, got %+v", entries)
	}
	for _, e := range entries {
		if e.FromRelativePath != "z.md" {
			t.Errorf("duplicate UUID should resolve to the greatest path, got %+v", e)
		}
	}
}

func TestIndexBacklinksMatchesBuildBacklinks(t *testing.T) {
	docs := []Document{
		backlinkDoc(t, "alpha.md", "uuid: aaa\n", "[[beta]]"),
		backlinkDoc(t, "beta.md", "uuid: bbb\n", "Beta."),
	}
	ix := &Index{Documents: docs, Edges: buildGraph(docs)}

	if got, want := ix.Backlinks(), BuildBacklinks(docs); !reflect.DeepEqual(got, want) {
		t.Errorf("Index.Backlinks() = %+v, want %+v", got, want)
	}
}

func TestIndexBacklinksWithoutEdges(t *testing.T) {
	docs := []Document{
		backlinkDoc(t, "alpha.md", "uuid: aaa\n", "[[beta]]"),
		backlinkDoc(t, "beta.md", "uuid: bbb\n", "Beta."),
	}
	got := (&Index{Documents: docs}).Backlinks()
	if len(got) != 2 {
		t.Fatalf("every document needs an entry, got %+v", got)
	}
	if got["bbb"].Backlinks == nil || len(got["bbb"].Backlinks) != 0 {
		t.Errorf("a nil edge map means no backlinks, got %+v", got["bbb"])
	}
}

func TestDisplayName(t *testing.T) {
	cases := []struct {
		name        string
		frontMatter string
		want        string
	}{
		{"title wins", "uuid: aaa\ntitle: A Proper Title\n", "A Proper Title"},
		{"no title falls back to the file name", "uuid: aaa\n", "note"},
		{"whitespace title falls back", "uuid: aaa\ntitle: \"   \"\n", "note"},
		{"non-string title falls back", "uuid: aaa\ntitle: 42\n", "note"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := backlinkDoc(t, "note.md", c.frontMatter, "Body.")
			if got := doc.DisplayName(); got != c.want {
				t.Errorf("DisplayName = %q, want %q", got, c.want)
			}
		})
	}
}

func TestBacklinksDoNotLeakIntoFrontMatter(t *testing.T) {
	// A front matter wikilink is FRONTMATTER_INTERNAL, not INTERNAL; the
	// published file only lists body links.
	doc := backlinkDoc(t, "alpha.md", "uuid: aaa\nrelated: \"[[beta]]\"\n", "Body.")
	other := backlinkDoc(t, "beta.md", "uuid: bbb\n", "Beta.")

	got := BuildBacklinks([]Document{doc, other})
	if n := len(got["bbb"].Backlinks); n != 0 {
		t.Errorf("front matter link must not be a backlink, got %d: %+v", n, got["bbb"].Backlinks)
	}
}

func TestBacklinkJSONFields(t *testing.T) {
	doc := backlinkDoc(t, "alpha.md", "uuid: aaa\ntitle: Alpha\n", "[[beta|Alias]]")
	other := backlinkDoc(t, "beta.md", "uuid: bbb\n", "Beta.")

	data, err := json.Marshal(BuildBacklinks([]Document{doc, other})["bbb"])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"document_id":"bbb"`,
		`"from_document_id":"aaa"`,
		`"from_name":"Alpha"`,
		`"from_relative_path":"alpha.md"`,
		`"title":"Alias"`,
		`"label":"INTERNAL"`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("JSON missing %s: %s", want, data)
		}
	}
}
