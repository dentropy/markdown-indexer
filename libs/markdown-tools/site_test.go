package markdowntools

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
)

func TestWriteSite(t *testing.T) {
	vault := t.TempDir()
	png := []byte("PNGDATA")
	if err := os.MkdirAll(filepath.Join(vault, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "assets", "diagram.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(png)
	hash := hex.EncodeToString(sum[:])

	docs := []markdownindexer.Document{
		{
			RelativePath:    "alpha.md",
			Name:            "alpha",
			Metadata:        json.RawMessage(`{"title":"Alpha","uuid":"aaa"}`),
			FrontmatterCID:  "bafybeiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			RawMarkdown:     "See ![[diagram.png]] and [[beta]].\n",
			RawMarkdownHash: strings.Repeat("1", 64),
			ID:              "aaa",
		},
		{
			RelativePath:    "beta.md",
			Name:            "beta",
			Metadata:        json.RawMessage(`{"title":"Beta","uuid":"bbb"}`),
			FrontmatterCID:  "bafybeiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			RawMarkdown:     "Beta.\n",
			RawMarkdownHash: strings.Repeat("2", 64),
			ID:              "bbb",
		},
	}
	assets := []markdownindexer.Asset{{
		RelativePath: "assets/diagram.png",
		Name:         "diagram.png",
		SHA256:       hash,
	}}

	out := filepath.Join(t.TempDir(), "site")
	res, err := WriteSite(docs, assets, vault, out, WriteOptions{Prune: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ManifestWritten {
		t.Error("expected to write manifest.json")
	}
	if _, err := os.Stat(filepath.Join(out, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	article, err := os.ReadFile(filepath.Join(out, "articles", "aaa.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(article), "assets/"+hash+".png") {
		t.Errorf("article body should use the hashed asset, got %s", article)
	}
	if strings.Contains(string(article), `"metadata"`) {
		t.Error("article must not inline metadata")
	}
	if _, err := os.Stat(filepath.Join(out, "frontmatter", docs[0].FrontmatterCID+".json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "assets", hash+".png")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "backlinks", "aaa.json")); err != nil {
		t.Fatal(err)
	}

	res, err = WriteSite(docs, assets, vault, out, WriteOptions{Prune: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.ManifestWritten || len(res.ArticlesWritten) != 0 || len(res.AssetsWritten) != 0 {
		t.Errorf("second run should be a no-op: %+v", res)
	}
}

func TestWriteSiteShareDoesNotLeakUnreferencedAsset(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "secret.png"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("secret"))
	hash := hex.EncodeToString(sum[:])
	docs := []markdownindexer.Document{{
		RelativePath:    "public.md",
		Name:            "public",
		Metadata:        json.RawMessage(`{"uuid":"aaa"}`),
		FrontmatterCID:  "bafybeibbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RawMarkdown:     "No assets here.\n",
		RawMarkdownHash: strings.Repeat("3", 64),
		ID:              "aaa",
	}}
	assets := []markdownindexer.Asset{{
		RelativePath: "secret.png",
		Name:         "secret.png",
		SHA256:       hash,
	}}
	out := filepath.Join(t.TempDir(), "site")
	if _, err := WriteSite(docs, assets, vault, out, WriteOptions{Prune: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "assets", hash+".png")); !os.IsNotExist(err) {
		t.Errorf("unreferenced asset must not be published, err=%v", err)
	}
}
