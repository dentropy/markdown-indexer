package markdownindexer

import (
	"errors"
	"strings"
	"testing"
)

func TestScan(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "one.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nOne, linking [[two]].\n")
	writeMD(t, dir, "nested/two.md", "---\nuuid: 22222222-2222-2222-2222-222222222222\n---\n\nTwo.\n")
	writeMD(t, dir, "notes.txt", "not markdown")

	ix, err := Scan(dir, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ix.Problems) != 0 {
		t.Errorf("unexpected problems: %v", ix.Problems)
	}
	if len(ix.Documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(ix.Documents))
	}
	if ix.Documents[0].RelativePath != "nested/two.md" || ix.Documents[1].RelativePath != "one.md" {
		t.Errorf("documents must be ordered by relative path, got %q, %q",
			ix.Documents[0].RelativePath, ix.Documents[1].RelativePath)
	}
	if len(ix.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(ix.Edges))
	}
	for cid, e := range ix.Edges {
		if e.FromDocumentID != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("edge %s from = %q", cid, e.FromDocumentID)
		}
		if e.ToDocumentID != "22222222-2222-2222-2222-222222222222" {
			t.Errorf("edge %s to = %q, want the target's UUID", cid, e.ToDocumentID)
		}
	}
}

func TestScanDefaults(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "one.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nOne.\n")

	// The zero Options must recurse and read the default id key.
	ix, err := Scan(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Documents) != 1 || ix.Documents[0].ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("zero Options should index everything by the default id key, got %+v", ix.Documents)
	}

	// A "./" prefixed pattern is normalized, not taken literally.
	ix, err = Scan(dir, Options{Pattern: "./*.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Documents) != 1 {
		t.Errorf("a ./-prefixed pattern should be normalized, got %d documents", len(ix.Documents))
	}
}

func TestScanPattern(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "top.md", "---\nuuid: 1\n---\n\nTop.\n")
	writeMD(t, dir, "nested/deep.md", "---\nuuid: 2\n---\n\nDeep.\n")

	ix, err := Scan(dir, Options{Pattern: "*.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Documents) != 1 || ix.Documents[0].RelativePath != "top.md" {
		t.Errorf("a single-segment pattern should not recurse, got %+v", ix.Documents)
	}
}

func TestScanIDKey(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "one.md", "---\nid: custom-id\n---\n\nOne.\n")

	ix, err := Scan(dir, Options{IDKey: "id"})
	if err != nil {
		t.Fatal(err)
	}
	if ix.Documents[0].ID != "custom-id" {
		t.Errorf("ID = %q, want custom-id", ix.Documents[0].ID)
	}
}

func TestScanShared(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "shared.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\nshare: true\n---\nSee [[private]].\n")
	writeMD(t, dir, "private.md", "---\nuuid: 22222222-2222-2222-2222-222222222222\nshare: false\n---\nSee [[shared]].\n")

	all, err := Scan(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Documents) != 2 {
		t.Fatalf("the default should index every document, got %d", len(all.Documents))
	}
	if len(all.Edges) != 2 {
		t.Errorf("the default should build every edge, got %d", len(all.Edges))
	}

	shared, err := Scan(dir, Options{Shared: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(shared.Documents) != 1 || shared.Documents[0].RelativePath != "shared.md" {
		t.Errorf("Shared should narrow to shared documents, got %+v", shared.Documents)
	}
	if len(shared.Edges) != 1 {
		t.Fatalf("Shared should build edges from shared documents only, got %d", len(shared.Edges))
	}
	for cid, e := range shared.Edges {
		if e.FromDocumentID != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("edge %s must originate from the shared document, got %+v", cid, e)
		}
	}
}

