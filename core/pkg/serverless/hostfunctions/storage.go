package hostfunctions

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/Orama-Network/orama/core/pkg/serverless"
)

// StoragePut uploads data to IPFS and returns the CID.
func (h *HostFunctions) StoragePut(ctx context.Context, data []byte) (string, error) {
	if h.storage == nil {
		return "", &serverless.HostFunctionError{Function: "storage_put", Cause: serverless.ErrStorageUnavailable}
	}

	reader := bytes.NewReader(data)
	resp, err := h.storage.Add(ctx, reader, "function-data")
	if err != nil {
		return "", &serverless.HostFunctionError{Function: "storage_put", Cause: err}
	}

	return resp.Cid, nil
}

// StorageGet retrieves data from IPFS by CID. The CID must be bare.
func (h *HostFunctions) StorageGet(ctx context.Context, cid string) ([]byte, error) {
	if h.storage == nil {
		return nil, &serverless.HostFunctionError{Function: "storage_get", Cause: serverless.ErrStorageUnavailable}
	}

	// GetStored, not Get: a CID nobody pins is ipfs.ErrNotInPinset at once,
	// instead of a networked search that can only run out the deadline.
	content, err := h.storage.GetStored(ctx, cid, h.ipfsAPIURL, 0)
	if err != nil {
		return nil, &serverless.HostFunctionError{Function: "storage_get", Cause: err}
	}
	defer content.Body.Close()

	data, err := io.ReadAll(content.Body)
	if err != nil {
		return nil, &serverless.HostFunctionError{Function: "storage_get", Cause: fmt.Errorf("failed to read data: %w", err)}
	}

	return data, nil
}
