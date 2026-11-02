package markdownindexer

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// manifestDoc builds one indexed document from a front matter block and a
// body.
func manifestDoc(t *testing.T, relPath, frontMatter, body string) Document {
	t.Helper()
	content := "---\n" + frontMatter + "---\n\n" + body
	doc, err := Parse([]byte(content), relPath, time.Time{}, "")
	if err != nil {
		t.Fatalf("parse %s: %v", relPath, err)
	}
	return doc
}

func TestBuildManifest(t *testing.T) {
	docs := []Document{
		manifestDoc(t, "b/zulu.md", "uuid: zzz\ntitle: Zulu\n", "Z."),
		manifestDoc(t, "a/alpha.md", "uuid: aaa\ntitle: Alpha\n", "A."),
		manifestDoc(t, "a/beta.md", "uuid: bbb\n", "B."),
	}

	got := BuildManifest(docs)

	want := []ManifestEntry{
		{Title: "Alpha", UUID: "aaa", Path: "a/alpha.md"},
		{Title: "beta", UUID: "bbb", Path: "a/beta.md"},
		{Title: "Zulu", UUID: "zzz", Path: "b/zulu.md"},
	}
	if !reflect.DeepEqual(got.Documents, want) {
		t.Errorf("documents = %+v, want %+v", got.Documents, want)
	}
}

func TestBuildManifestTitleFallback(t *testing.T) {
	docs := []Document{
		manifestDoc(t, "notes/untitled.md", "uuid: aaa\n", "No title."),
		manifestDoc(t, "notes/blank-title.md", "uuid: bbb\ntitle: '   '\n", "Blank title."),
		manifestDoc(t, "notes/number-title.md", "uuid: ccc\ntitle: 42\n", "Numeric title."),
		manifestDoc(t, "notes/no-front-matter.md", "", "Nothing."),
	}

	got := BuildManifest(docs)

	byID := make(map[string]ManifestEntry, len(got.Documents))
	for _, e := range got.Documents {
		byID[e.UUID] = e
	}
	for id, want := range map[string]string{
		"aaa": "untitled",
		"bbb": "blank-title",
		"ccc": "number-title",
	} {
		if byID[id].Title != want {
			t.Errorf("%s title = %q, want %q", id, byID[id].Title, want)
		}
	}
	if len(got.Documents) != 3 {
		t.Errorf("a document without a UUID must be omitted, got %+v", got.Documents)
	}
}

func TestBuildManifestSkipsEmptyUUID(t *testing.T) {
	docs := []Document{
		manifestDoc(t, "kept.md", "uuid: aaa\n", "Kept."),
		manifestDoc(t, "dropped.md", "title: No UUID\n", "Dropped."),
		{RelativePath: "synthetic.md", Name: "synthetic"},
	}

	got := BuildManifest(docs)
	if len(got.Documents) != 1 || got.Documents[0].UUID != "aaa" {
		t.Errorf("documents = %+v, want only the identified one", got.Documents)
	}
}

func TestManifestEmptyIsArray(t *testing.T) {
	data, err := json.Marshal(BuildManifest(nil))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "null") {
		t.Errorf("an empty manifest must encode documents as [], got %s", data)
	}
	var got struct {
		Documents []ManifestEntry `json:"documents"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Documents == nil || len(got.Documents) != 0 {
		t.Errorf("documents should decode as an empty slice, got %+v", data)
	}
}

func TestIndexManifest(t *testing.T) {
	ix := &Index{Documents: []Document{
		manifestDoc(t, "one.md", "uuid: aaa\ntitle: One\n", "One."),
		manifestDoc(t, "two.md", "uuid: bbb\n", "Two."),
	}}

	got := ix.Manifest()
	if len(got.Documents) != 2 {
		t.Fatalf("documents = %+v, want two", got.Documents)
	}
	if got.Documents[1].Title != "two" || got.Documents[1].Path != "two.md" {
		t.Errorf("second entry = %+v", got.Documents[1])
	}
}

func TestManifestKeepsDuplicateUUIDs(t *testing.T) {
	docs := []Document{
		manifestDoc(t, "b.md", "uuid: same\ntitle: B\n", "B."),
		manifestDoc(t, "a.md", "uuid: same\ntitle: A\n", "A."),
	}

	got := BuildManifest(docs)
	if len(got.Documents) != 2 {
		t.Fatalf("a manifest describes files, so both duplicates belong: %+v", got.Documents)
	}
	// Same UUID, ordered by path.
	if got.Documents[0].Path != "a.md" || got.Documents[1].Path != "b.md" {
		t.Errorf("duplicates should stay in path order, got %+v", got.Documents)
	}
}
