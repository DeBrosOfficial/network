package validate

import (
	"strings"
	"testing"
)

func TestValidateZone(t *testing.T) {
	for _, zone := range []string{"testnet.orama.network", "orama.network", "a-b.example.com"} {
		if err := ValidateZone(zone); err != nil {
			t.Errorf("ValidateZone(%q) = %v", zone, err)
		}
	}
	for _, zone := range []string{
		"", "network", "Testnet.orama.network", "testnet.orama.network.", "203.0.113.5", "a..b",
		"-a.example.com", "a-.example.com", "a_b.example.com", "exa mple.com", strings.Repeat("a", 64) + ".com",
		strings.Repeat("a.", 130) + "com",
	} {
		if err := ValidateZone(zone); err == nil {
			t.Errorf("ValidateZone(%q) succeeded", zone)
		}
	}
}

func TestZoneServedBy(t *testing.T) {
	for zone, want := range map[string]bool{
		"testnet.orama.network":       false,
		"names.testnet.orama.network": true,
		"orama.network":               false,
		"xtestnet.orama.network":      false,
		"other.example":               false,
	} {
		if got := ZoneServedBy(zone, "testnet.orama.network"); got != want {
			t.Errorf("ZoneServedBy(%q) = %v, want %v", zone, got, want)
		}
	}
	if ZoneServedBy("testnet.orama.network", "") {
		t.Error("a cluster with no base domain serves a zone")
	}
}

func TestValidateDNS(t *testing.T) {
	const base = "testnet.orama.network"
	for name, c := range map[string]DNSConfig{
		"disabled":          {BaseDomain: base},
		"disabled, no base": {},
		"below the base":    {NodeNamesZone: "nodes." + base, BaseDomain: base},
	} {
		if errs := ValidateDNS(c); len(errs) != 0 {
			t.Errorf("%s: %v", name, errs)
		}
	}
	for name, c := range map[string]DNSConfig{
		"the base domain itself": {NodeNamesZone: base, BaseDomain: base},
		"a malformed zone":       {NodeNamesZone: "Testnet.Orama.Network", BaseDomain: base},
		"another cluster zone":   {NodeNamesZone: "devnet.orama.network", BaseDomain: base},
		"the parent of the base": {NodeNamesZone: "orama.network", BaseDomain: base},
		"no base domain":         {NodeNamesZone: base},
	} {
		errs := ValidateDNS(c)
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "dns.node_names_zone") {
			t.Errorf("%s: errs = %v, want one naming dns.node_names_zone", name, errs)
		}
	}
}
