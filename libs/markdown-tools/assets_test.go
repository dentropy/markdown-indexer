package markdowntools

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
)

func TestWriteAssets(t *testing.T) {
	vault := t.TempDir()
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0a}
	if err := os.MkdirAll(filepath.Join(vault, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "assets", "diagram.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "assets", "copy.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(png)
	hash := hex.EncodeToString(sum[:])

	out := filepath.Join(t.TempDir(), "assets")
	assets := []markdownindexer.Asset{
		{RelativePath: "assets/diagram.png", Name: "diagram.png", SHA256: hash},
		{RelativePath: "assets/copy.png", Name: "copy.png", SHA256: hash},
	}
	written, removed, err := WriteAssets(assets, vault, out, WriteOptions{Prune: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 || written[0] != hash+".png" {
		t.Errorf("written = %v, want one hashed png", written)
	}
	if len(removed) != 0 {
		t.Errorf("removed = %v", removed)
	}
	got, err := os.ReadFile(filepath.Join(out, hash+".png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(png) {
		t.Fatal("published bytes must match the vault file")
	}

	written, removed, err = WriteAssets(assets, vault, out, WriteOptions{Prune: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 || len(removed) != 0 {
		t.Errorf("second run should be a no-op, written=%v removed=%v", written, removed)
	}

	if err := os.WriteFile(filepath.Join(out, "stale.bin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, removed, err = WriteAssets(assets, vault, out, WriteOptions{Prune: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != "stale.bin" {
		t.Errorf("removed = %v, want stale.bin", removed)
	}
}

func TestWriteAssetsRejectsBadHash(t *testing.T) {
	_, _, err := WriteAssets([]markdownindexer.Asset{{
		RelativePath: "x.png",
		Name:         "x.png",
		SHA256:       "not-a-hash",
	}}, t.TempDir(), t.TempDir(), WriteOptions{})
	if err == nil {
		t.Fatal("want an error for a non-hex sha256")
	}
}
