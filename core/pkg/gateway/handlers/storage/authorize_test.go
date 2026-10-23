package storage

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gwauth "github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
	"github.com/Orama-Network/orama/core/pkg/ipfs"
	"github.com/Orama-Network/orama/core/pkg/logging"
)

// The selector is applied before IPFS is touched, which is what lets these run
// without a cluster: a refused upload never reaches one.

func storageHandlers(t *testing.T) *Handlers {
	t.Helper()
	logger, err := logging.NewColoredLogger(logging.ComponentGeneral, false)
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	// A working IPFS client, so a request that gets past the selector fails
	// somewhere else — or not at all. What is being tested is which requests
	// reach it.
	return &Handlers{
		ipfsClient: &mockIPFSClient{
			addResp: &ipfs.AddResponse{Cid: "bafyupload", Name: "uploaded", Size: 5},
			pinResp: &ipfs.PinResponse{},
		},
		logger: logger,
	}
}

// uploadWithGrant builds a JSON upload carrying a narrowed grant.
func uploadWithGrant(t *testing.T, name, selector string) *http.Request {
	t.Helper()
	body := `{"data":"aGVsbG8=","name":"` + name + `"}`
	r := httptest.NewRequest(http.MethodPost, "/v1/storage/upload", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(r.Context(), ctxkeys.NamespaceOverride, "anchat")
	if selector != "" {
		ctx = context.WithValue(ctx, ctxkeys.Permissions,
			gwauth.PermissionsFor(gwauth.RoleRuntime, selector))
	}
	return r.WithContext(ctx)
}

// A grant narrowed to `storage:avatars/*` says where this credential may write,
// and the uploader names what it writes — the way an object store works.
func TestUploadHandler_appliesTheStorageSelector(t *testing.T) {
	h := storageHandlers(t)

	t.Run("a name inside the selector is not refused", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.UploadHandler(rec, uploadWithGrant(t, "avatars/me.png", "storage:avatars/*"))

		if rec.Code == http.StatusForbidden {
			t.Fatalf("a name inside the selector was refused: %s", rec.Body.String())
		}
	})

	t.Run("a name outside it is refused", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.UploadHandler(rec, uploadWithGrant(t, "keys/private.pem", "storage:avatars/*"))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
	})

	// The name is the only thing the selector compares against, so a name that
	// spells its way out of the prefix is refused as a name rather than
	// resolved into one.
	t.Run("a name containing .. is refused as a name", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.UploadHandler(rec, uploadWithGrant(t, "avatars/../keys/private.pem", "storage:avatars/*"))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "..") {
			t.Errorf("the refusal does not say what is wrong with the name: %s", rec.Body.String())
		}
	})

	// A name that differs only in spelling is the same object, or the boundary
	// depends on how the caller typed the request.
	t.Run("a name is normalised before it is compared", func(t *testing.T) {
		for _, name := range []string{"/avatars/me.png", "avatars//me.png", "./avatars/me.png"} {
			rec := httptest.NewRecorder()
			h.UploadHandler(rec, uploadWithGrant(t, name, "storage:avatars/*"))
			if rec.Code == http.StatusForbidden {
				t.Errorf("%q was refused although it names the same object as avatars/me.png", name)
			}
		}
	})

	// An unnamed upload cannot be placed, so a narrowed grant refuses it and an
	// unnarrowed one is unaffected.
	t.Run("an unnamed upload is refused for a narrowed grant", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.UploadHandler(rec, uploadWithGrant(t, "", "storage:avatars/*"))
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d: %s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
	})

	t.Run("an unnarrowed grant is untouched", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.UploadHandler(rec, uploadWithGrant(t, "keys/private.pem", ""))
		if rec.Code == http.StatusForbidden {
			t.Fatalf("an unnarrowed grant was refused: %s", rec.Body.String())
		}
	})
}

