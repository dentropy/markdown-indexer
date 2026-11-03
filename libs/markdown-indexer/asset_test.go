package markdownindexer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanDoesNotHashAssetsByDefault(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "one.md", "---\nuuid: 1\n---\n\nOne.\n")
	writeFile(t, dir, "notes.txt", []byte("not markdown"))

	ix, err := Scan(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Assets) != 0 {
		t.Errorf("HashAssets is off; Assets = %+v, want none", ix.Assets)
	}
	if len(ix.Documents) != 1 {
		t.Errorf("documents = %d, want 1", len(ix.Documents))
	}
}

func TestScanHashAssets(t *testing.T) {
	dir := t.TempDir()
	png := []byte{0x89, 0x50, 0x4e, 0x47}
	pdf := []byte("%PDF-1.4")
	writeMD(t, dir, "note.md", "---\nuuid: 1\nshare: true\n---\nSee ![[diagram.png]].\n")
	writeMD(t, dir, "private.md", "---\nuuid: 2\nshare: false\n---\nPrivate.\n")
	writeFile(t, dir, "assets/diagram.png", png)
	writeFile(t, dir, "guides/manual.PDF", pdf)
	writeFile(t, dir, "notes.txt", []byte("not markdown"))
	writeFile(t, dir, ".DS_Store", []byte("junk"))
	writeFile(t, dir, ".obsidian/app.json", []byte(`{}`))
	writeMD(t, dir, "nested/deep.md", "---\nuuid: 3\n---\nDeep.\n")
	writeFile(t, dir, "nested/pic.png", png)

	ix, err := Scan(dir, Options{HashAssets: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Problems) != 0 {
		t.Fatalf("unexpected problems: %v", ix.Problems)
	}

	got := ix.ByPath()
	wantPaths := []string{"assets/diagram.png", "guides/manual.PDF", "nested/pic.png", "notes.txt"}
	if len(got) != len(wantPaths) {
		t.Fatalf("assets = %d, want %d: %+v", len(got), len(wantPaths), got)
	}
	for i, path := range wantPaths {
		if i > 0 && ix.Assets[i-1].RelativePath > ix.Assets[i].RelativePath {
			t.Errorf("assets not ordered by path: %q then %q", ix.Assets[i-1].RelativePath, ix.Assets[i].RelativePath)
		}
		a, ok := got[path]
		if !ok {
			t.Fatalf("missing asset %q in %+v", path, got)
		}
		sum := sha256.Sum256(mustRead(t, filepath.Join(dir, path)))
		if a.SHA256 != hex.EncodeToString(sum[:]) {
			t.Errorf("%s sha256 = %q, want %s", path, a.SHA256, hex.EncodeToString(sum[:]))
		}
	}
	if got["assets/diagram.png"].SHA256 != got["nested/pic.png"].SHA256 {
		t.Error("identical bytes must share a sha256")
	}
	if got["guides/manual.PDF"].Name != "manual.PDF" {
		t.Errorf("name = %q, want the basename with extension", got["guides/manual.PDF"].Name)
	}
	if _, ok := got[".DS_Store"]; ok {
		t.Error("dotfiles must not be assets")
	}
	if _, ok := got[".obsidian/app.json"]; ok {
		t.Error("files under a hidden directory must not be assets")
	}

	shared, err := Scan(dir, Options{HashAssets: true, Shared: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(shared.Documents) != 1 {
		t.Errorf("Shared should narrow documents, got %d", len(shared.Documents))
	}
	if len(shared.Assets) != len(ix.Assets) {
		t.Errorf("Shared must not drop assets, got %d want %d", len(shared.Assets), len(ix.Assets))
	}

	top, err := Scan(dir, Options{HashAssets: true, Pattern: "*.md"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := top.ByPath()["nested/pic.png"]; !ok {
		t.Error("Pattern selects documents only; nested/pic.png should still hash")
	}
	for _, a := range top.Assets {
		if isMarkdownName(a.RelativePath) {
			t.Errorf("unmatched markdown must not become an asset: %q", a.RelativePath)
		}
	}
}

func TestScanHashAssetsUnreadableIsProblem(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "one.md", "---\nuuid: 1\n---\n\nOne.\n")
	p := filepath.Join(dir, "blocked.bin")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	if _, err := os.Open(p); err == nil {
		t.Skip("process can read a 000 file; cannot test the unreadable-asset path")
	}

	ix, err := Scan(dir, Options{HashAssets: true})
	if err != nil {
		t.Fatalf("an unreadable asset must not abort the scan: %v", err)
	}
	if len(ix.Documents) != 1 {
		t.Errorf("the markdown file should still index, got %d documents", len(ix.Documents))
	}
	if len(ix.Problems) != 1 || ix.Problems[0].Path() != "blocked.bin" {
		t.Errorf("want a problem for blocked.bin, got %v", ix.Problems)
	}
}

func TestResolveAsset(t *testing.T) {
	ix := &Index{Assets: []Asset{
		{RelativePath: "assets/diagram.png", Name: "diagram.png", SHA256: "aaa"},
		{RelativePath: "guides/manual.pdf", Name: "manual.pdf", SHA256: "bbb"},
		{RelativePath: "dup/a.png", Name: "a.png", SHA256: "ccc"},
		{RelativePath: "other/a.png", Name: "a.png", SHA256: "ddd"},
	}}

	a, err := ix.ResolveAsset("assets/diagram.png", "note.md")
	if err != nil || a.SHA256 != "aaa" {
		t.Errorf("exact path: got %+v, %v", a, err)
	}
	a, err = ix.ResolveAsset("diagram.png", "note.md")
	if err != nil || a.SHA256 != "aaa" {
		t.Errorf("unique basename: got %+v, %v", a, err)
	}
	a, err = ix.ResolveAsset("../assets/diagram.png", "guides/intro.md")
	if err != nil || a.SHA256 != "aaa" {
		t.Errorf("relative to source: got %+v, %v", a, err)
	}
	if _, err := ix.ResolveAsset("a.png", "note.md"); !errors.Is(err, ErrAssetAmbiguous) {
		t.Errorf("ambiguous basename: err = %v, want ErrAssetAmbiguous", err)
	}
	if _, err := ix.ResolveAsset("missing.png", "note.md"); !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("missing: err = %v, want ErrAssetNotFound", err)
	}
	if _, err := ix.ResolveAsset("https://example.com/a.png", "note.md"); !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("URL: err = %v, want ErrAssetNotFound", err)
	}
	if _, err := ix.ResolveAsset("", "note.md"); !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("empty: err = %v, want ErrAssetNotFound", err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
