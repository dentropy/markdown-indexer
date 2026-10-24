package markdownindexer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	cid "github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/multiformats/go-multicodec"
	"gopkg.in/yaml.v3"
)

const frontMatterDelimiter = "---"

// Document is one markdown file as indexed. Its identity is its UUID, not its
// path: a rename leaves the same document in place with a new RelativePath.
type Document struct {
	// RelativePath is the path of the markdown file relative to the vault root,
	// always using forward slashes regardless of OS.
	RelativePath string `json:"relativePath"`

	// Name is the file name without the .md extension.
	Name string `json:"name"`

	// ModifiedUnix is the time the document was last edited, as a unix
	// timestamp in seconds.
	ModifiedUnix int64 `json:"modifiedUnix"`

	// Metadata holds the parsed YAML front matter re-encoded as JSON.
	Metadata json.RawMessage `json:"metadata"`

	// FrontmatterCID is the CIDv1 (dag-json codec, sha2-256) of the front
	// matter serialized as DAG-JSON. Identical front matter always yields the
	// same address.
	FrontmatterCID string `json:"frontmatter_cid"`

	// RawMarkdown is the raw markdown content of the file with the YAML front
	// matter stripped out.
	RawMarkdown string `json:"raw_markdown"`

	// RawMarkdownHash is the hex-encoded sha256 of the raw markdown body.
	RawMarkdownHash string `json:"raw_markdown_hash"`

	// ID is the document UUID read from the YAML front matter. It is not
	// serialized as a field: it is the document's key in an index. A document
	// whose front matter has no UUID gets an empty ID and is still carried by
	// Index.Documents.
	ID string `json:"-"`
}

// FrontMatter returns the document's front matter decoded into a map. It
// returns an empty map when the front matter is absent or undecodable.
func (d Document) FrontMatter() map[string]any {
	var m map[string]any
	if err := json.Unmarshal(d.Metadata, &m); err != nil {
		return map[string]any{}
	}
	return m
}

// Parse indexes a single markdown document held in memory. relPath is the
// document's path relative to the vault root, used for both RelativePath and
// Name. idKey is the front matter key holding the document's UUID, defaulting
// to DefaultIDKey when empty.
func Parse(data []byte, relPath string, modTime time.Time, idKey string) (Document, error) {
	if idKey == "" {
		idKey = DefaultIDKey
	}

	frontMatter, err := extractFrontMatter(data)
	if err != nil {
		return Document{}, fmt.Errorf("parse front matter: %w", err)
	}

	metadata, err := json.Marshal(frontMatter)
	if err != nil {
		return Document{}, fmt.Errorf("marshal metadata to JSON: %w", err)
	}

	fmCID, err := FrontMatterCID(frontMatter)
	if err != nil {
		return Document{}, err
	}

	body := string(stripFrontMatter(data))
	bodyHash := sha256.Sum256([]byte(body))

	return Document{
		RelativePath:    relPath,
		Name:            FileNameWithoutExt(relPath),
		ModifiedUnix:    modTime.Unix(),
		Metadata:        metadata,
		FrontmatterCID:  fmCID,
		RawMarkdown:     body,
		RawMarkdownHash: hex.EncodeToString(bodyHash[:]),
		ID:              uuidFrom(metadata, idKey),
	}, nil
}

// FrontMatterCID returns the CIDv1 of the front matter encoded as DAG-JSON.
// The CID uses the dag-json multicodec and a sha2-256 multihash, so it
// identifies the exact serialized bytes of the front matter.
func FrontMatterCID(frontMatter map[string]any) (string, error) {
	data, err := json.Marshal(frontMatter)
	if err != nil {
		return "", fmt.Errorf("marshal front matter to JSON: %w", err)
	}
	link, err := dagJSONCID(data)
	if err != nil {
		return "", fmt.Errorf("front matter: %w", err)
	}
	return link, nil
}

