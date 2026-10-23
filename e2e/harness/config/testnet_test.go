package config

import (
	"strings"
	"testing"
)

func goodPins() TestnetPins {
	return TestnetPins{RunID: "testnet-20260930-101500", Env: TestnetEnv, BaseDomain: TestnetBaseDomain, OperatorNamespace: TestnetOperatorNamespace,
		GatewayURL: TestnetGatewayURL, ChainID: "orama-testnet-4", NodeIPs: TestnetIPs()}
}

func TestCheckTestnet_acceptsThePinnedValues(t *testing.T) {
	if err := CheckTestnet(goodPins()); err != nil {
		t.Fatal(err)
	}
}

func TestCheckTestnet_reportsEveryMismatch(t *testing.T) {
	p := goodPins()
	p.Env, p.ChainID, p.NodeIPs = "devnet", "orama-devnet-1", nil
	err := CheckTestnet(p)
	if err == nil {
		t.Fatal("accepted")
	}
	for _, want := range []string{"environment", "chain id", "public addresses"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q: %v", want, err)
		}
	}
}

func TestCheckTestnetChainID_shape(t *testing.T) {
	for id, ok := range map[string]bool{
		"orama-testnet-4": true, "orama-testnet-12": true, "orama-testnet-": false, "orama-testnet-4x": false,
		"xorama-testnet-4": false, "orama-devnet-4": false, "": false, "orama-testnet-4\n": false,
	} {
		if err := CheckTestnetChainID(id); (err == nil) != ok {
			t.Errorf("CheckTestnetChainID(%q) = %v, want ok=%v", id, err, ok)
		}
	}
}

func TestCheckTarget(t *testing.T) {
	for target, ok := range map[string]bool{TargetFleet: true, TargetTestnet: true, "devnet": false, "stagenet": false, "Testnet": false} {
		if err := CheckTarget(target); (err == nil) != ok {
			t.Errorf("CheckTarget(%q) = %v, want ok=%v", target, err, ok)
		}
	}
}

func TestTestnetPins_neverNameASharedEnvironment(t *testing.T) {
	for _, v := range []string{TestnetEnv, TestnetBaseDomain, TestnetGatewayURL} {
		if strings.Contains(v, "devnet") || strings.Contains(v, "stagenet") {
			t.Errorf("%q names a shared environment", v)
		}
	}
	if len(TestnetNodes) != 5 || len(TestnetIPs()) != 5 {
		t.Fatalf("want 5 testnet nodes, have %d", len(TestnetNodes))
	}
}
