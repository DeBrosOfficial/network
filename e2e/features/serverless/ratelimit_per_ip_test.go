//go:build e2e_fleet

package serverless

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Orama-Network/orama/e2e/harness/eventually"
	"github.com/Orama-Network/orama/e2e/harness/gw"
)

// perIPBudget is how long the runner waits for a namespace's per-IP limits to
// reach every gateway: the session policy cache's 5 s bound, plus room for
// the calls each attempt sends.
const perIPBudget = 30 * time.Second

// TestInvoke_perIPAuthenticatedCeiling: a namespace that turns its per-IP
// limits on holds authenticated callers from one network to its invoke
// ceiling whichever wallet signs, 429 RATE_LIMITED scope=per_ip_authenticated
// with Retry-After (website/src/docs/developer/functions.mdx#invoke-rate-limits).
// Six a minute is a burst of one per gateway and a token every ten seconds,
// so a quick run of calls from two wallets is refused on some gateway.
func TestInvoke_perIPAuthenticatedCeiling(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	deploy(t, fx, fnSpec{name: "e2e-perip"})
	fx.n.CLI.MustOK(t, "namespace", "session-policy", "--per-ip-limits", "on", "--invokes-per-ip-minute", "6")

	wallets := []string{fx.admin, fx.runtime}
	var refused *gw.Response
	eventually.Require(t, pollEvery, perIPBudget, "a call past the network's invoke ceiling", func() (bool, error) {
		for i := 0; i < 2*gatewayBound(fx)+1; i++ {
			r := invoke(t, fx.c, "e2e-perip", wallets[i%len(wallets)], map[string]any{"op": "echo"})
			switch r.Status {
			case http.StatusOK:
			case http.StatusTooManyRequests:
				refused = r
				return true, nil
			default:
				return false, fmt.Errorf("call %d: want 200 or 429, got %d %.300s", i+1, r.Status, r.Body)
			}
		}
		return false, nil
	})
	checkRefusal(t, refused, "per_ip_authenticated", 10*time.Second)
}
