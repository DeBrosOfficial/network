package fleet

import (
	"strings"
	"testing"

	"github.com/Orama-Network/orama/e2e/harness/config"
)

const testnetHome = "/home/o"

func testnetState() *State {
	st := &State{
		Target: config.TargetTestnet, RunID: "testnet-20260930-101500", Env: config.TestnetEnv, OperatorNamespace: config.TestnetOperatorNamespace,
		BaseDomain: config.TestnetBaseDomain, GatewayURL: config.TestnetGatewayURL, ChainID: "orama-testnet-4",
		Home:       config.TestnetPath(testnetHome, config.TestnetHomeRel),
		RWSock:     config.TestnetPath(testnetHome, config.TestnetRWSockRel),
		CAFile:     config.TestnetPath(testnetHome, config.TestnetCAFileRel),
		SSHKeyFile: config.TestnetPath(testnetHome, config.TestnetSSHKeyRel),
	}
	for _, n := range config.TestnetNodes {
		role := RoleNode
		if n.Nameserver {
			role = RoleNameserver
		}
		st.Nodes = append(st.Nodes, Node{Name: n.Name, Role: role, PublicIP: n.IP, WGIP: n.WGIP, SSHUser: n.User})
	}
	return st
}

func TestCheckState_testnetAcceptsThePinnedState(t *testing.T) {
	if err := CheckState(testnetState(), testnetHome); err != nil {
		t.Fatal(err)
	}
}

func TestCheckState_testnetRefusals(t *testing.T) {
	cases := map[string]struct {
		mutate func(*State)
		want   string
	}{
		"env devnet":         {func(s *State) { s.Env = "devnet" }, "environment"},
		"env stagenet":       {func(s *State) { s.Env = "stagenet" }, "environment"},
		"env e2e run":        {func(s *State) { s.Env = "e2e-ab12" }, "environment"},
		"devnet domain":      {func(s *State) { s.BaseDomain = "devnet.orama.network" }, "base domain"},
		"stagenet domain":    {func(s *State) { s.BaseDomain = "stagenet.orama.network" }, "base domain"},
		"e2e run domain":     {func(s *State) { s.BaseDomain = "e2e-ab12.dbrsteting.bid" }, "base domain"},
		"devnet gateway":     {func(s *State) { s.GatewayURL = "https://devnet.orama.network" }, "gateway url"},
		"gateway http":       {func(s *State) { s.GatewayURL = "http://testnet.orama.network" }, "gateway url"},
		"devnet chain":       {func(s *State) { s.ChainID = "orama-devnet-e2e-ab12" }, "chain id"},
		"stagenet chain":     {func(s *State) { s.ChainID = "orama-stagenet-1" }, "chain id"},
		"chain suffix":       {func(s *State) { s.ChainID = "orama-testnet-4-devnet-1" }, "chain id"},
		"no chain":           {func(s *State) { s.ChainID = "" }, "chain id"},
		"fleet run id":       {func(s *State) { s.RunID = "ab12cd34" }, "run id"},
		"extra node ip":      {func(s *State) { s.Nodes = append(s.Nodes, Node{Name: "node-4", PublicIP: "203.0.113.9"}) }, "public addresses"},
		"unknown node ip":    {func(s *State) { s.Nodes[2].PublicIP = "203.0.113.9" }, "public addresses"},
		"missing node":       {func(s *State) { s.Nodes = s.Nodes[:2] }, "public addresses"},
		"duplicate node":     {func(s *State) { s.Nodes[2].PublicIP = s.Nodes[1].PublicIP }, "public addresses"},
		"extra server":       {func(s *State) { s.Extras = []Node{{Name: "extra-1", PublicIP: config.TestnetNodes[0].IP}} }, "extras or probes"},
		"probe":              {func(s *State) { s.Probes = []Node{{Name: "probe-1", PublicIP: config.TestnetNodes[0].IP}} }, "extras or probes"},
		"real wallet socket": {func(s *State) { s.RWSock = "/home/o/.rootwallet/agent.sock" }, "real"},
		"other socket":       {func(s *State) { s.RWSock = "/home/o/other/agent.sock" }, "agent socket"},
		"no socket":          {func(s *State) { s.RWSock = "" }, "RW_AGENT_SOCK"},
		"real home as HOME":  {func(s *State) { s.Home = testnetHome }, "CLI HOME"},
		"fleet agent HOME":   {func(s *State) { s.Home, s.RWSock = "/tmp/e2e-rw-test0001", "/tmp/e2e-rw-test0001/a.sock" }, "CLI HOME"},
		"other CA":           {func(s *State) { s.CAFile = "/tmp/ca.pem" }, "CA bundle"},
		"other ssh key":      {func(s *State) { s.SSHKeyFile = "/home/o/.ssh/id_ed25519" }, "SSH key"},
		"unknown target":     {func(s *State) { s.Target = "devnet" }, "target"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			st := testnetState()
			c.mutate(st)
			err := CheckState(st, testnetHome)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestCheckState_testnetTargetRefusesFleetValues(t *testing.T) {
	st := goodState()
	st.Target = config.TargetTestnet
	err := CheckState(st, testnetHome)
	if err == nil {
		t.Fatal("the target testnet accepted a fleet state")
	}
	for _, want := range []string{"environment", "base domain", "chain id", "run id"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not report the %s: %v", want, err)
		}
	}
}

func TestCheckState_fleetTargetRefusesTestnetValues(t *testing.T) {
	st := testnetState()
	st.Target = config.TargetFleet
	if err := CheckState(st, testnetHome); err == nil {
		t.Fatal("the default target accepted the testnet state")
	}
}

func TestRunIDShape_acceptsTestnetRunIDs(t *testing.T) {
	for id, want := range map[string]bool{
		"testnet-20260930-101500": true, "ab12cd34": true, "testnet-": false, "testnet-2026": false, "Testnet-20260930-101500": false,
	} {
		if got := runIDShape(id); got != want {
			t.Errorf("runIDShape(%q) = %v, want %v", id, got, want)
		}
	}
}
