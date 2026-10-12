package orchardproc

import (
	"errors"
	"testing"
	"time"

	"github.com/DeBrosOfficial/network/chain/x/shielded/verify"
)

func TestTimedMutex(t *testing.T) {
	m := newTimedMutex()
	if !m.TryLock() {
		t.Fatal("a free mutex was not taken")
	}
	if m.TryLock() {
		t.Fatal("a held mutex was taken")
	}
	start := time.Now()
	if m.LockWithin(30 * time.Millisecond) {
		t.Fatal("a held mutex was taken within the wait")
	}
	if took := time.Since(start); took < 25*time.Millisecond || took > time.Second {
		t.Fatalf("waited %s, want about 30ms", took)
	}
	go func() { time.Sleep(20 * time.Millisecond); m.Unlock() }()
	if !m.LockWithin(2 * time.Second) {
		t.Fatal("a mutex released inside the wait was not taken")
	}
	m.Unlock()
	if m.LockWithin(0) != true {
		t.Fatal("a free mutex is taken with no wait")
	}
}

// A request with a deadline takes no longer than the deadline, whatever the request timeout is: the
// process that did not answer is stopped like any other timeout, and the answer is a fault.
func TestVerifyRunningWithin_aRequestIsCutToTheDeadline(t *testing.T) {
	f := newFixture(t, 30*time.Second)
	if err := f.v.Warm(); err != nil {
		t.Fatal(err)
	}
	f.mode(modeHang)
	start := time.Now()
	err := f.v.VerifyRunningWithin(fakeBundle(), nil, 300*time.Millisecond)
	if !errors.Is(err, verify.ErrVerifierFault) {
		t.Fatalf("got %v, want a fault at the deadline", err)
	}
	if took := time.Since(start); took < 250*time.Millisecond || took > 3*time.Second {
		t.Fatalf("the request took %s, want about 300ms and not the 30 s request timeout (err %v)", took, err)
	}
}

// The wait for a busy verifier is cut to the deadline too.
func TestVerifyRunningWithin_theWaitIsCutToTheDeadline(t *testing.T) {
	f := newFixture(t, 30*time.Second)
	if err := f.v.Warm(); err != nil {
		t.Fatal(err)
	}
	f.v.mu.Lock()
	defer f.v.mu.Unlock()
	start := time.Now()
	err := f.v.VerifyRunningWithin(fakeBundle(), nil, 40*time.Millisecond)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("got %v, want ErrBusy", err)
	}
	if took := time.Since(start); took > DefaultBusyWait-50*time.Millisecond {
		t.Fatalf("waited %s, want about 40ms and not the %s busy wait", took, DefaultBusyWait)
	}
}
