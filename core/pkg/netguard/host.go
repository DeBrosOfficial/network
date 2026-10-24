package netguard

import (
	"net/netip"
	"strings"
)

// ReservedHost reports whether a destination host, as a caller wrote it (no
// port, no brackets), names a reserved address or the local machine.
//
// It is for destinations the gateway hands on UNRESOLVED, to the Tor SOCKS
// port: the exit's resolver reads the text, so the text is judged the way a
// resolver reads it, not the way net.ParseIP does. A resolver built on
// inet_aton takes 2130706433, 0x7f000001, 0177.0.0.1, 127.1 and 0x7f.1 as
// 127.0.0.1, and every one of them is refused here exactly like the dotted
// form. localhost and every name under it are refused too (RFC 6761 6.3:
// resolvers answer them with loopback). A name that is not an address is not
// reserved here; it is resolved, and refused if internal, at the exit.
func ReservedHost(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	addr, ok := parseHostAddr(h)
	return ok && Reserved(addr.AsSlice())
}

// parseHostAddr reads host as an IP address in every notation a resolver
// accepts: an IPv6 literal (its zone dropped: the zone selects an interface,
// not a different address) or an IPv4 address in inet_aton's forms.
func parseHostAddr(host string) (netip.Addr, bool) {
	if strings.Contains(host, ":") {
		addr, err := netip.ParseAddr(host)
		if err != nil {
			return netip.Addr{}, false
		}
		return addr.WithZone(""), true
	}
	return parseInetAton(host)
}

// parseInetAton parses s with the rules of BSD and glibc inet_aton: one to
// four parts separated by dots, each decimal, octal (a leading 0) or
// hexadecimal (a leading 0x, whose digits may be absent and then read as 0);
// the last part fills every byte the parts before it left, so a.b.c.d has
// four 8-bit parts, a.b.c a 16-bit last part, a.b a 24-bit one and a alone
// is all 32 bits.
func parseInetAton(s string) (netip.Addr, bool) {
	parts := strings.Split(s, ".")
	if len(parts) > net4Parts {
		return netip.Addr{}, false
	}
	vals := make([]uint64, len(parts))
	for i, p := range parts {
		v, ok := parseInetAtonPart(p)
		if !ok {
			return netip.Addr{}, false
		}
		vals[i] = v
	}
	last := len(vals) - 1
	var n uint64
	for _, v := range vals[:last] {
		if v > 0xff {
			return netip.Addr{}, false
		}
		n = n<<8 | v
	}
	lastBits := 8 * uint(net4Parts-last)
	if vals[last] >= 1<<lastBits {
		return netip.Addr{}, false
	}
	n = n<<lastBits | vals[last]
	return netip.AddrFrom4([4]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}), true
}

// net4Parts is the most parts an IPv4 address has in inet_aton's notation.
const net4Parts = 4

// parseInetAtonPart reads one part. Values are capped above 32 bits, which
// every caller refuses, so a long run of digits cannot overflow.
func parseInetAtonPart(p string) (uint64, bool) {
	if p == "" {
		return 0, false
	}
	base, digits := uint64(10), p
	switch {
	case len(p) >= 2 && p[0] == '0' && (p[1] == 'x' || p[1] == 'X'):
		base, digits = 16, p[2:]
	case len(p) >= 2 && p[0] == '0':
		base, digits = 8, p[1:]
	}
	var v uint64
	for i := 0; i < len(digits); i++ {
		d, ok := digitValue(digits[i])
		if !ok || d >= base {
			return 0, false
		}
		v = v*base + d
		if v > 0xffffffff {
			return 0, false
		}
	}
	return v, true
}

func digitValue(c byte) (uint64, bool) {
	switch {
	case c >= '0' && c <= '9':
		return uint64(c - '0'), true
	case c >= 'a' && c <= 'f':
		return uint64(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return uint64(c-'A') + 10, true
	}
	return 0, false
}
