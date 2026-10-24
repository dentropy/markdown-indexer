package markdowntools

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
)

func writeMD(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scan(t *testing.T, dir, idKey string) *markdownindexer.Index {
	t.Helper()
	ix, err := markdownindexer.Scan(dir, markdownindexer.Options{IDKey: idKey})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Problems) != 0 {
		t.Fatalf("unexpected scan problems: %v", ix.Problems)
	}
	return ix
}

func readDoc(t *testing.T, dir, name, idKey string) (markdownindexer.Document, string) {
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
	writeMD(t, dir, "no-frontmatter.md", "Body only.\n")
	writeMD(t, dir, "missing-keys.md", "---\ntitle: Missing\n---\n\nBody.\n")
	writeMD(t, dir, "already-shared.md", "---\nuuid: keep-me\nshare: true\n---\n\nBody.\n")
	writeMD(t, dir, "nested/note.md", "---\nuuid: nested-id\nshare: false\n---\n\nNested.\n")

	changed, err := InitFrontMatter(scan(t, dir, "uuid"), dir, "uuid", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"missing-keys.md", "no-frontmatter.md"}; !equalStrings(changed, want) {
		t.Errorf("changed = %v, want %v", changed, want)
	}

	doc, raw := readDoc(t, dir, "no-frontmatter.md", "uuid")
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

	doc, raw = readDoc(t, dir, "missing-keys.md", "uuid")
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

	doc, raw = readDoc(t, dir, "already-shared.md", "uuid")
	if doc.ID != "keep-me" {
		t.Errorf("already-shared.md should keep its UUID, got %q: %s", doc.ID, raw)
	}
	if !markdownindexer.SharedMetadata(doc.Metadata) {
		t.Errorf("already-shared.md should keep share: true: %s", raw)
	}

	doc, raw = readDoc(t, dir, "nested/note.md", "uuid")
	if doc.ID != "nested-id" {
		t.Errorf("nested/note.md should keep its UUID, got %q: %s", doc.ID, raw)
	}
	if markdownindexer.SharedMetadata(doc.Metadata) {
		t.Errorf("nested/note.md should keep share: false: %s", raw)
	}
}

func TestInitFrontMatterShareTrue(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "missing-share.md", "---\nuuid: one\n---\n\nBody.\n")
	writeMD(t, dir, "existing-false.md", "---\nuuid: two\nshare: false\n---\n\nBody.\n")

	share := true
	changed, err := InitFrontMatter(scan(t, dir, "uuid"), dir, "uuid", &share)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"missing-share.md"}; !equalStrings(changed, want) {
		t.Errorf("changed = %v, want %v", changed, want)
	}

	doc, raw := readDoc(t, dir, "missing-share.md", "uuid")
	if !markdownindexer.SharedMetadata(doc.Metadata) {
		t.Errorf("missing-share.md should get share: true: %s", raw)
	}
	doc, raw = readDoc(t, dir, "existing-false.md", "uuid")
	if markdownindexer.SharedMetadata(doc.Metadata) {
		t.Errorf("existing-false.md should keep share: false: %s", raw)
	}
}

func TestInitFrontMatterCustomIDKey(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "note.md", "---\ntitle: Custom\n---\n\nBody.\n")

	if _, err := InitFrontMatter(scan(t, dir, "id"), dir, "id", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc, raw := readDoc(t, dir, "note.md", "id")
	if doc.ID == "" {
		t.Errorf("note.md should get an id key: %s", raw)
	}
	if strings.Contains(raw, "uuid:") {
		t.Errorf("note.md should use the configured id key, not uuid: %s", raw)
	}
}

func TestSetShareRecursive(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "no-frontmatter.md", "Body only.\n")
	writeMD(t, dir, "shared.md", "---\nuuid: keep-a\nshare: true\n---\n\nShared body.\n")
	writeMD(t, dir, "nested/private.md", "---\nuuid: keep-b\nshare: false\n---\n\nPrivate body.\n")
	writeMD(t, dir, "nested/deep/upper.md", "---\nuuid: keep-c\nshare: TRUE\n---\n\nUpper body.\n")

	if _, err := SetShare(scan(t, dir, "uuid"), dir, "uuid", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{"no-frontmatter.md", "shared.md", "nested/private.md", "nested/deep/upper.md"} {
		doc, raw := readDoc(t, dir, name, "uuid")
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
		doc, raw := readDoc(t, dir, name, "uuid")
		if doc.ID != want {
			t.Errorf("%s should keep UUID %q, got %q: %s", name, want, doc.ID, raw)
		}
	}
	for name, want := range map[string]string{
		"shared.md":            "Shared body.",
		"nested/private.md":    "Private body.",
		"nested/deep/upper.md": "Upper body.",
	} {
		_, raw := readDoc(t, dir, name, "uuid")
		if !strings.Contains(raw, want) {
			t.Errorf("%s lost its body: %s", name, raw)
		}
	}

	before := make(map[string]string)
	for _, name := range []string{"no-frontmatter.md", "shared.md", "nested/private.md", "nested/deep/upper.md"} {
		_, raw := readDoc(t, dir, name, "uuid")
		before[name] = raw
	}
	if changed, err := SetShare(scan(t, dir, "uuid"), dir, "uuid", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if len(changed) != 0 {
		t.Errorf("repeated SetShare changed %v", changed)
	}
	for name, want := range before {
		_, raw := readDoc(t, dir, name, "uuid")
		if raw != want {
			t.Errorf("%s changed on a repeated SetShare run:\nbefore %q\nafter  %q", name, want, raw)
		}
	}

	if _, err := SetShare(scan(t, dir, "uuid"), dir, "uuid", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{"no-frontmatter.md", "shared.md", "nested/private.md", "nested/deep/upper.md"} {
		doc, raw := readDoc(t, dir, name, "uuid")
		if !markdownindexer.SharedMetadata(doc.Metadata) {
			t.Errorf("%s should be share: true: %s", name, raw)
		}
	}
}

func TestReplaceDuplicateUUIDs(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "one.md", "---\nuuid: dup\ntitle: One\n---\n\nOne.\n")
	writeMD(t, dir, "two.md", "---\nuuid: dup\ntitle: Two\n---\n\nTwo.\n")
	writeMD(t, dir, "unique.md", "---\nuuid: keep\n---\n\nUnique.\n")

	changed, err := ReplaceDuplicateUUIDs(scan(t, dir, "uuid"), dir, "uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Strings(changed)
	if want := []string{"one.md", "two.md"}; !equalStrings(changed, want) {
		t.Errorf("changed = %v, want %v", changed, want)
	}

	ids := make(map[string]string)
	for _, name := range []string{"one.md", "two.md"} {
		doc, raw := readDoc(t, dir, name, "uuid")
		if doc.ID == "" || doc.ID == "dup" {
			t.Errorf("%s should get a fresh UUID, got %q: %s", name, doc.ID, raw)
		}
		if prev, ok := ids[doc.ID]; ok {
			t.Errorf("%s and %s share new UUID %s", name, prev, doc.ID)
		}
		ids[doc.ID] = name
		if !strings.Contains(raw, "title:") {
			t.Errorf("%s should keep its front matter: %s", name, raw)
		}
	}
	doc, raw := readDoc(t, dir, "unique.md", "uuid")
	if doc.ID != "keep" {
		t.Errorf("unique.md should keep its UUID, got %q: %s", doc.ID, raw)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
