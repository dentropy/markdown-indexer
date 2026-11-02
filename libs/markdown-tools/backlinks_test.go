package markdowntools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
)

func TestWriteBacklinks(t *testing.T) {
	out := filepath.Join(t.TempDir(), "backlinks")
	set := map[string]markdownindexer.Backlinks{
		"aaa": {
			DocumentID: "ignored: the key is authoritative",
			Backlinks: []markdownindexer.Backlink{{
				FromDocumentID:   "bbb",
				FromName:         "Beta",
				FromRelativePath: "notes/beta.md",
				Title:            "the alpha note",
				Label:            "INTERNAL",
			}},
		},
		"bbb": {},
	}

	written, removed, err := WriteBacklinks(set, out, BacklinksOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"aaa.json", "bbb.json"}; !equalStrings(written, want) {
		t.Errorf("written = %v, want %v", written, want)
	}
	if len(removed) != 0 {
		t.Errorf("removed = %v, want none", removed)
	}

	data, err := os.ReadFile(filepath.Join(out, "aaa.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got markdownindexer.Backlinks
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode aaa.json: %v\n%s", err, data)
	}
	if got.DocumentID != "aaa" {
		t.Errorf("document_id = %q, want the map key aaa", got.DocumentID)
	}
	if len(got.Backlinks) != 1 || got.Backlinks[0].Title != "the alpha note" {
		t.Errorf("aaa.json backlinks = %+v", got.Backlinks)
	}

	empty, err := os.ReadFile(filepath.Join(out, "bbb.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Backlinks json.RawMessage `json:"backlinks"`
	}
	if err := json.Unmarshal(empty, &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw.Backlinks) != "[]" {
		t.Errorf("an unlinked document must publish an empty array, got %s", raw.Backlinks)
	}
}

func TestWriteBacklinksIsIdempotent(t *testing.T) {
	out := filepath.Join(t.TempDir(), "backlinks")
	set := map[string]markdownindexer.Backlinks{"aaa": {}}

	if _, _, err := WriteBacklinks(set, out, BacklinksOptions{}); err != nil {
		t.Fatal(err)
	}
	written, removed, err := WriteBacklinks(set, out, BacklinksOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 || len(removed) != 0 {
		t.Errorf("a repeat run should change nothing, wrote %v removed %v", written, removed)
	}
}

func TestWriteBacklinksPrunesOnlyStaleJSON(t *testing.T) {
	out := t.TempDir()
	writeMD(t, out, "stale.json", "{}\n")
	writeMD(t, out, "notes.txt", "not ours\n")
	writeMD(t, out, filepath.Join("sub", "nested.json"), "{}\n")

	written, removed, err := WriteBacklinks(
		map[string]markdownindexer.Backlinks{"fresh": {}},
		out,
		BacklinksOptions{Prune: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"fresh.json"}; !equalStrings(written, want) {
		t.Errorf("written = %v, want %v", written, want)
	}
	if want := []string{"stale.json"}; !equalStrings(removed, want) {
		t.Errorf("removed = %v, want %v", removed, want)
	}
	if _, err := os.Stat(filepath.Join(out, "stale.json")); !os.IsNotExist(err) {
		t.Errorf("stale.json should be gone, stat err = %v", err)
	}
	for _, keep := range []string{"notes.txt", filepath.Join("sub", "nested.json")} {
		if _, err := os.Stat(filepath.Join(out, keep)); err != nil {
			t.Errorf("%s should be left alone: %v", keep, err)
		}
	}
}

func TestWriteBacklinksWithoutPruneKeepsStale(t *testing.T) {
	out := t.TempDir()
	writeMD(t, out, "stale.json", "{}\n")

	_, removed, err := WriteBacklinks(
		map[string]markdownindexer.Backlinks{"fresh": {}},
		out,
		BacklinksOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Errorf("removed = %v, want none", removed)
	}
	if _, err := os.Stat(filepath.Join(out, "stale.json")); err != nil {
		t.Errorf("stale.json should survive without Prune: %v", err)
	}
}

func TestWriteBacklinksEmptySetWithPruneClears(t *testing.T) {
	out := t.TempDir()
	writeMD(t, out, "stale.json", "{}\n")

	written, removed, err := WriteBacklinks(map[string]markdownindexer.Backlinks{}, out, BacklinksOptions{Prune: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 {
		t.Errorf("written = %v, want none", written)
	}
	if want := []string{"stale.json"}; !equalStrings(removed, want) {
		t.Errorf("removed = %v, want %v", removed, want)
	}
}

func TestWriteBacklinksRejectsUnsafeIDs(t *testing.T) {
	ids := []string{"", ".", "..", "../evil", "a/b", `a\b`, "C:evil", "bad\nname"}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "backlinks")
			_, _, err := WriteBacklinks(map[string]markdownindexer.Backlinks{id: {}}, out, BacklinksOptions{Prune: true})
			if err == nil {
				t.Fatalf("want an error for id %q", id)
			}
			if !strings.Contains(err.Error(), "invalid backlink document id") {
				t.Errorf("error should name the problem, got: %v", err)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Errorf("nothing should be written when an id is unsafe, stat err = %v", err)
			}
		})
	}
}

func TestWriteBacklinksRejectsEmptyDirectory(t *testing.T) {
	if _, _, err := WriteBacklinks(nil, "  ", BacklinksOptions{}); err == nil {
		t.Fatal("want an error for an empty directory")
	}
}

func TestWriteBacklinksDirectoryIsAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WriteBacklinks(map[string]markdownindexer.Backlinks{"aaa": {}}, path, BacklinksOptions{}); err == nil {
		t.Fatal("want an error when the directory path is a file")
	}
}
