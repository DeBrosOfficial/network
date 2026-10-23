//go:build e2e_fleet

package authkeysroles

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Orama-Network/orama/e2e/harness"
	"github.com/Orama-Network/orama/e2e/harness/eventually"
	"github.com/Orama-Network/orama/e2e/harness/gw"
	"github.com/Orama-Network/orama/e2e/harness/ns"
)

const (
	pathTenantQuery = "/v1/rqlite/query"
	pathCacheGet    = "/v1/cache/get"
	// requestLogBudget covers the batcher's 5 s flush and the namespace
	// RQLite's replication (docs/whitepaper/technical-reference/vol1/12-gateway-architecture.md#logging-and-metrics).
	requestLogBudget = 30 * time.Second
)

// TestKeys_requestLogNamesTheKey (bug-944): a request a key makes through the
// namespace host leaves a request_logs row on the namespace gateway naming
// the key's id, whether the key was presented bare or exchanged for a token
// (docs/whitepaper/technical-reference/vol1/12-gateway-architecture.md#logging-and-metrics).
func TestKeys_requestLogNamesTheKey(t *testing.T) {
	t.Parallel()
	n := ns.New(t, harness.Fleet(t), ns.Options{})
	bare := mintKey(t, n, map[string]any{"scope": "cache", "label": "logged-bare"})
	exchangedKey := mintKey(t, n, map[string]any{"scope": "cache", "label": "logged-exchanged"})
	exchanged, _, err := harness.GW(t).For(t).Token(t.Context(), exchangedKey.APIKey)
	if err != nil {
		t.Fatalf("exchanging a fresh key: %v", err)
	}

	body := []byte(`{"dmap":"request-log","key":"absent"}`)
	for _, bearer := range []string{bare.APIKey, exchanged.AccessToken} {
		n.Client.MustSend(t, gw.Req{Method: http.MethodPost, Path: pathCacheGet, Bearer: bearer,
			Header: http.Header{"Content-Type": {"application/json"}}, Body: body})
	}

	for name, id := range map[string]int64{"bare": bare.ID, "exchanged": exchangedKey.ID} {
		eventually.Require(t, pollEvery, requestLogBudget, "a request_logs row for the "+name+" key", func() (bool, error) {
			var out struct {
				Items []struct {
					N int64 `json:"n"`
				} `json:"items"`
			}
			resp := send(t, n.Client, http.MethodPost, pathTenantQuery, n.Owner.Token(), map[string]any{
				"sql":  "SELECT COUNT(*) AS n FROM request_logs WHERE api_key_id = ? AND path = ?",
				"args": []any{id, pathCacheGet},
			})
			if resp.Status != http.StatusOK {
				return false, eventually.Stop(fmt.Errorf("POST %s: %d %s", pathTenantQuery, resp.Status, resp.Body))
			}
			if err := resp.Decode(&out); err != nil {
				return false, eventually.Stop(err)
			}
			return len(out.Items) == 1 && out.Items[0].N > 0, nil
		})
	}
}
