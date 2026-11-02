package markdowntools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairFrontMatterData(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		fields  []string
		wantErr bool
	}{
		{
			name:   "quotes a title containing a colon and space",
			in:     "---\ntitle: Telegraf: metrics collector\ndate: 2019-09-07\ntags: [ 'telegraf' ]\n---\n\nBody.\n",
			want:   "---\ntitle: \"Telegraf: metrics collector\"\ndate: 2019-09-07\ntags: [ 'telegraf' ]\n---\n\nBody.\n",
			fields: []string{"title"},
		},
		{
			name:   "keeps a trailing comment outside the quotes",
			in:     "---\ntitle: Telegraf: metrics collector # main\n---\n\nBody.\n",
			want:   "---\ntitle: \"Telegraf: metrics collector\" # main\n---\n\nBody.\n",
			fields: []string{"title"},
		},
		{
			name:   "quotes a value with a trailing colon",
			in:     "---\ntitle: Done:\n---\n",
			want:   "---\ntitle: \"Done:\"\n---\n",
			fields: []string{"title"},
		},
		{
			name:   "escapes quotes and backslashes",
			in:     "---\ntitle: He said \"hi\": C:\\notes\n---\n",
			want:   "---\ntitle: \"He said \\\"hi\\\": C:\\\\notes\"\n---\n",
			fields: []string{"title"},
		},
		{
			name:   "quotes nested mapping values",
			in:     "---\nmeta:\n  title: A: B\n  author: Me\n---\n",
			want:   "---\nmeta:\n  title: \"A: B\"\n  author: Me\n---\n",
			fields: []string{"title"},
		},
		{
			name:   "quotes mapping values inside a sequence",
			in:     "---\nitems:\n  - name: A: B\n---\n",
			want:   "---\nitems:\n  - name: \"A: B\"\n---\n",
			fields: []string{"name"},
		},
		{
			name:   "quotes several values in document order",
			in:     "---\ntitle: A: B\nsummary: C: D\n---\n",
			want:   "---\ntitle: \"A: B\"\nsummary: \"C: D\"\n---\n",
			fields: []string{"title", "summary"},
		},
		{
			name:   "preserves CRLF line endings",
			in:     "---\r\ntitle: A: B\r\n---\r\n\r\nBody.\r\n",
			want:   "---\r\ntitle: \"A: B\"\r\n---\r\n\r\nBody.\r\n",
			fields: []string{"title"},
		},
		{
			name: "leaves valid front matter byte-for-byte alone",
			in:   "---\ntitle: Fine\ndate: 2019-09-07\ntags: [ 'a', 'b' ]\nurl: http://example.com\n---\n\nBody.\n",
			want: "---\ntitle: Fine\ndate: 2019-09-07\ntags: [ 'a', 'b' ]\nurl: http://example.com\n---\n\nBody.\n",
		},
		{
			name: "leaves quoted values alone",
			in:   "---\ntitle: \"A: B\"\nother: 'C: D'\n---\n",
			want: "---\ntitle: \"A: B\"\nother: 'C: D'\n---\n",
		},
		{
			name: "leaves block scalars alone",
			in:   "---\ntitle: |\n  A: B\n  C: D\n---\n",
			want: "---\ntitle: |\n  A: B\n  C: D\n---\n",
		},
		{
			name: "leaves comments and colon-free comments alone",
			in:   "---\ntitle: A # note: still a comment\n---\n",
			want: "---\ntitle: A # note: still a comment\n---\n",
		},
		{
			name: "does nothing without front matter",
			in:   "Title: not front matter\n\nBody.\n",
			want: "Title: not front matter\n\nBody.\n",
		},
		{
			name: "does nothing for an empty front matter block",
			in:   "---\n---\n\nBody.\n",
			want: "---\n---\n\nBody.\n",
		},
		{
			name:    "reports an unterminated front matter block",
			in:      "---\ntitle: A: B\n",
			wantErr: true,
		},
		{
			name:    "reports front matter that is still broken",
			in:      "---\ntitle: [unclosed\n---\n",
			wantErr: true,
		},
		{
			name:    "does not rewrite a multi-line plain scalar continuation",
			in:      "---\ntitle: Telegraf\n  metrics: collector\n---\n",
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, fields, changed, err := repairFrontMatterData([]byte(c.in))
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				if got != nil {
					t.Errorf("a failed repair must not return output, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != c.want {
				t.Errorf("repairFrontMatterData =\n%q\nwant\n%q", got, c.want)
			}
			if changed != (len(c.fields) > 0) {
				t.Errorf("changed = %v, want %v", changed, len(c.fields) > 0)
			}
			if strings.Join(fields, ",") != strings.Join(c.fields, ",") {
				t.Errorf("fields = %v, want %v", fields, c.fields)
			}
		})
	}
}