// dagJSONCID returns the CIDv1 (dag-json codec, sha2-256) of the exact bytes
// of a serialized DAG-JSON document.
func dagJSONCID(data []byte) (string, error) {
	nb := basicnode.Prototype.Any.NewBuilder()
	if err := dagjson.Decode(nb, bytes.NewReader(data)); err != nil {
		return "", fmt.Errorf("decode dag-json: %w", err)
	}

	lsys := cidlink.DefaultLinkSystem()
	lnk, err := lsys.ComputeLink(cidlink.LinkPrototype{
		Prefix: cid.Prefix{
			Version:  1,
			Codec:    uint64(multicodec.DagJson),
			MhType:   uint64(multicodec.Sha2_256),
			MhLength: -1,
		},
	}, nb.Build())
	if err != nil {
		return "", fmt.Errorf("compute dag-json CID: %w", err)
	}
	return lnk.String(), nil
}

// FileNameWithoutExt returns the base file name without its .md extension.
func FileNameWithoutExt(relPath string) string {
	return strings.TrimSuffix(path.Base(relPath), filepath.Ext(relPath))
}

// extractFrontMatter parses the YAML block delimited by --- lines at the top
// of a markdown file. Files without front matter yield an empty map.
func extractFrontMatter(data []byte) (map[string]any, error) {
	if !hasFrontMatter(data) {
		return map[string]any{}, nil
	}

	body := frontMatterStart(data)
	end := findClosingDelimiter(body)
	if end < 0 {
		return nil, ErrUnterminatedFrontMatter
	}

	var fm map[string]any
	if err := yaml.Unmarshal(body[:end], &fm); err != nil {
		return nil, err
	}
	return fm, nil
}

// stripFrontMatter returns the file content without its leading YAML front
// matter block. Files without front matter are returned unchanged.
func stripFrontMatter(data []byte) []byte {
	if !hasFrontMatter(data) {
		return data
	}
	body := frontMatterStart(data)
	end := findClosingDelimiter(body)
	if end < 0 {
		return data
	}
	return bytes.TrimLeft(body[end:], " \t\r\n")
}

// ErrUnterminatedFrontMatter reports front matter opened with --- but never
// closed, so the rest of the file cannot be told apart from the YAML block.
var ErrUnterminatedFrontMatter = errors.New("unterminated front matter: missing closing ---")

// frontMatterStart returns the bytes just past the opening --- line, skipping
// any trailing horizontal whitespace and the line break itself.
func frontMatterStart(data []byte) []byte {
	body := data[len(frontMatterDelimiter):]
	body = bytes.TrimLeft(body, " \t")
	if len(body) > 0 && (body[0] == '\n' || body[0] == '\r') {
		body = trimOneBreak(body)
	}
	return body
}

func hasFrontMatter(data []byte) bool {
	if !bytes.HasPrefix(data, []byte(frontMatterDelimiter)) {
		return false
	}
	rest := data[len(frontMatterDelimiter):]
	if len(rest) == 0 {
		return false
	}
	c := rest[0]
	return c == '\n' || c == '\r' || c == ' ' || c == '\t'
}

func trimOneBreak(b []byte) []byte {
	if b[0] == '\r' && len(b) > 1 && b[1] == '\n' {
		return b[2:]
	}
	return b[1:]
}

func findClosingDelimiter(body []byte) int {
	for _, line := range bytes.Split(body, []byte("\n")) {
		trimmed := bytes.TrimRight(line, " \t\r")
		if bytes.Equal(trimmed, []byte(frontMatterDelimiter)) {
			return bytes.Index(body, line) + len(line)
		}
	}
	return -1
}

// uuidFrom reads the document UUID from a key in the JSON-encoded metadata.
func uuidFrom(metadata json.RawMessage, key string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(metadata, &m); err != nil {
		return ""
	}
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var id string
	if err := json.Unmarshal(raw, &id); err != nil || strings.TrimSpace(id) == "" {
		return ""
	}
	return id
}
