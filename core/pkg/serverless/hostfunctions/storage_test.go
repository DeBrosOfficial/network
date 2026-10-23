package hostfunctions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/ipfs"
)

// storedOnly answers GetStored only; Get would be the networked search that
// a gone CID can only run out (bugboard #432).
type storedOnly struct {
	ipfs.IPFSClient
	gone bool
}

func (s storedOnly) GetStored(_ context.Context, cid, _ string, _ time.Duration) (*ipfs.StoredContent, error) {
	if s.gone {
		return nil, fmt.Errorf("%w: %s", ipfs.ErrNotInPinset, cid)
	}
	return &ipfs.StoredContent{Body: io.NopCloser(bytes.NewReader([]byte("bytes"))), Size: 5}, nil
}

func TestStorageGet_readsThroughGetStored(t *testing.T) {
	h := &HostFunctions{storage: storedOnly{}}
	got, err := h.StorageGet(context.Background(), "QmX")
	if err != nil || string(got) != "bytes" {
		t.Fatalf("StorageGet = %q, %v", got, err)
	}
}

func TestStorageGet_goneCIDIsNotInPinset(t *testing.T) {
	h := &HostFunctions{storage: storedOnly{gone: true}}
	if _, err := h.StorageGet(context.Background(), "QmGone"); !errors.Is(err, ipfs.ErrNotInPinset) {
		t.Fatalf("err = %v, want ipfs.ErrNotInPinset", err)
	}
}
