package markdowntools

import "testing"

func TestSetFrontMatterValue(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		key     string
		value   string
		want    string
		wantErr bool
	}{
		{
			name:  "creates front matter when absent",
			in:    "Body.\n",
			key:   "uuid",
			value: "abc",
			want:  "---\nuuid: abc\n---\n\nBody.\n",
		},
		{
			name:  "replaces an existing key",
			in:    "---\nuuid: old\ntitle: Keep\n---\n\nBody.\n",
			key:   "uuid",
			value: "new",
			want:  "---\nuuid: new\ntitle: Keep\n---\n\nBody.\n",
		},
		{
			name:  "appends a missing key",
			in:    "---\ntitle: Keep\n---\n\nBody.\n",
			key:   "share",
			value: "false",
			want:  "---\ntitle: Keep\nshare: false\n---\n\nBody.\n",
		},
		{
			name:  "appends to an empty front matter block",
			in:    "---\n---\n\nBody.\n",
			key:   "share",
			value: "true",
			want:  "---\nshare: true\n---\n\nBody.\n",
		},
		{
			name:    "reports unterminated front matter",
			in:      "---\ntitle: broken\n",
			key:     "uuid",
			value:   "abc",
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := SetFrontMatterValue([]byte(c.in), c.key, c.value)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != c.want {
				t.Errorf("SetFrontMatterValue =\n%s\nwant\n%s", got, c.want)
			}
		})
	}
}
