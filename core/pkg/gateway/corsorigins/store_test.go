package corsorigins

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/core/migrations"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
	"github.com/Orama-Network/orama/core/pkg/rqlite"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// A migrated registry on real SQLite, with one namespace.
func registry(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := rqlite.ApplyEmbeddedMigrations(t.Context(), db, migrations.FS, zap.NewNop()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO namespaces(id, name) VALUES (10, 'anchat')`); err != nil {
		t.Fatal(err)
	}
	return NewStore(rqlite.NewClient(db)), db
}

func TestStore_setReadClearAndReload(t *testing.T) {
	store, _ := registry(t)
	ctx := context.Background()
	if err := store.Set(ctx, "anchat", []string{"https://app.example.com"}, "0xowner"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := store.Origins(ctx, "anchat")
	if err != nil || len(got) != 1 || got[0] != "https://app.example.com" {
		t.Fatalf("Origins = %v, %v", got, err)
	}

	cache := NewCache(store, nil)
	if cache.Allows("anchat", "https://app.example.com") {
		t.Fatal("the cache admitted an origin before it read anything")
	}
	if err := cache.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if !cache.Allows("anchat", "https://APP.example.com:443") {
		t.Error("the listed origin, in another spelling, is not admitted")
	}
	if cache.Allows("other", "https://app.example.com") {
		t.Error("another namespace admits anchat's origin")
	}

	if err := store.Set(ctx, "anchat", nil, "0xowner"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if err := cache.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if cache.Allows("anchat", "https://app.example.com") {
		t.Error("a cleared list still admits")
	}
	if err := store.Set(ctx, "nosuch", []string{"https://x.example.com"}, "0xowner"); err == nil {
		t.Error("a list was recorded for a namespace that does not exist")
	}
}

// A namespace deleted and created again under the same name is a new tenant,
// and must not admit the previous one's origins.
func TestStore_aRecreatedNamespaceStartsEmpty(t *testing.T) {
	store, db := registry(t)
	ctx := context.Background()
	if err := store.Set(ctx, "anchat", []string{"https://old-tenant.example.com"}, "0xold"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM namespaces WHERE id = 10`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO namespaces(id, name) VALUES (11, 'anchat')`); err != nil {
		t.Fatal(err)
	}
	got, err := store.Origins(ctx, "anchat")
	if err != nil || len(got) != 0 {
		t.Errorf("the re-created namespace lists %v (%v), want nothing", got, err)
	}
}

func request(method, namespace, body string) *http.Request {
	r := httptest.NewRequest(method, "/v1/namespace/cors-origins", strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), ctxkeys.NamespaceOverride, namespace))
}

func TestHandler_putValidatesStoresAndAppliesAtOnce(t *testing.T) {
	store, _ := registry(t)
	cache := NewCache(store, nil)
	h := NewHandler(store, cache, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request(http.MethodPut, "anchat", `{"origins":["https://*.example.com"]}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("wildcard: %d, want 400", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, request(http.MethodPut, "anchat", `{"origins":["https://App.example.com/","https://app.example.com"]}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Origins []string `json:"origins"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Origins) != 1 || body.Origins[0] != "https://app.example.com" {
		t.Errorf("stored %v, want the one canonical origin", body.Origins)
	}
	if !cache.Allows("anchat", "https://app.example.com") {
		t.Error("the gateway that recorded the list does not apply it until the next reload")
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, request(http.MethodGet, "anchat", ""))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "https://app.example.com") {
		t.Errorf("get: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, request(http.MethodPut, "default", `{"origins":[]}`))
	if rec.Code != http.StatusForbidden {
		t.Errorf("the lobby: %d, want 403", rec.Code)
	}
}
