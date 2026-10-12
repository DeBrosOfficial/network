package pool

import (
	"testing"
	"time"

	"cosmossdk.io/math"
)

func TestTurnstileUnderflowAndMigration(t *testing.T) {
	p := NewPools()
	from := Key{Vintage: 1, Asset: NativeAsset}
	to := Key{Vintage: 2, Asset: NativeAsset}
	if err := p.Add(from, math.NewInt(100)); err != nil {
		t.Fatal(err)
	}
	if err := p.Sub(from, math.NewInt(101)); err != ErrUnderflow {
		t.Fatalf("underflow: %v", err)
	}
	if err := p.Move(from, to, math.NewInt(40)); err != nil {
		t.Fatal(err)
	}
	if !p.Balance(from).Equal(math.NewInt(60)) || !p.Balance(to).Equal(math.NewInt(40)) {
		t.Fatalf("balances from=%s to=%s", p.Balance(from), p.Balance(to))
	}
	older := Key{Vintage: 1, Asset: NativeAsset}
	if err := p.Move(to, older, math.NewInt(1)); err != ErrVintage {
		t.Fatalf("downgrade: %v", err)
	}
	other := to
	other.Asset[0] = 1
	if err := p.Move(from, other, math.NewInt(1)); err != ErrVintage {
		t.Fatalf("asset change: %v", err)
	}
}

func TestMultiAssetStaysInert(t *testing.T) {
	var other [32]byte
	other[0] = 9
	if err := AllowAsset(false, other); err != ErrMultiAssetInactive {
		t.Fatalf("got %v", err)
	}
	if err := AllowAsset(false, NativeAsset); err != nil {
		t.Fatal(err)
	}
	if err := AllowAsset(true, other); err != nil {
		t.Fatal(err)
	}
}

func TestTokenPowers(t *testing.T) {
	if err := AllowToken(false, false, false); err != nil {
		t.Fatal(err)
	}
	for _, powers := range [][3]bool{{true, false, false}, {false, true, false}, {false, false, true}} {
		if err := AllowToken(powers[0], powers[1], powers[2]); err != ErrTokenPowers {
			t.Fatalf("powers %v: %v", powers, err)
		}
	}
}

func TestCapFloorAndExemptFee(t *testing.T) {
	floor := math.NewInt(UnshieldFloor)
	// 2% of 10 ORAMA is 0.2 ORAMA, so the floor wins.
	small := math.NewInt(10_000_000_000)
	if !CapAmount(small, floor).Equal(floor) {
		t.Fatalf("floor: %s", CapAmount(small, floor))
	}
	// 2% of 1000 ORAMA is 20 ORAMA.
	large := math.NewInt(1000_000_000_000)
	want := math.NewInt(20_000_000_000)
	if !CapAmount(large, floor).Equal(want) {
		t.Fatalf("pct: %s", CapAmount(large, floor))
	}

	lim := &Limiter{}
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	if out, err := lim.Apply(now, large, floor, math.NewInt(1), KindFeeBurn); err != nil || out != OutcomeAllow {
		t.Fatalf("fee: %v %v", out, err)
	}
	if !lim.Counted.IsZero() {
		t.Fatal("fee burn counted against the cap")
	}
	if _, err := lim.Apply(now, large, floor, want, KindAdapter); err != nil {
		t.Fatal(err)
	}
	if _, err := lim.Apply(now, large, floor, math.NewInt(1), KindAdapter); err != ErrCapExhausted {
		t.Fatalf("adapter over cap: %v", err)
	}
	out, err := lim.Apply(now, large, floor, math.NewInt(1), KindBond)
	if err != nil || out != OutcomeQueue {
		t.Fatalf("bond should queue, got %v %v", out, err)
	}
	// A full 24 hours later the bucket has drained empty.
	later := now.Add(Window)
	if _, err := lim.Apply(later, large, floor, math.NewInt(1), KindAdapter); err != nil {
		t.Fatal(err)
	}
}

