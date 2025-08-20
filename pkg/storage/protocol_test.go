package storage

import "testing"

func TestRequestResponseJSON(t *testing.T) {
	req := &StorageRequest{Type: MessageTypePut, Key: "k", Value: []byte("v"), Namespace: "ns"}
	b, err := req.Marshal()
	if err != nil { t.Fatal(err) }
	var out StorageRequest
	if err := out.Unmarshal(b); err != nil { t.Fatal(err) }
	if out.Type != MessageTypePut || out.Key != "k" || out.Namespace != "ns" {
		t.Fatalf("roundtrip mismatch: %+v", out)
	}

	resp := &StorageResponse{Success: true, Keys: []string{"a"}, Exists: true}
	b, err = resp.Marshal()
	if err != nil { t.Fatal(err) }
	var outR StorageResponse
	if err := outR.Unmarshal(b); err != nil { t.Fatal(err) }
	if !outR.Success || !outR.Exists || len(outR.Keys) != 1 {
		t.Fatalf("resp mismatch: %+v", outR)
	}
}
