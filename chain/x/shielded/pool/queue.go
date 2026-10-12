package pool

import "cosmossdk.io/math"

// Request is one queued bond or deposit unshield.
type Request struct {
	Address string
	Amount  math.Int
}

// Grant is what the window actually pays that request.
type Grant struct {
	Address string
	Amount  math.Int
}

// Serve pays queued unshields oldest first, in the order of reqs. Each request gets the least of
// what it asked, what the window's capacity has left and what its address's per-address cap has
// left across all that address's requests, so no address takes more than perAddress and a request
// cannot be starved by one that arrived later. The result has one grant per request, in order.
//
// A grant that does not pay the whole request is a part payment. When accept is not nil it must
// agree to a part payment: a part that accept refuses (a delegation that would be dust) is not
// issued and the capacity stays for the next request. A grant that pays the whole request is always
// issued, so the target's own refusal is reported when it is paid.
//
// pay makes the payment and reports whether it went out (nil means every payment goes out). The
// capacity and the address's cap are used up only by a payment that went out: a request whose
// payment is refused is skipped, its grant is zero, its index is returned in refused, and the
// requests behind it are served from the capacity it did not take.
func Serve(capacity, perAddress math.Int, reqs []Request, accept func(i int, grant math.Int) bool, pay func(i int, grant math.Int) bool) (grants []Grant, refused []int) {
	grants = make([]Grant, len(reqs))
	left := capacity
	paid := map[string]math.Int{}
	for i, r := range reqs {
		grants[i] = Grant{Address: r.Address, Amount: math.ZeroInt()}
		if !left.IsPositive() || !perAddress.IsPositive() || r.Amount.IsNil() || !r.Amount.IsPositive() {
			continue
		}
		used, ok := paid[r.Address]
		if !ok {
			used = math.ZeroInt()
		}
		grant := math.MinInt(r.Amount, math.MinInt(left, perAddress.Sub(used)))
		if !grant.IsPositive() {
			continue
		}
		if grant.LT(r.Amount) && accept != nil && !accept(i, grant) {
			continue
		}
		if pay != nil && !pay(i, grant) {
			refused = append(refused, i)
			continue
		}
		grants[i].Amount = grant
		left = left.Sub(grant)
		paid[r.Address] = used.Add(grant)
	}
	return grants, refused
}
