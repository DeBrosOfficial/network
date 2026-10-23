package netguard

import "testing"

// Bug #712: the anonymity proxy judged a destination with net.ParseIP, which
// reads only the canonical dotted form, so 2130706433 and its kin passed as
// names although a resolver reads them as 127.0.0.1.
func TestReservedHost_refusesEveryNotationOfAReservedAddress(t *testing.T) {
	for name, host := range map[string]string{
		"dotted loopback":             "127.0.0.1",
		"decimal loopback":            "2130706433",
		"hex loopback":                "0x7f000001",
		"upper-case hex":              "0X7F000001",
		"hex without digits":          "0x",
		"octal loopback":              "017700000001",
		"octal first part":            "0177.0.0.1",
		"hex first part":              "0x7f.0.0.1",
		"mixed parts":                 "0x7f.0.0.01",
		"short form a.b":              "127.1",
		"short form a.b.c":            "127.0.1",
		"short hex form":              "0x7f.1",
		"all hex parts":               "0x7f.0x0.0x0.0x1",
		"leading zeros":               "127.000.000.001",
		"trailing dot":                "127.0.0.1.",
		"decimal private":             "167772161",
		"short private":               "10.1",
		"octal private":               "012.0.0.1",
		"hex metadata":                "0xa9fea9fe",
		"decimal metadata":            "2852039166",
		"CGNAT":                       "100.64.0.1",
		"decimal CGNAT":               "1681915905",
		"zero":                        "0.0.0.0",
		"bare zero":                   "0",
		"decimal zero network":        "1",
		"overlay":                     "10.0.0.1",
		"chain namespace":             "198.18.0.2",
		"broadcast":                   "4294967295",
		"ipv6 loopback":               "::1",
		"ipv6 unspecified":            "::",
		"ipv4-mapped loopback":        "::ffff:127.0.0.1",
		"ipv4-mapped hex":             "::ffff:7f00:1",
		"ipv4-compatible":             "::127.0.0.1",
		"link-local with zone":        "fe80::1%eth0",
		"unique local":                "fd00::1",
		"localhost":                   "localhost",
		"upper-case localhost":        "LOCALHOST",
		"localhost with trailing dot": "localhost.",
		"name under localhost":        "app.localhost",
	} {
		if !ReservedHost(host) {
			t.Errorf("%s: %q must be reserved", name, host)
		}
	}
}

func TestReservedHost_allowsPublicAddressesAndNames(t *testing.T) {
	for name, host := range map[string]string{
		"dotted public":            "8.8.8.8",
		"decimal public":           "134744072",
		"hex public":               "0x08080808",
		"short public":             "8.526344",
		"ipv6 public":              "2606:4700:4700::1111",
		"name":                     "example.com",
		"numeric-looking name":     "1.2.3.4.example.com",
		"name ending in localhost": "notlocalhost.com",
		"octal digit 8 is a name":  "0128.0.0.1",
		"five parts is a name":     "1.2.3.4.5",
		"part over 255 is a name":  "256.0.0.1",
		"last part too wide":       "1.2.3.256",
		"over 32 bits is a name":   "4294967296",
		"empty part is a name":     "127..1",
		"hex digit in decimal":     "12a.0.0.1",
	} {
		if ReservedHost(host) {
			t.Errorf("%s: %q must not be reserved", name, host)
		}
	}
}

func TestParseInetAton_matchesTheResolversReading(t *testing.T) {
	for in, want := range map[string]string{
		"2130706433":   "127.0.0.1",
		"0x7f000001":   "127.0.0.1",
		"017700000001": "127.0.0.1",
		"127.1":        "127.0.0.1",
		"127.0.1":      "127.0.0.1",
		"0x7f.1":       "127.0.0.1",
		"1.2.772":      "1.2.3.4",
		"1.131844":     "1.2.3.4",
		"0x01.02.03.4": "1.2.3.4",
	} {
		got, ok := parseInetAton(in)
		if !ok || got.String() != want {
			t.Errorf("parseInetAton(%q) = %v, %v; want %s", in, got, ok, want)
		}
	}
}
