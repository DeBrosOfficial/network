package serverless

import (
	"context"
	"encoding/json"
	"testing"
)

// chg-956: every version names the SHA-256 of the WASM as uploaded, the same
// for the same bytes however the upload was sealed, on every read path.
func TestRegister_recordsTheUploadedWASMSHA256(t *testing.T) {
	r, _ := newVersionedRegistry(t)
	ctx := context.Background()
	wasm := []byte("\x00asm\x01\x00\x00\x00")
	// printf '\0asm\1\0\0\0' | shasum -a 256
	const want = "93a44bbb96c751218e4c00d479e4c14358122a389acca16205b1e4d0dc5f9476"
	for i := 0; i < 2; i++ {
		if _, err := r.Register(ctx, &FunctionDefinition{Name: "fn", Namespace: "ns"}, wasm); err != nil {
			t.Fatalf("deploy %d: %v", i+1, err)
		}
	}
	if _, err := r.Register(ctx, &FunctionDefinition{Name: "other", Namespace: "ns"}, []byte("other")); err != nil {
		t.Fatalf("deploy other: %v", err)
	}

	versions, err := r.ListVersions(ctx, "ns", "fn")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("want 2 versions, got %d", len(versions))
	}
	current, err := r.Get(ctx, "ns", "fn", 0)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	all, err := r.ListAll(ctx, "ns")
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, fn := range append(append(versions, current), all...) {
		if fn.Name != "fn" {
			continue
		}
		if fn.WASMSHA256 == nil || *fn.WASMSHA256 != want {
			t.Errorf("%s@%d wasm_sha256 = %v, want %s", fn.Name, fn.Version, fn.WASMSHA256, want)
		}
	}
	other, err := r.Get(ctx, "ns", "other", 0)
	if err != nil {
		t.Fatalf("Get other: %v", err)
	}
	if other.WASMSHA256 == nil || *other.WASMSHA256 == want {
		t.Errorf("different bytes gave wasm_sha256 %v", other.WASMSHA256)
	}
}

// A version deployed before migration 093 has no hash: the record answers
// null, never an empty string.
func TestFunction_wasmSHA256OfAnOlderVersionIsNull(t *testing.T) {
	r, db := newVersionedRegistry(t)
	if _, err := db.Exec(`INSERT INTO functions (id, name, namespace, version, wasm_cid, created_by) VALUES ('old', 'legacy', 'ns', 1, 'cid', 'ns')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	fn, err := r.Get(context.Background(), "ns", "legacy", 0)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fn.WASMSHA256 != nil {
		t.Fatalf("wasm_sha256 of a pre-093 row = %q, want nil", *fn.WASMSHA256)
	}
	raw, err := json.Marshal(fn)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if v, ok := fields["wasm_sha256"]; !ok || v != nil {
		t.Errorf("JSON wasm_sha256 = %v (present %v), want null", v, ok)
	}
}