// A CID this namespace has no name for is an object the selector cannot be
// compared against. For a narrowed grant that is a refusal: "I could not work
// out what you are touching" is not a reason to allow it.
func TestAuthorizeCID_refusesAnUnnamedObjectForANarrowedGrant(t *testing.T) {
	h := storageHandlers(t)

	r := httptest.NewRequest(http.MethodGet, "/v1/storage/get/bafyunknown", nil)
	ctx := context.WithValue(r.Context(), ctxkeys.Permissions,
		gwauth.PermissionsFor(gwauth.RoleRuntime, "storage:avatars/*"))
	rec := httptest.NewRecorder()

	// h.db is nil, so no name can be read — the same shape as a CID with no row.
	if h.authorizeCID(rec, r.WithContext(ctx), "bafyunknown", "anchat", gwauth.ActionRead) {
		t.Fatal("an object with no recorded name was permitted by a narrowed grant")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	t.Run("and permits it for an unnarrowed one", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/v1/storage/get/bafyunknown", nil)
		rec := httptest.NewRecorder()
		if !h.authorizeCID(rec, r, "bafyunknown", "anchat", gwauth.ActionRead) {
			t.Errorf("an unnarrowed grant was refused: %s", rec.Body.String())
		}
	})
}

// Content to be pinned is imported locally and pinned once by pinAsync with
// its replication factor: Add pinned it everywhere first, and narrowing that
// made every other peer start fetching it and then cancel and unpin it, churn
// that held the cluster's pin slots (stagenet e2e, 2026-10-03).
func TestUploadHandler_contentToBePinnedIsImportedLocally(t *testing.T) {
	h := storageHandlers(t)
	m := h.ipfsClient.(*mockIPFSClient)
	h.UploadHandler(httptest.NewRecorder(), uploadWithGrant(t, "doc.txt", ""))
	if m.localAdds.Load() != 1 || m.adds.Load() != 0 {
		t.Fatalf("AddLocal %d, Add %d: content to be pinned must be imported without the pin-everywhere", m.localAdds.Load(), m.adds.Load())
	}
}

// uploadWithPinField is a multipart upload of doc.txt carrying pin=value.
func uploadWithPinField(t *testing.T, value string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "doc.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := mw.WriteField("pin", value); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/storage/upload", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r.WithContext(context.WithValue(r.Context(), ctxkeys.NamespaceOverride, "anchat"))
}

// bugboard #432: pin=false pinned on every peer with no reference. Every stored
// object is pinned now, and an upload asking otherwise is refused before
// anything is imported.
func TestUploadHandler_refusesAnUnpinnedUpload(t *testing.T) {
	for _, value := range []string{"false", "0", "no"} {
		h := storageHandlers(t)
		m := h.ipfsClient.(*mockIPFSClient)
		rec := httptest.NewRecorder()
		h.UploadHandler(rec, uploadWithPinField(t, value))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "UNPINNED_UPLOAD_REFUSED") {
			t.Fatalf("pin=%s: status %d body %s, want 400 UNPINNED_UPLOAD_REFUSED", value, rec.Code, rec.Body.String())
		}
		if m.adds.Load() != 0 || m.localAdds.Load() != 0 {
			t.Fatalf("pin=%s: content was imported (Add %d, AddLocal %d)", value, m.adds.Load(), m.localAdds.Load())
		}
	}
}

// SDKs before 0.3.5 send pin=true on every upload; it is still accepted.
func TestUploadHandler_acceptsPinTrue(t *testing.T) {
	h := storageHandlers(t)
	m := h.ipfsClient.(*mockIPFSClient)
	h.UploadHandler(httptest.NewRecorder(), uploadWithPinField(t, "true"))
	if m.localAdds.Load() != 1 || m.adds.Load() != 0 {
		t.Fatalf("pin=true: AddLocal %d, Add %d, want the pinned import", m.localAdds.Load(), m.adds.Load())
	}
}

// An upload whose pin never succeeds was imported on this node only, so its
// local pin is given back with its reference rather than left untracked.
func TestPinAsync_aPinThatNeverSucceedsGivesBackTheLocalCopy(t *testing.T) {
	h := storageHandlers(t)
	m := h.ipfsClient.(*mockIPFSClient)
	m.pinErr = errors.New("cluster refused the pin")
	h.pinAsync("bafyupload", "doc.txt", 3, "anchat")
	m.unpinMu.Lock()
	defer m.unpinMu.Unlock()
	if m.unpinCalls != 1 {
		t.Fatalf("unpinned %d times after the pin failed for good, want 1", m.unpinCalls)
	}
}
