package dnsdelegation

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// A delegation is checked by asking the parent zone's own nameservers, never
// the host's caching resolver (bug-930). Setup polls from the moment it prints
// the records, so its first question comes before the operator has created
// them, and a caching resolver (mDNSResponder on macOS) keeps that NXDOMAIN for
// the parent zone's negative TTL: 30 minutes on Cloudflare. Every later poll
// read the cached "no such host" while the parent zone already returned the
// records. An authoritative server keeps no negative cache, so each poll sees
// the zone as it is.
//
// The questions are non-recursive. Asked for the NS set of a delegated name,
// the parent answers with a referral: the NS records in the authority section
// and the glue in the additional section, which are exactly the records the
// operator creates (Records). Only the parent zone's own nameservers and their
// addresses come from the system resolver: they exist before setup starts, so
// no negative answer for them can be cached.

const (
	// dnsPort is where an authoritative nameserver answers.
	dnsPort = "53"
	// authoritativeQueryTimeout bounds one question to one parent nameserver.
	authoritativeQueryTimeout = 3 * time.Second
)

// exchangeFunc sends one DNS message to server (host:port) and returns the reply.
type exchangeFunc func(ctx context.Context, m *dns.Msg, server string) (*dns.Msg, error)

// parentZone answers a delegation's two questions from the authoritative
// nameservers of the zone the records go in.
type parentZone struct {
	// domain is the delegated name.
	domain string
	// servers finds the parent zone's nameservers as host:port.
	servers func(ctx context.Context, domain string) ([]string, error)
	// exchange sends one question.
	exchange exchangeFunc

	found []string
}

// newParentZone asks the real parent zone of domain over the network.
func newParentZone(domain string) *parentZone {
	return &parentZone{domain: domain, servers: parentZoneServers(net.DefaultResolver), exchange: exchangeUDPThenTCP}
}

// NS is the NS set the parent zone holds for name: a referral's authority
// section, or an answer section when the parent's servers also serve name.
func (p *parentZone) NS(ctx context.Context, name string) ([]string, error) {
	resp, err := p.ask(ctx, name, dns.TypeNS)
	if err != nil || resp == nil {
		return nil, err
	}
	var out []string
	for _, rr := range append(append([]dns.RR{}, resp.Answer...), resp.Ns...) {
		if ns, ok := rr.(*dns.NS); ok && sameName(ns.Hdr.Name, name) {
			out = append(out, strings.TrimSuffix(ns.Ns, "."))
		}
	}
	return out, nil
}

// Host is the addresses the parent zone holds for name: the glue in a
// referral's additional section, or an A record answered directly.
func (p *parentZone) Host(ctx context.Context, name string) ([]string, error) {
	resp, err := p.ask(ctx, name, dns.TypeA)
	if err != nil || resp == nil {
		return nil, err
	}
	var out []string
	for _, rr := range append(append([]dns.RR{}, resp.Answer...), resp.Extra...) {
		switch r := rr.(type) {
		case *dns.A:
			if sameName(r.Hdr.Name, name) {
				out = append(out, r.A.String())
			}
		case *dns.AAAA:
			if sameName(r.Hdr.Name, name) {
				out = append(out, r.AAAA.String())
			}
		}
	}
	return out, nil
}

// ask puts one non-recursive question to the parent zone's nameservers and
// returns the first authoritative reply. NXDOMAIN is a nil reply and no error:
// the name has no records, which is the state before the operator creates
// them. A server that does not answer, or answers SERVFAIL or REFUSED, is
// passed over for the next one of the zone's servers, as any resolver walks an
// NS set; when none answers, the error names each failure.
func (p *parentZone) ask(ctx context.Context, name string, qtype uint16) (*dns.Msg, error) {
	servers, err := p.zoneServers(ctx)
	if err != nil {
		return nil, err
	}
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	m.RecursionDesired = false
	var failures []error
	for _, server := range servers {
		resp, err := p.exchange(ctx, m, server)
		switch {
		case err != nil:
			failures = append(failures, fmt.Errorf("%s: %w", server, err))
		case resp.Rcode == dns.RcodeNameError:
			return nil, nil
		case resp.Rcode != dns.RcodeSuccess:
			failures = append(failures, fmt.Errorf("%s: %s", server, dns.RcodeToString[resp.Rcode]))
		default:
			return resp, nil
		}
	}
	return nil, fmt.Errorf("no nameserver of the parent zone of %s answered %s %s: %w",
		p.domain, dns.TypeToString[qtype], name, errors.Join(failures...))
}

// zoneServers finds the parent zone's nameservers once per check.
func (p *parentZone) zoneServers(ctx context.Context) ([]string, error) {
	if p.found != nil {
		return p.found, nil
	}
	servers, err := p.servers(ctx, p.domain)
	if err != nil {
		return nil, err
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("the parent zone of %s has no nameserver with an address", p.domain)
	}
	p.found = servers
	return servers, nil
}

// parentZoneServers finds, through r, the nameservers of the closest zone
// above domain: the first of domain's ancestors that has an NS set, starting
// at ParentZone(domain). Those names exist before any delegation record does,
// so the caching resolver has nothing negative to hold for them.
func parentZoneServers(r *net.Resolver) func(ctx context.Context, domain string) ([]string, error) {
	return func(ctx context.Context, domain string) ([]string, error) {
		for zone := ParentZone(domain); zone != ""; zone = ParentZone(zone) {
			nss, err := r.LookupNS(ctx, zone)
			var dnsErr *net.DNSError
			if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("look up the nameservers of %s: %w", zone, err)
			}
			var servers []string
			for _, ns := range nss {
				addrs, err := r.LookupIP(ctx, "ip", ns.Host)
				if err != nil {
					return nil, fmt.Errorf("look up the address of %s, a nameserver of %s: %w", ns.Host, zone, err)
				}
				for _, a := range addrs {
					servers = append(servers, net.JoinHostPort(a.String(), dnsPort))
				}
			}
			return servers, nil
		}
		return nil, fmt.Errorf("no zone above %s has nameservers", domain)
	}
}

// exchangeUDPThenTCP asks over UDP and repeats the question over TCP when the
// reply is truncated, which is how DNS carries an answer too large for UDP.
func exchangeUDPThenTCP(ctx context.Context, m *dns.Msg, server string) (*dns.Msg, error) {
	ctx, cancel := context.WithTimeout(ctx, authoritativeQueryTimeout)
	defer cancel()
	resp, _, err := (&dns.Client{Net: "udp", Timeout: authoritativeQueryTimeout}).ExchangeContext(ctx, m, server)
	if err == nil && resp.Truncated {
		resp, _, err = (&dns.Client{Net: "tcp", Timeout: authoritativeQueryTimeout}).ExchangeContext(ctx, m, server)
	}
	return resp, err
}

// sameName compares DNS names without case and trailing dot.
func sameName(a, b string) bool {
	return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
}
