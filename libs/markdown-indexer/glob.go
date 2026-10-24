package markdownindexer

import (
	"path"
	"strings"
)

// MatchGlob reports whether name matches pattern, supporting the ** wildcard
// to match across any number of directory levels. Every other segment is
// matched with path.Match, so ? and character classes work inside a segment
// but never span a slash.
func MatchGlob(pattern, name string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(pat, seg []string) bool {
	for len(pat) > 0 {
		switch pat[0] {
		case "**":
			if len(pat) == 1 {
				return true
			}
			for i := 0; i <= len(seg); i++ {
				if matchSegments(pat[1:], seg[i:]) {
					return true
				}
			}
			return false
		default:
			if len(seg) == 0 {
				return false
			}
			ok, err := path.Match(pat[0], seg[0])
			if err != nil || !ok {
				return false
			}
			pat, seg = pat[1:], seg[1:]
		}
	}
	return len(seg) == 0
}
