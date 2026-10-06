package markdownindexer_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dentropy/markdown-indexer/libs/markdown-index"
)

// Example indexes a vault and walks its wikilink graph.
func Example() {
	dir, err := os.MkdirTemp("", "vault-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	note := filepath.Join(dir, "note.md")
	if err := os.WriteFile(note, []byte("---\nuuid: 11111111-1111-1111-1111-111111111111\n---\nSee [[other]].\n"), 0o644); err != nil {
		panic(err)
	}
	other := filepath.Join(dir, "other.md")
	if err := os.WriteFile(other, []byte("---\nuuid: 22222222-2222-2222-2222-222222222222\n---\nNothing here.\n"), 0o644); err != nil {
		panic(err)
	}

	idx, err := markdownindexer.Scan(dir, markdownindexer.Options{})
	if err != nil {
		panic(err)
	}

	for _, doc := range idx.Documents {
		fmt.Println(doc.RelativePath, doc.ID)
	}
	for _, edge := range idx.Edges {
		fmt.Println(edge.FromDocumentID, "->", edge.ToDocumentID)
	}

	// Output:
	// note.md 11111111-1111-1111-1111-111111111111
	// other.md 22222222-2222-2222-2222-222222222222
	// 11111111-1111-1111-1111-111111111111 -> 22222222-2222-2222-2222-222222222222
}

// ExampleIndex_ByUUID looks a document up by its UUID rather than its path.
func ExampleIndex_ByUUID() {
	dir, err := os.MkdirTemp("", "vault-byuuid-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "note.md"),
		[]byte("---\nuuid: 11111111-1111-1111-1111-111111111111\ntitle: A note\n---\nBody.\n"), 0o644); err != nil {
		panic(err)
	}

	idx, err := markdownindexer.Scan(dir, markdownindexer.Options{})
	if err != nil {
		panic(err)
	}

	doc := idx.ByUUID()["11111111-1111-1111-1111-111111111111"]
	fmt.Println(doc.FrontMatter()["title"])

	// Output:
	// A note
}

// ExampleOptions_Shared narrows the index to documents marked shareable.
func ExampleOptions_Shared() {
	dir, err := os.MkdirTemp("", "vault-shared-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			panic(err)
		}
	}
	write("shared.md", "---\nuuid: 1\nshare: true\n---\nShared.\n")
	write("private.md", "---\nuuid: 2\nshare: false\n---\nPrivate.\n")

	everything, err := markdownindexer.Scan(dir, markdownindexer.Options{})
	if err != nil {
		panic(err)
	}
	shared, err := markdownindexer.Scan(dir, markdownindexer.Options{Shared: true})
	if err != nil {
		panic(err)
	}

	fmt.Println(len(everything.Documents), len(shared.Documents))

	// Output:
	// 2 1
}

// ExampleCheck reports a vault's structural problems.
func ExampleCheck() {
	dir, err := os.MkdirTemp("", "vault-check-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "a.md"),
		[]byte("---\nuuid: 11111111-1111-1111-1111-111111111111\n---\nA.\n"), 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.md"),
		[]byte("---\nuuid: 11111111-1111-1111-1111-111111111111\n---\nB.\n"), 0o644); err != nil {
		panic(err)
	}

	problems, err := markdownindexer.Check(dir, markdownindexer.Options{})
	if err != nil {
		panic(err)
	}
	for _, p := range problems {
		fmt.Println(p.Error())
	}

	// Output:
	// a.md, b.md: duplicate UUID 11111111-1111-1111-1111-111111111111
}
