package olric

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func addrs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("10.0.0.%d:10102", i+1)
	}
	return out
}

// viewsOf gives every node the member set of its side.
func viewsOf(sides [][]string) map[string][]string {
	v := map[string][]string{}
	for _, side := range sides {
		for _, n := range side {
			v[n] = side
		}
	}
	return v
}

// The stagenet split of bug-933: three, one and one. Only the lower of the
// two singletons restarts; the three stay.
func TestDecideHeal_theStagenetSplit(t *testing.T) {
	n := addrs(5)
	views := viewsOf([][]string{{n[0], n[1], n[4]}, {n[2]}, {n[3]}})
	for _, self := range n {
		d := DecideHeal(self, n, views)
		want := HealWait
		if self == n[2] {
			want = HealRestart
		}
		if d.Action != want {
			t.Errorf("%s: %v (%s), want %v", self, d.Action, d.Reason, want)
		}
	}
}

func TestDecideHeal_wholeRingDoesNothing(t *testing.T) {
	n := addrs(3)
	if d := DecideHeal(n[0], n, viewsOf([][]string{n})); d.Action != HealNone {
		t.Fatalf("%v (%s)", d.Action, d.Reason)
	}
}

// A node that does not answer is a down node or an unhealed network: nobody
// restarts into that.
func TestDecideHeal_anUnreachableNodeMeansWait(t *testing.T) {
	n := addrs(3)
	views := map[string][]string{n[0]: {n[0]}, n[1]: {n[1]}} // n[2] did not answer
	for _, self := range n[:2] {
		if d := DecideHeal(self, n, views); d.Action != HealWait {
			t.Errorf("%s: %v (%s), want wait", self, d.Action, d.Reason)
		}
	}
}

// Equal sides: the side whose lowest member sorts first stays.
func TestDecideHeal_aTieKeepsTheSideThatSortsFirst(t *testing.T) {
	n := addrs(4)
	views := viewsOf([][]string{{n[0], n[1]}, {n[2], n[3]}})
	if d := DecideHeal(n[2], n, views); d.Action != HealRestart {
		t.Fatalf("n3: %v (%s), want restart", d.Action, d.Reason)
	}
	for _, self := range []string{n[0], n[1], n[3]} {
		if d := DecideHeal(self, n, views); d.Action != HealWait {
			t.Errorf("%s: %v, want wait", self, d.Action)
		}
	}
}

// Never two at once: for any partition of any cluster size, with every node
// deciding from the same views, exactly one node restarts, and it is not on
// the side that stays; a whole ring restarts nobody.
func TestDecideHeal_exactlyOneNodeRestartsForAnyPartition(t *testing.T) {
	rng := rand.New(rand.NewSource(933))
	for trial := 0; trial < 5000; trial++ {
		n := addrs(1 + rng.Intn(12))
		rng.Shuffle(len(n), func(i, j int) { n[i], n[j] = n[j], n[i] })
		var sides [][]string
		for len(n) > 0 {
			k := 1 + rng.Intn(len(n))
			sides = append(sides, n[:k])
			n = n[k:]
		}
		all := addrs(0)
		for _, s := range sides {
			all = append(all, s...)
		}
		views := viewsOf(sides)
		restarts := 0
		for _, self := range all {
			if DecideHeal(self, all, views).Action == HealRestart {
				restarts++
			}
		}
		want := 1
		if len(sides) == 1 {
			want = 0
		}
		if restarts != want {
			t.Fatalf("trial %d, sides %v: %d nodes restart, want %d", trial, sides, restarts, want)
		}
	}
}

// The hold-down: two consecutive restart decisions; then the backoff, which
// doubles to an hour and resets once the ring is whole.
func TestHealer_holdDownAndBackoff(t *testing.T) {
	restart := HealDecision{Action: HealRestart}
	var h Healer
	now := time.Unix(1_700_000_000, 0)
	if h.Observe(restart, now) {
		t.Fatal("restarted on the first decision")
	}
	if !h.Observe(restart, now.Add(HealCheckInterval)) {
		t.Fatal("did not restart on the second consecutive decision")
	}
	now = now.Add(HealCheckInterval)
	for i := 1; i <= 4; i++ {
		backoff := min(healMinBackoff<<(i-1), healMaxBackoff)
		if h.Observe(restart, now.Add(backoff-time.Second)) {
			t.Fatalf("restart %d: restarted inside a backoff of %s", i, backoff)
		}
		now = now.Add(backoff)
		if !h.Observe(restart, now) {
			t.Fatalf("restart %d: did not restart once the backoff of %s ended", i, backoff)
		}
	}
	if h.backoff != healMaxBackoff {
		t.Fatalf("backoff %s after five restarts", h.backoff)
	}
	h.Observe(HealDecision{Action: HealNone}, now)
	if h.backoff != 0 || h.confirmed != 0 {
		t.Fatalf("a whole ring did not reset the pacing: %+v", h)
	}
	if h.Observe(restart, now.Add(time.Hour)) {
		t.Fatal("a wait or a whole ring must break the hold-down")
	}
}

func TestHealState_recordAndRead(t *testing.T) {
	dir := t.TempDir()
	if st, err := ReadHealState(dir); err != nil || st.Restarts != 0 {
		t.Fatalf("no file: %+v %v", st, err)
	}
	at := time.Unix(1_700_000_000, 0)
	for i := 0; i < 2; i++ {
		if err := RecordHealRestart(dir, "split", at); err != nil {
			t.Fatal(err)
		}
	}
	st, err := ReadHealState(dir)
	if err != nil || st.Restarts != 2 || st.LastReason != "split" || !st.LastRestart.Equal(at) {
		t.Fatalf("%+v %v", st, err)
	}
}
