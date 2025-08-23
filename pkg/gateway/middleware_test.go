package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractAPIKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer ak_foo:ns")
	if got := extractAPIKey(r); got != "ak_foo:ns" {
		t.Fatalf("got %q", got)
	}
	r.Header.Set("Authorization", "ApiKey ak2")
	if got := extractAPIKey(r); got != "ak2" {
		t.Fatalf("got %q", got)
	}
	r.Header.Set("Authorization", "ak3raw")
	if got := extractAPIKey(r); got != "ak3raw" {
		t.Fatalf("got %q", got)
	}
	r.Header = http.Header{}
	r.Header.Set("X-API-Key", "xkey")
	if got := extractAPIKey(r); got != "xkey" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateNamespaceParam(t *testing.T) {
	g := &Gateway{}
	r := httptest.NewRequest(http.MethodGet, "/v1/storage/get?namespace=ns1&key=k", nil)
	// no context namespace: should be false
	if g.validateNamespaceParam(r) {
		t.Fatalf("expected false without context ns")
	}
}