func TestScanCollectsProblemsInsteadOfAborting(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "good.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nGood.\n")
	writeMD(t, dir, "broken.md", "---\ntitle: [unclosed\n---\n\nBroken.\n")

	ix, err := Scan(dir, Options{})
	if err != nil {
		t.Fatalf("a broken file must not abort the scan: %v", err)
	}
	if len(ix.Documents) != 1 || ix.Documents[0].RelativePath != "good.md" {
		t.Errorf("the healthy document should still be indexed, got %+v", ix.Documents)
	}
	if len(ix.Problems) != 1 {
		t.Fatalf("expected 1 problem, got %v", ix.Problems)
	}
	if ix.Problems[0].Path() != "broken.md" {
		t.Errorf("problem path = %q, want broken.md", ix.Problems[0].Path())
	}
	if !strings.Contains(ix.Problems[0].Error(), "broken.md") {
		t.Errorf("Problem.Error should name the file, got %q", ix.Problems[0].Error())
	}
	if ix.Err() == nil {
		t.Error("Err should report the problem")
	}
}

func TestScanIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "b.md", "---\nuuid: 2\n---\n\n[[a]]\n")
	writeMD(t, dir, "a.md", "---\nuuid: 1\n---\n\n[[b]]\n")

	first, err := Scan(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Scan(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Edges) != len(second.Edges) {
		t.Fatalf("edge count changed between scans: %d vs %d", len(first.Edges), len(second.Edges))
	}
	for cid := range first.Edges {
		if _, ok := second.Edges[cid]; !ok {
			t.Errorf("edge %s missing from the second scan", cid)
		}
	}
}

func TestScanMissingDirectory(t *testing.T) {
	if _, err := Scan(t.TempDir()+"/nope", Options{}); err == nil {
		t.Fatal("want an error when the vault cannot be walked")
	}
}

func TestScanErrNilWhenClean(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "one.md", "---\nuuid: 1\n---\n\nOne.\n")
	ix, err := Scan(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if ix.Err() != nil {
		t.Errorf("Err = %v, want nil for a clean vault", ix.Err())
	}
}

func TestCheckReportsProblems(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "one.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nOne.\n")
	writeMD(t, dir, "two.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nTwo.\n")
	writeMD(t, dir, "broken.md", "---\ntitle: [unclosed\n---\n\nBroken.\n")
	writeMD(t, dir, "missing-id.md", "---\ntitle: No UUID\n---\n\nBody.\n")
	writeMD(t, dir, "no-frontmatter.md", "Deliberately no front matter.\n")

	problems, err := Check(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 3 {
		t.Fatalf("expected 3 problems, got %d: %v", len(problems), problems)
	}
	joined := problemsErrorString(problems)
	for _, want := range []string{"broken.md", "duplicate UUID", "missing-id.md"} {
		if !strings.Contains(joined, want) {
			t.Errorf("problems should mention %q, got %q", want, joined)
		}
	}
	if strings.Contains(joined, "no-frontmatter.md") {
		t.Errorf("files without front matter must be tolerated, got %q", joined)
	}
	if !errors.Is(problems[2].Err, ErrMissingUUID) {
		t.Errorf("the missing-id problem should wrap ErrMissingUUID, got %v", problems[2].Err)
	}
}

func TestCheckClean(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "one.md", "---\nuuid: 11111111-1111-1111-1111-111111111111\n---\n\nOne.\n")
	writeMD(t, dir, "no-frontmatter.md", "No front matter is fine here.\n")

	problems, err := Check(dir, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("expected a clean vault, got %v", problems)
	}
}

func TestCheckCustomIDKey(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "one.md", "---\nid: \"1\"\n---\n\nOne.\n")

	if problems, _ := Check(dir, Options{IDKey: "id"}); len(problems) != 0 {
		t.Errorf("the configured id key should satisfy Check, got %v", problems)
	}
	problems, err := Check(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Errorf("the default id key should report the missing uuid, got %v", problems)
	}
}

func problemsErrorString(problems []Problem) string {
	var b strings.Builder
	for _, p := range problems {
		b.WriteString(p.Error())
		b.WriteString("\n")
	}
	return b.String()
}
