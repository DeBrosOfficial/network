package oramacli

import (
	"context"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/e2e/harness/config"
)

// The testnet target signs through the dev agent, whose socket is in the
// owner's home; the real ~/.rootwallet stays refused on every target.
func TestCheck_testnetSocketInRealHome(t *testing.T) {
	home := "/nonexistent/real-home"
	cases := map[string]struct {
		target string
		sock   string
		ok     bool
	}{
		"fleet target refuses a socket in the home":   {config.TargetFleet, home + "/rwdev/agent.sock", false},
		"testnet target accepts the dev agent":        {config.TargetTestnet, home + "/rwdev/agent.sock", true},
		"testnet target refuses the real wallet":      {config.TargetTestnet, home + "/.rootwallet/agent.sock", false},
		"testnet target refuses an empty socket":      {config.TargetTestnet, "", false},
		"testnet target refuses a relative socket":    {config.TargetTestnet, "rwdev/agent.sock", false},
		"testnet target refuses the wallet's subpath": {config.TargetTestnet, home + "/.rootwallet/x/agent.sock", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r, _ := newRunner(t)
			r.Target, r.AgentSock = c.target, c.sock
			_, err := r.Run(context.Background(), "version")
			if c.ok && err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !c.ok && (err == nil || !strings.Contains(err.Error(), "refusing to run orama")) {
				t.Fatalf("err %v, want a refusal", err)
			}
		})
	}
}
