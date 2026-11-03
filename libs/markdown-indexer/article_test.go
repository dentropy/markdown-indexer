package markdownindexer

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildArticles(t *testing.T) {
	docs := []Document{
		{
			RelativePath:    "a/alpha.md",
			Name:            "alpha",
			ModifiedUnix:    1,
			Metadata:        json.RawMessage(`{"title":"Alpha"}`),
			FrontmatterCID:  "cid-a",
			RawMarkdown:     "Alpha body.\n",
			RawMarkdownHash: "hash-a",
			ID:              "aaa",
		},
		{
			RelativePath:    "b/beta.md",
			Name:            "beta",
			ModifiedUnix:    2,
			Metadata:        json.RawMessage(`{"title":"Beta"}`),
			FrontmatterCID:  "cid-b",
			RawMarkdown:     "Beta body.\n",
			RawMarkdownHash: "hash-b",
			ID:              "bbb",
		},
		{
			RelativePath: "no-id.md",
			Name:         "no-id",
			ID:           "",
		},
		{
			RelativePath:    "z/dup.md",
			Name:            "dup",
			FrontmatterCID:  "cid-dup",
			RawMarkdown:     "later path wins.\n",
			RawMarkdownHash: "hash-dup",
			ID:              "aaa",
		},
	}

	got := BuildArticles(docs)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (empty UUID omitted, duplicate collapsed)", len(got))
	}
	if got["aaa"].RelativePath != "z/dup.md" {
		t.Errorf("duplicate UUID kept %q, want the greatest path", got["aaa"].RelativePath)
	}
	if got["aaa"].UUID != "aaa" {
		t.Errorf("uuid = %q, want the map key", got["aaa"].UUID)
	}
	if _, ok := got["aaa"]; ok {
		if strings.Contains(mustJSON(t, got["aaa"]), "metadata") {
			t.Errorf("article JSON must not inline metadata: %s", mustJSON(t, got["aaa"]))
		}
	}
	if got["bbb"].RawMarkdown != "Beta body.\n" || got["bbb"].FrontmatterCID != "cid-b" {
		t.Errorf("beta = %+v", got["bbb"])
	}
}

func TestBuildArticlesEmpty(t *testing.T) {
	got := BuildArticles(nil)
	if got == nil {
		t.Fatal("want an empty map, not nil")
	}
	if len(got) != 0 {
		t.Errorf("got %d articles, want 0", len(got))
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
