package markdowntools

import (
	"strings"
	"testing"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
)

func TestRewriteAssetLinks(t *testing.T) {
	ix := &markdownindexer.Index{Assets: []markdownindexer.Asset{
		{RelativePath: "assets/diagram.png", Name: "diagram.png", SHA256: strings.Repeat("a", 64)},
		{RelativePath: "guides/manual.pdf", Name: "manual.pdf", SHA256: strings.Repeat("b", 64)},
	}}
	used := map[string]markdownindexer.Asset{}
	body := RewriteAssetLinks(
		"See ![[diagram.png]] and ![alt](assets/diagram.png) and [[manual.pdf|the manual]] and [[note]] and ![[note]] and ![](https://example.com/a.png).\n",
		"notes/idea.md",
		ix,
		used,
	)
	png := strings.Repeat("a", 64) + ".png"
	pdf := strings.Repeat("b", 64) + ".pdf"
	if !strings.Contains(body, "![](assets/"+png+")") {
		t.Errorf("wikilink embed not rewritten: %s", body)
	}
	if !strings.Contains(body, "![alt](assets/"+png+")") {
		t.Errorf("markdown image not rewritten: %s", body)
	}
	if !strings.Contains(body, "[the manual](assets/"+pdf+")") {
		t.Errorf("pdf wikilink not rewritten: %s", body)
	}
	if !strings.Contains(body, "[[note]]") || !strings.Contains(body, "![[note]]") {
		t.Errorf("document links must stay: %s", body)
	}
	if !strings.Contains(body, "![](https://example.com/a.png)") {
		t.Errorf("URL image must stay: %s", body)
	}
	if len(used) != 2 {
		t.Errorf("used = %d, want 2: %+v", len(used), used)
	}
}

func TestRewriteAssetLinksLeavesMissing(t *testing.T) {
	ix := &markdownindexer.Index{}
	got := RewriteAssetLinks("![[gone.png]]", "note.md", ix, nil)
	if got != "![[gone.png]]" {
		t.Errorf("missing asset must stay as written, got %q", got)
	}
}