func TestLimiterRoll_drainsOneCapPerWindow(t *testing.T) {
	cap := math.NewInt(86_400) // one norama per second
	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	lim := &Limiter{Start: start, Counted: cap}
	lim.Roll(start.Add(1000*time.Second), cap)
	if !lim.Counted.Equal(math.NewInt(85_400)) || !lim.Start.Equal(start.Add(1000*time.Second)) {
		t.Fatalf("after 1000 s: %s at %s", lim.Counted, lim.Start)
	}
	lim.Roll(start.Add(Window), cap)
	if !lim.Counted.Equal(math.ZeroInt()) {
		t.Fatalf("a full window drains a full bucket: %s", lim.Counted)
	}
}

func TestLimiterRoll_shortIntervalsStillAddUpToADrain(t *testing.T) {
	cap := math.NewInt(50) // 50 per 86,400 s: a norama drains every 1,728 s
	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	lim := &Limiter{Start: start, Counted: math.NewInt(10)}
	for s := int64(5); s <= 1727; s += 5 { // a block every 5 s, none drains a whole norama
		lim.Roll(start.Add(time.Duration(s)*time.Second), cap)
	}
	if !lim.Counted.Equal(math.NewInt(10)) || !lim.Start.Equal(start) {
		t.Fatalf("nothing drained yet and no time was thrown away: %s at %s", lim.Counted, lim.Start)
	}
	lim.Roll(start.Add(1728*time.Second), cap)
	if !lim.Counted.Equal(math.NewInt(9)) {
		t.Fatalf("one norama drained at 1,728 s: %s", lim.Counted)
	}
}

func TestLimiterRoll_neverDrainsMoreThanTheElapsedTimeAllows(t *testing.T) {
	cap := math.NewInt(50)
	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	for _, step := range []int64{1, 7, 61, 1728, 3000} {
		lim := &Limiter{Start: start, Counted: cap}
		var drainedTotal int64
		for s := step; s <= 3*86_400; s += step {
			before := lim.Counted
			lim.Roll(start.Add(time.Duration(s)*time.Second), cap)
			drainedTotal += before.Sub(lim.Counted).Int64()
			lim.Counted = cap // outflow refills the bucket at once
		}
		if drainedTotal > 3*cap.Int64() {
			t.Fatalf("step %d s: %d drained in 3 days, at most %d allowed", step, drainedTotal, 3*cap.Int64())
		}
	}
}

func TestLimiterRoll_emptyZeroAndShrunkenCases(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	fresh := &Limiter{}
	fresh.Roll(now, math.NewInt(50))
	if !fresh.Counted.IsZero() || !fresh.Start.Equal(now) {
		t.Fatalf("a new limiter starts empty at now: %s at %s", fresh.Counted, fresh.Start)
	}
	zeroCap := &Limiter{Start: now, Counted: math.NewInt(5)}
	zeroCap.Roll(now.Add(Window), math.ZeroInt())
	if !zeroCap.Counted.Equal(math.NewInt(5)) {
		t.Fatalf("a zero cap drains nothing: %s", zeroCap.Counted)
	}
	back := &Limiter{Start: now, Counted: math.NewInt(5)}
	back.Roll(now.Add(-time.Hour), math.NewInt(50))
	if !back.Counted.Equal(math.NewInt(5)) {
		t.Fatalf("a clock behind Start drains nothing: %s", back.Counted)
	}
	shrunk := &Limiter{Start: now, Counted: math.NewInt(100)}
	if !shrunk.Remaining(math.NewInt(50)).IsZero() {
		t.Fatal("a cap below the level leaves nothing, never a negative amount")
	}
	if !shrunk.Remaining(math.NewInt(130)).Equal(math.NewInt(30)) {
		t.Fatalf("remaining: %s", shrunk.Remaining(math.NewInt(130)))
	}
}

func TestConservation(t *testing.T) {
	p := NewPools()
	k := Key{Vintage: 1, Asset: NativeAsset}
	if err := p.Add(k, math.NewInt(1000)); err != nil {
		t.Fatal(err)
	}
	if err := p.Sub(k, math.NewInt(100)); err != nil {
		t.Fatal(err)
	}
	if err := p.Sub(k, math.NewInt(5)); err != nil {
		t.Fatal(err)
	}
	if !p.Balance(k).Equal(math.NewInt(895)) {
		t.Fatalf("balance %s", p.Balance(k))
	}
}
