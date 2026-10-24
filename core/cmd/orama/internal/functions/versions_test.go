package functions

import (
	"encoding/json"
	"testing"
)

// chg-956: a version's wasm_sha256 prints as the gateway sent it, and a
// version deployed before the gateway recorded one (null) prints "-".
func TestValStrOrDash_wasmSHA256(t *testing.T) {
	const digest = "93a44bbb96c751218e4c00d479e4c14358122a389acca16205b1e4d0dc5f9476"
	for body, want := range map[string]string{
		`{"wasm_sha256":"` + digest + `"}`: digest,
		`{"wasm_sha256":null}`:             "-",
		`{}`:                               "-",
	} {
		var row map[string]interface{}
		if err := json.Unmarshal([]byte(body), &row); err != nil {
			t.Fatal(err)
		}
		if got := valStrOrDash(row, "wasm_sha256"); got != want {
			t.Errorf("%s: got %q, want %q", body, got, want)
		}
	}
}
