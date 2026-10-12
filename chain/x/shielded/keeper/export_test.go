package keeper

import "time"

// SetCheckerClockForTest replaces a proposal checker's clock and deadline.
func SetCheckerClockForTest(c *ProposalChecker, now func() time.Time, deadline time.Time) {
	c.now, c.deadline = now, deadline
}

// AdvanceFaultMemoryForTest moves the mempool's clock forward by d, to expire remembered faults.
func AdvanceFaultMemoryForTest(k Keeper, d time.Duration) {
	base := k.admission.now
	k.admission.now = func() time.Time { return base().Add(d) }
}
