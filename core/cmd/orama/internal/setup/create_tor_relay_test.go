package setup

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// A creation's relays are installed by its Tor step, after the on-chain step, so a relay has no
// identity when its node is registered (stagenet chain 11 stopped there). The node is registered
// with the relay role and without the relay's key; once the consensus lists the relays, each
// relay's key is bound to its node and the relay is registered, and a run again sends none of it.
func TestRunCreate_tor_relaysAreRegisteredOnTheChainOnceTheTorStepStartedThem(t *testing.T) {
	h := newCreateHarness(t)
	h.mustCreate(t, h.torOpts(3, torIPs...))
	all := h.w.entries()
	names := NodeNames(DefaultCreateNodeName, 5)
	for i, ip := range torIPs[3:] {
		name := names[3+i]
		reg := h.w.index("tx register-node " + name + " ")
		start := h.w.index("tor-start " + ip)
		bind := h.w.index("tx update-bindings " + name + " ")
		relay := h.w.index(fmt.Sprintf("tx register-relay %s fp=%s exit=false", name, fakeRelayHex(ip)))
		if reg < 0 || strings.HasSuffix(all[reg], ",relay") {
			t.Errorf("%s registers without the relay's key, which does not exist yet: %q", name, entryAt(all, reg))
		}
		if start < 0 || bind < start || relay < bind {
			t.Errorf("%s: the relay key is bound (%d) and the relay registered (%d) after its Tor starts (%d):\n%s", name, bind, relay, start, strings.Join(all, "\n"))
		}
		if got := serviceList(h.w.nodes[name].Bindings); !strings.HasSuffix(got, ",relay") {
			t.Errorf("%s ends with bindings %s, want the relay's key among them", name, got)
		}
		if h.w.relays[name] == nil {
			t.Errorf("the chain holds no relay for %s", name)
		}
	}
	for _, name := range names[:3] {
		if h.w.index("tx register-relay "+name+" ") >= 0 {
			t.Errorf("the authority %s registered a relay", name)
		}
	}

	h.finished(torIPs, h.seats.seats[torIPs[0]].genesis)
	mark := len(h.w.entries())
	h.mustCreate(t, h.torOpts(3, torIPs...))
	for _, e := range h.w.entries()[mark:] {
		if strings.HasPrefix(e, "tx register-relay") || strings.HasPrefix(e, "tx update-relay") || strings.HasPrefix(e, "tx update-bindings") {
			t.Errorf("a run again sent %q", e)
		}
	}
}

func entryAt(entries []string, i int) string {
	if i < 0 || i >= len(entries) {
		return "<none>"
	}
	return entries[i]
}

// An exit relay is registered with x/relay as an exit; an authority is no relay and registers none.
func TestRunCreate_tor_exitRelaysAreRegisteredOnTheChainAsExits(t *testing.T) {
	h := newCreateHarness(t)
	opts := h.torOpts(3, torIPs...)
	opts.Exit = true
	h.mustCreate(t, opts)
	all := h.w.entries()
	names := NodeNames(DefaultCreateNodeName, 5)
	for i, ip := range torIPs[3:] {
		want := fmt.Sprintf("tx register-relay %s fp=%s exit=true", names[3+i], fakeRelayHex(ip))
		if !slices.Contains(all, want) {
			t.Errorf("no %q:\n%s", want, strings.Join(all, "\n"))
		}
		if r := h.w.relays[names[3+i]]; r == nil || !r.Exit {
			t.Errorf("the chain holds %+v for %s, want an exit relay", r, names[3+i])
		}
	}
	if n := countPrefix(all, "tx register-relay"); n != 2 {
		t.Errorf("%d relays registered, want the 2 relay machines only:\n%s", n, strings.Join(all, "\n"))
	}
}

// A run that stopped at the second relay's registration has the first relay on the chain; the
// run again registers the second and sends nothing for the first.
func TestRunCreate_tor_aPartialRelayRegistrationResumesWithTheRelaysNotYetRegistered(t *testing.T) {
	h := newCreateHarness(t)
	names := NodeNames(DefaultCreateNodeName, 5)
	h.w.registerRelayErr = map[string]error{names[4]: errBoom}
	if _, err := Run(context.Background(), h.torOpts(3, torIPs...), h.deps); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want the second relay's registration failure", err)
	}
	if h.w.relays[names[3]] == nil || h.w.relays[names[4]] != nil {
		t.Fatalf("relays held after the failure: %v, want the first only", h.w.relays)
	}

	h.w.registerRelayErr = nil
	h.finished(torIPs, h.seats.seats[torIPs[0]].genesis)
	mark := len(h.w.entries())
	h.mustCreate(t, h.torOpts(3, torIPs...))
	var sent []string
	for _, e := range h.w.entries()[mark:] {
		if strings.HasPrefix(e, "tx register-relay") || strings.HasPrefix(e, "tx update-relay") {
			sent = append(sent, e)
		}
	}
	if want := fmt.Sprintf("tx register-relay %s fp=%s exit=false", names[4], fakeRelayHex(torIPs[4])); len(sent) != 1 || sent[0] != want {
		t.Errorf("the run again sent %q, want only %q", sent, want)
	}
	if h.w.relays[names[3]] == nil || h.w.relays[names[4]] == nil {
		t.Errorf("relays held after the run again: %v, want both", h.w.relays)
	}
}

// The relays are registered before the nodes leave the bootstrap mode, so a registration that
// fails stops the run there, with the machine named.
func TestRunCreate_tor_aRelayRegistrationFailureStopsTheRunBeforeTheBootstrapIsLeft(t *testing.T) {
	h := newCreateHarness(t)
	names := NodeNames(DefaultCreateNodeName, 5)
	h.w.registerRelayErr = map[string]error{names[3]: errBoom}
	_, err := Run(context.Background(), h.torOpts(3, torIPs...), h.deps)
	if !errors.Is(err, errBoom) || !strings.Contains(err.Error(), "machine "+torIPs[3]) {
		t.Fatalf("err = %v, want the failure naming machine %s", err, torIPs[3])
	}
	entries := h.w.entries()
	if i := slices.IndexFunc(entries, func(e string) bool {
		return strings.HasPrefix(e, "tor-install ") && strings.Contains(e, "bootstrap=false")
	}); i >= 0 {
		t.Errorf("the bootstrap mode was left after the failure: %q", entries[i])
	}
	if n := countPrefix(torLog(h.w, 0), "tor-restart "); n != 0 {
		t.Errorf("%d restarts after the failure", n)
	}
	if h.w.index("tor-start "+torIPs[4]) < 0 {
		t.Errorf("the relays had started before the registration:\n%s", strings.Join(entries, "\n"))
	}
}
