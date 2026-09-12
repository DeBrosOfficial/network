package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DeBrosOfficial/network/pkg/client"
	"github.com/DeBrosOfficial/network/pkg/logging"
)

// acmeQueryDB records Query calls so the present/cleanup handlers can be
// exercised without a real rqlite.
type acmeQueryDB struct {
	client.DatabaseClient
	calls int
}

func (d *acmeQueryDB) Query(_ context.Context, _ string, _ ...interface{}) (*client.QueryResult, error) {
	d.calls++
	return &client.QueryResult{}, nil
}

func acmeGateway(t *testing.T, db client.DatabaseClient) *Gateway {
	t.Helper()
	log, err := logging.NewColoredLogger(logging.ComponentGateway, false)
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	return &Gateway{
		logger: log,
		client: &fakeNetworkClient{db: db},
	}
}

func acmeRequest(t *testing.T, path, remote, forwarded string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	r := httptest.NewRequest(http.MethodPost, path, &buf)
	r.RemoteAddr = remote
	if forwarded != "" {
		r.Header.Set("X-Forwarded-For", forwarded)
	}
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestACMEPresent_refusesARequestThatCameThroughCaddy(t *testing.T) {
	db := &acmeQueryDB{}
	g := acmeGateway(t, db)
	rec := httptest.NewRecorder()
	g.acmePresentHandler(rec, acmeRequest(t, "/v1/internal/acme/present",
		"127.0.0.1:54321", "198.51.100.9",
		ACMERequest{FQDN: "_acme-challenge.ns-alice.example.", Value: "token"}))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if db.calls != 0 {
		t.Fatalf("wrote %d DNS rows from the internet", db.calls)
	}
}

func TestACMECleanup_refusesARequestThatCameThroughCaddy(t *testing.T) {
	db := &acmeQueryDB{}
	g := acmeGateway(t, db)
	rec := httptest.NewRecorder()
	g.acmeCleanupHandler(rec, acmeRequest(t, "/v1/internal/acme/cleanup",
		"127.0.0.1:54321", "198.51.100.9",
		ACMERequest{FQDN: "_acme-challenge.ns-alice.example.", Value: "token"}))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if db.calls != 0 {
		t.Fatalf("deleted %d DNS rows from the internet", db.calls)
	}
}

func TestACMEPresent_acceptsCaddyOnThisHost(t *testing.T) {
	db := &acmeQueryDB{}
	g := acmeGateway(t, db)
	rec := httptest.NewRecorder()
	g.acmePresentHandler(rec, acmeRequest(t, "/v1/internal/acme/present",
		"127.0.0.1:54321", "",
		ACMERequest{FQDN: "_acme-challenge.ns-alice.example.", Value: "token-value"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if db.calls != 1 {
		t.Fatalf("Query calls = %d, want 1", db.calls)
	}
}

func TestACMECleanup_acceptsCaddyOnThisHost(t *testing.T) {
	db := &acmeQueryDB{}
	g := acmeGateway(t, db)
	rec := httptest.NewRecorder()
	g.acmeCleanupHandler(rec, acmeRequest(t, "/v1/internal/acme/cleanup",
		"127.0.0.1:54321", "",
		ACMERequest{FQDN: "_acme-challenge.ns-alice.example.", Value: "token-value"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if db.calls != 1 {
		t.Fatalf("Query calls = %d, want 1", db.calls)
	}
}
