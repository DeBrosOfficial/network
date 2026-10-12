package pool

import (
	"testing"

	"cosmossdk.io/math"
)

func requireGrants(t *testing.T, got []Grant, want ...string) {
	t.Helper()
	have := make([]string, len(got))
	for i, g := range got {
		have[i] = g.Amount.String()
	}
	if len(have) != len(want) {
		t.Fatalf("grants %v, want %v", have, want)
	}
	for i := range want {
		if have[i] != want[i] {
			t.Fatalf("grants %v, want %v", have, want)
		}
	}
}

func req(address string, amount int64) Request {
	return Request{Address: address, Amount: math.NewInt(amount)}
}

func TestServe_capsTheWhale(t *testing.T) {
	got := Serve(math.NewInt(100), math.NewInt(40), []Request{req("small", 10), req("whale", 90)}, nil)
	requireGrants(t, got, "10", "40")
}

func TestServe_paysInFullWhenThereIsRoom(t *testing.T) {
	got := Serve(math.NewInt(100), math.NewInt(100), []Request{req("a", 25), req("b", 75)}, nil)
	requireGrants(t, got, "25", "75")
}

func TestServe_servesOldestFirstUntilTheCapacityIsGone(t *testing.T) {
	got := Serve(math.NewInt(50), math.NewInt(1000), []Request{req("a", 40), req("b", 40), req("c", 40)}, nil)
	requireGrants(t, got, "40", "10", "0")
}

func TestServe_aSmallRequestIsNotStarvedByALargerOneBehindIt(t *testing.T) {
	got := Serve(math.NewInt(50), math.NewInt(1000), []Request{req("small", 10), req("whale", 900)}, nil)
	requireGrants(t, got, "10", "40")
}

func TestServe_perAddressCapBindsAcrossAnAddressesRequests(t *testing.T) {
	got := Serve(math.NewInt(100), math.NewInt(30), []Request{req("w", 20), req("w", 20), req("o", 20)}, nil)
	requireGrants(t, got, "20", "10", "20")
}

func TestServe_aPartPaymentTheTargetRefusesIsSkippedAndKeepsTheCapacity(t *testing.T) {
	refuse := func(_ int, grant math.Int) bool { return grant.GTE(math.NewInt(10)) }
	got := Serve(math.NewInt(50), math.NewInt(1000), []Request{req("a", 45), req("b", 40), req("c", 5)}, refuse)
	// a is paid in full; b could only be paid 5, a part below 10, and is skipped; c is paid in full.
	requireGrants(t, got, "45", "0", "5")
}

func TestServe_aFullPaymentIsIssuedEvenIfAcceptWouldRefuseIt(t *testing.T) {
	refuse := func(int, math.Int) bool { return false }
	got := Serve(math.NewInt(50), math.NewInt(1000), []Request{req("a", 7)}, refuse)
	requireGrants(t, got, "7")
}

func TestServe_emptyCapacityAndBadRequests(t *testing.T) {
	requireGrants(t, Serve(math.ZeroInt(), math.NewInt(10), []Request{req("a", 5)}, nil), "0")
	requireGrants(t, Serve(math.NewInt(10), math.ZeroInt(), []Request{req("a", 5)}, nil), "0")
	requireGrants(t, Serve(math.NewInt(10), math.NewInt(10), []Request{req("a", 0), {Address: "b"}, req("c", -3)}, nil), "0", "0", "0")
	if got := Serve(math.NewInt(10), math.NewInt(10), nil, nil); len(got) != 0 {
		t.Fatalf("no requests: %v", got)
	}
}
