package pool

import (
	"testing"
	"time"

	"cosmossdk.io/math"
)

// FuzzPoolServe: the queue never pays out more than the window's capacity, never pays an address
// more than it asked for or than the per-address cap, never a negative amount, and the grants
// line up one to one with the requests.
func FuzzPoolServe(f *testing.F) {
	f.Add(uint64(100), uint64(50), []byte{10, 20, 30, 0, 200})
	f.Add(uint64(0), uint64(5), []byte{1})
	f.Add(uint64(1<<63), uint64(1<<62), []byte{255, 255, 255, 255})
	f.Fuzz(func(t *testing.T, capacity, perAddress uint64, amounts []byte) {
		reqs := make([]Request, len(amounts))
		for i, a := range amounts {
			// A spread of magnitudes from the one byte, up to 2^64.
			reqs[i] = Request{Address: string(rune('a' + i%26)), Amount: math.NewIntFromUint64(uint64(a) << (uint(a) % 57))}
		}
		capInt, perInt := math.NewIntFromUint64(capacity), math.NewIntFromUint64(perAddress)
		grants := Serve(capInt, perInt, reqs)
		if len(grants) != len(reqs) {
			t.Fatalf("%d grants for %d requests", len(grants), len(reqs))
		}
		total := math.ZeroInt()
		for i, g := range grants {
			if g.Amount.IsNegative() || g.Amount.GT(reqs[i].Amount) || g.Amount.GT(perInt) {
				t.Fatalf("grant %d is %s for a request of %s and a per-address cap of %s", i, g.Amount, reqs[i].Amount, perInt)
			}
			total = total.Add(g.Amount)
		}
		if total.GT(capInt) {
			t.Fatalf("paid %s out of a capacity of %s", total, capInt)
		}
		again := Serve(capInt, perInt, reqs)
		for i := range grants {
			if !grants[i].Amount.Equal(again[i].Amount) {
				t.Fatal("Serve is not deterministic")
			}
		}
	})
}

// FuzzPoolAccounting runs a byte-coded sequence of Add, Sub, Move and limiter calls. A pool never
// goes negative, a failed call changes nothing, a Move keeps the total, and the limiter never lets
// a window's counted outflow pass its cap.
func FuzzPoolAccounting(f *testing.F) {
	f.Add([]byte{0, 1, 50, 1, 1, 20, 2, 1, 2, 30, 3, 1, 9})
	f.Add([]byte{1, 0, 1, 2, 0, 1})
	f.Fuzz(func(t *testing.T, script []byte) {
		p := NewPools()
		var lim Limiter
		clock := time.Unix(1_700_000_000, 0)
		key := func(b byte) Key { return Key{Vintage: uint32(b % 3), Asset: NativeAsset} }
		total := func() math.Int {
			sum := math.ZeroInt()
			for v := byte(0); v < 3; v++ {
				sum = sum.Add(p.Balance(key(v)))
			}
			return sum
		}
		for len(script) >= 3 {
			op, a, b := script[0]%4, script[1], script[2]
			script = script[3:]
			amount := math.NewIntFromUint64(uint64(b) * 1_000_000)
			before := total()
			switch op {
			case 0:
				if err := p.Add(key(a), amount); err == nil && !total().Equal(before.Add(amount)) {
					t.Fatal("Add changed the total by another amount")
				}
			case 1:
				bal := p.Balance(key(a))
				err := p.Sub(key(a), amount)
				if err != nil && !p.Balance(key(a)).Equal(bal) {
					t.Fatal("a refused Sub changed the balance")
				}
				if err == nil && !p.Balance(key(a)).Equal(bal.Sub(amount)) {
					t.Fatal("Sub took another amount")
				}
			case 2:
				if err := p.Move(key(a), key(b), amount); err == nil && !total().Equal(before) {
					t.Fatal("Move changed the total")
				} else if err != nil && !total().Equal(before) {
					t.Fatal("a refused Move changed the total")
				}
			default:
				clock = clock.Add(time.Duration(a) * time.Hour)
				kind := Kind(b % 5)
				out, err := lim.Apply(clock, total(), math.NewInt(UnshieldFloor), amount, kind)
				if err == nil && out == OutcomeAllow && kind != KindFeeBurn &&
					lim.Counted.GT(CapAmount(total(), math.NewInt(UnshieldFloor))) {
					t.Fatalf("counted %s passes the cap %s", lim.Counted, CapAmount(total(), math.NewInt(UnshieldFloor)))
				}
			}
			for v := byte(0); v < 3; v++ {
				if p.Balance(key(v)).IsNegative() {
					t.Fatal("a pool went negative")
				}
			}
		}
	})
}
