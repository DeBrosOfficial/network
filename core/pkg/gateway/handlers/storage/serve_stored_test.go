package storage

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Both download routes send the content with its Content-Length, which is what
// lets a client reading a stream tell a body cut short from a whole one.
func TestServeStored_sendsTheContentWithItsLength(t *testing.T) {
	mock := &mockIPFSClient{getReader: sizedBody("file contents"), heldLocally: true}
	h, gate := fetchHandlers(t, mock, &countingDB{ownershipDB: ownershipDB{owned: true}})
	caps := mintTokens(t, gate, testCID, "dev-1", 1)

	for name, serve := range map[string]func() *httptest.ResponseRecorder{
		"relayed": func() *httptest.ResponseRecorder { return serveRelayed(h, relayedGET(testCID, caps[0].Token)) },
		"get": func() *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			h.DownloadHandler(rec, withNamespace(httptest.NewRequest(http.MethodGet, "/v1/storage/get/"+testCID, nil), fetchTestNS))
			return rec
		},
	} {
		mock.getReader = sizedBody("file contents")
		rec := serve()
		if rec.Code != http.StatusOK || rec.Body.String() != "file contents" {
			t.Fatalf("%s: status %d body %q", name, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Content-Length") != "13" {
			t.Errorf("%s: Content-Length = %q, want 13", name, rec.Header().Get("Content-Length"))
		}
	}
}
