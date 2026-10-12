package chainread

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
)

// TreeFrontier is a public module query: a wallet downloads the right edge of the note-commitment
// tree through the same route as the other reads, and the bytes fields arrive as base64.
func TestQuery_treeFrontierIsPublicAndDecodesItsBytes(t *testing.T) {
	root := bytes.Repeat([]byte{0xab}, 32)
	frontier := append([]byte{1, 0, 0, 0, 0, 0, 0, 0}, bytes.Repeat([]byte{0xcd}, 64)...)
	// QueryTreeFrontierResponse{height: 9, tree_size: 2, current_root, frontier}
	answer := []byte{0x08, 9, 0x10, 2, 0x1a, byte(len(root))}
	answer = append(answer, root...)
	answer = append(answer, 0x22, byte(len(frontier)))
	answer = append(answer, frontier...)

	up := &abciUpstream{value: answer}
	p := queryProxy(t, up)
	rr := getQuery(p, http.MethodGet, "/v1/chain/query/orama.shielded.v1.Query/TreeFrontier")
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %q", rr.Code, rr.Body.String())
	}
	var got struct {
		Height      string `json:"height"`
		TreeSize    string `json:"tree_size"`
		CurrentRoot string `json:"current_root"`
		Frontier    string `json:"frontier"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q: %v", rr.Body.String(), err)
	}
	if got.Height != "9" || got.TreeSize != "2" {
		t.Fatalf("height %q size %q", got.Height, got.TreeSize)
	}
	if b, err := base64.StdEncoding.DecodeString(got.CurrentRoot); err != nil || !bytes.Equal(b, root) {
		t.Fatalf("current_root %q: %v", got.CurrentRoot, err)
	}
	if b, err := base64.StdEncoding.DecodeString(got.Frontier); err != nil || !bytes.Equal(b, frontier) {
		t.Fatalf("frontier %q: %v", got.Frontier, err)
	}
	if q := up.calls[0]; q.Get("path") != `"/orama.shielded.v1.Query/TreeFrontier"` || q.Get("prove") != "false" {
		t.Fatalf("upstream query %v", q)
	}
}
