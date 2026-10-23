package dnsdelegation

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/miekg/dns"
)

// fakeParent is the parent zone's authoritative nameserver: before
// delegate() it answers NXDOMAIN for everything under the domain, after it a
// referral carrying the NS set and the glue.
type fakeParent struct {
	mu        sync.Mutex
	delegated bool
	queries   int
	d         Delegation
}

func (f *fakeParent) delegate() { f.mu.Lock(); f.delegated = true; f.mu.Unlock() }

func (f *fakeParent) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries++
	resp := new(dns.Msg)
	resp.SetReply(req)
	if req.RecursionDesired {
		resp.Rcode = dns.RcodeRefused // an authoritative server does not recurse
		_ = w.WriteMsg(resp)
		return
	}
	if !f.delegated {
		resp.Rcode = dns.RcodeNameError
		_ = w.WriteMsg(resp)
		return
	}
	apex := dns.Fqdn(f.d.Domain)
	for _, ns := range f.d.Nameservers {
		target := ns.Hostname + "." + apex
		resp.Ns = append(resp.Ns, &dns.NS{Hdr: dns.RR_Header{Name: apex, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300}, Ns: target})
		resp.Extra = append(resp.Extra, &dns.A{Hdr: dns.RR_Header{Name: target, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: net.ParseIP(ns.IP)})
	}
	_ = w.WriteMsg(resp)
}

// checkAgainst runs Check with the default lookups pointed at server as the
// parent zone's only nameserver.
func checkAgainst(t *testing.T, d Delegation, server string) []Finding {
	t.Helper()
	parent := &parentZone{
		domain:   d.Domain,
		servers:  func(context.Context, string) ([]string, error) { return []string{server}, nil },
		exchange: exchangeUDPThenTCP,
	}
	findings, err := Check(context.Background(), d, Lookups{NS: parent.NS, Host: parent.Host})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return findings
}

// bug-930: the first poll comes before the records exist and is answered
// NXDOMAIN. The next poll after the operator creates them must see them: the
// check asks the parent zone's server, non-recursively, every time, so no
// negative cache sits between the record and setup.
func TestCheck_aRecordCreatedAfterAnNXDOMAINIsSeenOnTheNextPoll(t *testing.T) {
	f := &fakeParent{d: twoSlots}
	server := startResolver(t, f)

	if findings := checkAgainst(t, twoSlots, server); len(findings) != 2*len(twoSlots.Nameservers) {
		t.Fatalf("before the records exist: %v, want every NS and glue missing", findings)
	}
	f.delegate()
	if findings := checkAgainst(t, twoSlots, server); len(findings) != 0 {
		t.Fatalf("the poll after the records were created: %v, want none", findings)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.queries < 2*(1+len(twoSlots.Nameservers)) {
		t.Errorf("the parent was asked %d times over two polls; every poll must ask it", f.queries)
	}
}

// Glue at the wrong address is a finding naming what the parent holds.
func TestCheck_authoritativeWrongGlue(t *testing.T) {
	f := &fakeParent{d: Delegation{Domain: twoSlots.Domain, Nameservers: []Nameserver{
		{Hostname: "ns1", IP: "203.0.113.5"}, {Hostname: "ns2", IP: "198.51.100.9"},
	}}}
	f.delegate()
	findings := checkAgainst(t, twoSlots, startResolver(t, f))
	if len(findings) != 1 || findings[0].Kind != FindingWrongGlue || findings[0].Got[0] != "198.51.100.9" {
		t.Fatalf("findings %v, want ns2's glue wrong at 198.51.100.9", findings)
	}
}

// A parent that answers nothing usable is an error, not a finding: it says
// nothing about the records.
func TestParentZone_noUsableAnswerIsAnError(t *testing.T) {
	refusing := &parentZone{
		domain:  twoSlots.Domain,
		servers: func(context.Context, string) ([]string, error) { return []string{"a:53", "b:53"}, nil },
		exchange: func(_ context.Context, m *dns.Msg, _ string) (*dns.Msg, error) {
			resp := new(dns.Msg)
			resp.SetRcode(m, dns.RcodeServerFailure)
			return resp, nil
		},
	}
	if _, err := Check(context.Background(), twoSlots, Lookups{NS: refusing.NS, Host: refusing.Host}); err == nil {
		t.Fatal("every parent nameserver answered SERVFAIL and the check reported findings")
	}
}

// The parent zone is the closest ancestor with an NS set: stage.example.test
// goes in example.test even when a resolver knows no zone at the first step.
func TestParentZoneServers_walkUpToTheEnclosingZone(t *testing.T) {
	f := &zoneServer{zones: map[string][]string{"example.test.": {"ns.example.test."}}, hosts: map[string]string{"ns.example.test.": "192.0.2.53"}}
	resolverAddr := startResolver(t, f)
	r := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", resolverAddr)
	}}
	servers, err := parentZoneServers(r)(context.Background(), "x.stage.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0] != "192.0.2.53:53" {
		t.Fatalf("servers %v, want example.test's nameserver", servers)
	}
}

// zoneServer is a recursive resolver that knows only the given zones' NS sets
// and the nameservers' addresses.
type zoneServer struct {
	zones map[string][]string
	hosts map[string]string
}

func (z *zoneServer) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	resp := new(dns.Msg)
	resp.SetReply(req)
	q := req.Question[0]
	name := dns.CanonicalName(q.Name)
	switch {
	case q.Qtype == dns.TypeNS && z.zones[name] != nil:
		for _, ns := range z.zones[name] {
			resp.Answer = append(resp.Answer, &dns.NS{Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 60}, Ns: ns})
		}
	case q.Qtype == dns.TypeA && z.hosts[name] != "":
		resp.Answer = append(resp.Answer, &dns.A{Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP(z.hosts[name])})
	case q.Qtype == dns.TypeAAAA && z.hosts[name] != "":
	default:
		resp.Rcode = dns.RcodeNameError
	}
	_ = w.WriteMsg(resp)
}

// startResolver serves h on a loopback UDP port and returns host:port.
func startResolver(t *testing.T, h dns.Handler) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &dns.Server{PacketConn: pc, Handler: h}
	started := make(chan struct{})
	srv.NotifyStartedFunc = func() { close(started) }
	go func() { _ = srv.ActivateAndServe() }()
	<-started
	t.Cleanup(func() { _ = srv.Shutdown() })
	return pc.LocalAddr().String()
}
