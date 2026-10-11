package keeper

import "time"

// AdvanceFaultMemoryForTest moves the mempool's clock forward by d, to expire remembered faults.
func AdvanceFaultMemoryForTest(k Keeper, d time.Duration) {
	base := k.admission.now
	k.admission.now = func() time.Time { return base().Add(d) }
}
