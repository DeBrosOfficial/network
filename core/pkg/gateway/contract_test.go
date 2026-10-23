package gateway

import (
	"testing"

	"github.com/Orama-Network/orama/core/pkg/contracttest"
)

// The bodies the TypeScript SDK's network client sends must decode into these
// request structs with nothing left over. network.connect once sent
// {peer_addr} to a handler that reads {multiaddr}, so every call was a 400
// (bug-724). See pkg/contracttest for why.
func TestNetworkContract(t *testing.T) {
	fixtures, err := contracttest.For(".", "network/")
	if err != nil {
		t.Fatalf("load contracts: %v", err)
	}
	if len(fixtures) < 3 {
		t.Fatalf("expected the network fixtures, found %d", len(fixtures))
	}

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			switch fixture.Route {
			case "/v1/network/connect":
				var body networkConnectRequest
				if err := fixture.DecodeStrict(&body); err != nil {
					t.Fatalf("the gateway does not parse what %s sends: %v", fixture.SDK, err)
				}
				if body.Multiaddr == "" {
					t.Error("connect: a multiaddr is required")
				}
			case "/v1/network/disconnect":
				var body networkDisconnectRequest
				if err := fixture.DecodeStrict(&body); err != nil {
					t.Fatalf("the gateway does not parse what %s sends: %v", fixture.SDK, err)
				}
				if body.PeerID == "" {
					t.Error("disconnect: a peer id is required")
				}
			case "/v1/proxy/anon":
				if err := fixture.DecodeStrict(&anonProxyRequest{}); err != nil {
					t.Fatalf("the gateway does not parse what %s sends: %v", fixture.SDK, err)
				}
			default:
				t.Fatalf("fixture %s names route %s, which this test does not know", fixture.Name, fixture.Route)
			}
		})
	}
}
