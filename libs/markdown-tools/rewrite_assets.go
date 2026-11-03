package markdowntools

import (
	"regexp"
	"strings"

	"github.com/dentropy/markdown-indexer/libs/markdown-indexer"
)

// These patterns must stay in lockstep with markdownindexer/edge.go.
var (
	assetWikilinkEmbed = regexp.MustCompile(`!\[\[([^\]\|#]+)(?:#([^\]\|]+))?(?:\|([^\]]+))?\]\]`)
	assetWikilink      = regexp.MustCompile(`\[\[([^\]\|#]+)(?:#([^\]\|]+))?(?:\|([^\]]+))?\]\]`)
	assetMarkdownLink  = regexp.MustCompile(`\[([^\]\n]*)\]\(([^)\s]+)\)`)
)

// RewriteAssetLinks replaces vault asset references in body with site-relative
// markdown links to assets/<sha256>.<ext>. Document wikilinks, markdown embeds,
// and URL-scheme targets are left as written. used collects each asset that
// resolved, keyed by published file name.
func RewriteAssetLinks(body, docRelPath string, ix *markdownindexer.Index, used map[string]markdownindexer.Asset) string {
	if used == nil {
		used = map[string]markdownindexer.Asset{}
	}
	body = assetWikilinkEmbed.ReplaceAllStringFunc(body, func(raw string) string {
		m := assetWikilinkEmbed.FindStringSubmatch(raw)
		if m == nil {
			return raw
		}
		target := strings.TrimSpace(m[1])
		alias := ""
		if len(m) > 3 {
			alias = strings.TrimSpace(m[3])
		}
		a, err := ix.ResolveAsset(target, docRelPath)
		if err != nil {
			return raw
		}
		name := AssetFileName(a)
		used[name] = a
		alt := alias
		return "![" + alt + "](assets/" + name + ")"
	})
	body = replaceMarkdownImages(body, docRelPath, ix, used)
	body = assetWikilink.ReplaceAllStringFunc(body, func(raw string) string {
		m := assetWikilink.FindStringSubmatch(raw)
		if m == nil {
			return raw
		}
		target := strings.TrimSpace(m[1])
		alias := ""
		if len(m) > 3 {
			alias = strings.TrimSpace(m[3])
		}
		a, err := ix.ResolveAsset(target, docRelPath)
		if err != nil {
			return raw
		}
		name := AssetFileName(a)
		used[name] = a
		title := alias
		if title == "" {
			title = target
		}
		return "[" + title + "](assets/" + name + ")"
	})
	return body
}

func replaceMarkdownImages(body, docRelPath string, ix *markdownindexer.Index, used map[string]markdownindexer.Asset) string {
	locs := assetMarkdownLink.FindAllStringSubmatchIndex(body, -1)
	if len(locs) == 0 {
		return body
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		if loc[0] == 0 || body[loc[0]-1] != '!' {
			continue
		}
		alt := ""
		if loc[2] >= 0 {
			alt = body[loc[2]:loc[3]]
		}
		dest := body[loc[4]:loc[5]]
		a, err := ix.ResolveAsset(dest, docRelPath)
		if err != nil {
			continue
		}
		name := AssetFileName(a)
		used[name] = a
		b.WriteString(body[last:loc[0]])
		b.WriteString("[")
		b.WriteString(alt)
		b.WriteString("](assets/")
		b.WriteString(name)
		b.WriteString(")")
		last = loc[1]
	}
	b.WriteString(body[last:])
	return b.String()
}