func TestRepairFrontMatter(t *testing.T) {
	dir := t.TempDir()
	broken := "---\ntitle: Telegraf: metrics collector\nuuid: keep-me\nshare: true\n---\n\nBody: untouched.\n"
	writeMD(t, dir, "broken.md", broken)
	writeMD(t, dir, "valid.md", "---\ntitle: Fine\n---\n\nBody.\n")
	writeMD(t, dir, "unrepairable.md", "---\ntitle: [unclosed\n---\n\nBody.\n")

	planned, err := RepairFrontMatter(dir, RepairOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(planned.Fixes) != 1 || planned.Fixes[0].Path != "broken.md" {
		t.Fatalf("planned fixes = %+v, want broken.md", planned.Fixes)
	}
	if got := strings.Join(planned.Fixes[0].Fields, ","); got != "title" {
		t.Errorf("planned fields = %q, want title", got)
	}
	if len(planned.Unrepairable) != 1 || planned.Unrepairable[0].Path != "unrepairable.md" {
		t.Fatalf("unrepairable = %+v, want unrepairable.md", planned.Unrepairable)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "broken.md")); err != nil || string(data) != broken {
		t.Errorf("a planning pass must not write: %v %q", err, data)
	}

	applied, err := RepairFrontMatter(dir, RepairOptions{Apply: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied.Fixes) != 1 || applied.Fixes[0].Path != "broken.md" {
		t.Fatalf("applied fixes = %+v, want broken.md", applied.Fixes)
	}

	doc, raw := readDoc(t, dir, "broken.md", "uuid")
	if doc.DisplayName() != "Telegraf: metrics collector" {
		t.Errorf("title = %q, want %q: %s", doc.DisplayName(), "Telegraf: metrics collector", raw)
	}
	if doc.ID != "keep-me" {
		t.Errorf("uuid = %q, want keep-me: %s", doc.ID, raw)
	}
	if !strings.Contains(raw, "Body: untouched.") {
		t.Errorf("body was modified: %s", raw)
	}
	if _, ok := doc.FrontMatter()["share"]; !ok {
		t.Errorf("share key was lost: %s", raw)
	}

	unrepairable, err := os.ReadFile(filepath.Join(dir, "unrepairable.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(unrepairable), "title: [unclosed") {
		t.Errorf("unrepairable file must be left untouched: %s", unrepairable)
	}

	again, err := RepairFrontMatter(dir, RepairOptions{})
	if err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}
	if len(again.Fixes) != 0 {
		t.Errorf("repair is not idempotent: %+v", again.Fixes)
	}
}

func TestRepairFrontMatterPattern(t *testing.T) {
	dir := t.TempDir()
	broken := "---\ntitle: A: B\n---\n\nBody.\n"
	writeMD(t, dir, "a.md", broken)
	writeMD(t, dir, "notes/b.md", broken)

	result, err := RepairFrontMatter(dir, RepairOptions{Pattern: "notes/**/*.md", Apply: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Fixes) != 1 || result.Fixes[0].Path != "notes/b.md" {
		t.Fatalf("fixes = %+v, want notes/b.md", result.Fixes)
	}

	a, err := os.ReadFile(filepath.Join(dir, "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != broken {
		t.Errorf("a.md is outside the pattern and must be untouched: %s", a)
	}
	b, err := os.ReadFile(filepath.Join(dir, "notes/b.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `title: "A: B"`) {
		t.Errorf("notes/b.md should be repaired: %s", b)
	}
}
