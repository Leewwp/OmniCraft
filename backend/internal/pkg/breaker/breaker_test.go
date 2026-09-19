package breaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestBreaker() (*Breaker, *fakeClock) {
	c := &fakeClock{t: time.Unix(0, 0)}
	b := New("dep", Config{FailureThreshold: 2, OpenTimeout: 30 * time.Second}, nil)
	b.SetClock(c.now)
	return b, c
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeClock) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

func TestClosedUntilThresholdThenOpen(t *testing.T) {
	b, clock := newTestBreaker()
	if b.State() != Closed {
		t.Fatalf("initial state = %s, want closed", b.State())
	}
	if err := b.Do(func() error { return errors.New("boom") }); err == nil {
		t.Fatal("first failure must propagate")
	}
	if b.State() != Closed {
		t.Fatalf("one failure = %s, want closed (threshold 2)", b.State())
	}
	_ = b.Do(func() error { return errors.New("boom") })
	if b.State() != Open {
		t.Fatalf("two failures = %s, want open", b.State())
	}
	if err := b.Do(func() error { return nil }); err != ErrOpen {
		t.Fatalf("open circuit Do = %v, want ErrOpen", err)
	}
	// Still inside the open window.
	clock.advance(29 * time.Second)
	if err := b.Do(func() error { return nil }); err != ErrOpen {
		t.Fatalf("inside timeout Do = %v, want ErrOpen", err)
	}
}

func TestHalfOpenSingleProbeAndClose(t *testing.T) {
	b, clock := newTestBreaker()
	_ = b.Do(func() error { return errors.New("a") })
	_ = b.Do(func() error { return errors.New("b") })
	clock.advance(31 * time.Second)
	if err := b.Do(func() error { return nil }); err != nil {
		t.Fatalf("probe call errored: %v", err)
	}
	if b.State() != Closed {
		t.Fatalf("successful probe = %s, want closed", b.State())
	}
	// Failure streak reset: one new failure must not re-open.
	_ = b.Do(func() error { return errors.New("c") })
	if b.State() != Closed {
		t.Fatalf("single failure after close = %s, want closed", b.State())
	}
}

func TestHalfOpenProbeFailureReopensWithFreshClock(t *testing.T) {
	b, clock := newTestBreaker()
	_ = b.Do(func() error { return errors.New("a") })
	_ = b.Do(func() error { return errors.New("b") })
	clock.advance(31 * time.Second)
	if err := b.Do(func() error { return errors.New("probe fails") }); err == nil {
		t.Fatal("probe failure must propagate")
	}
	if b.State() != Open {
		t.Fatalf("failed probe = %s, want open", b.State())
	}
	// Immediately after re-opening the circuit must reject again.
	if err := b.Do(func() error { return nil }); err != ErrOpen {
		t.Fatalf("right after reopen Do = %v, want ErrOpen", err)
	}
}

func TestConcurrentHalfOpenPermitsSingleProbe(t *testing.T) {
	b, clock := newTestBreaker()
	_ = b.Do(func() error { return errors.New("a") })
	_ = b.Do(func() error { return errors.New("b") })
	clock.advance(31 * time.Second)

	// Hold the single half-open probe in flight while racers pile in: the
	// CAS permit must reject every one of them (once the probe completes and
	// closes the circuit, later callers legitimately flow again — that is
	// closed-state behaviour, not a second probe).
	probeRelease := make(chan struct{})
	probeDone := make(chan struct{})
	go func() {
		defer close(probeDone)
		_ = b.Do(func() error {
			<-probeRelease
			return nil
		})
	}()
	time.Sleep(10 * time.Millisecond)

	const racers = 16
	var rejected, permitted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Do(func() error { return nil }); err == ErrOpen {
				rejected.Add(1)
			} else if err == nil {
				permitted.Add(1)
			}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	close(probeRelease)
	wg.Wait()
	<-probeDone

	if got := permitted.Load(); got != 0 {
		t.Fatalf("%d racers ran alongside the in-flight probe, want 0", got)
	}
	if got := rejected.Load(); got != racers {
		t.Fatalf("rejected %d of %d racers, want all", got, racers)
	}
	if b.State() != Closed {
		t.Fatalf("after successful probe = %s, want closed", b.State())
	}
}

// 防旧探测误关 (polyu blueprint): a stale probe result from an earlier
// half-open window must not close a freshly re-opened circuit.
func TestStaleEpochProbeCannotCloseReopenedCircuit(t *testing.T) {
	b, clock := newTestBreaker()
	_ = b.Do(func() error { return errors.New("a") })
	_ = b.Do(func() error { return errors.New("b") })
	clock.advance(31 * time.Second)

	// Acquire the first half-open probe permit manually and hold it.
	staleEpoch, ok := b.acquire()
	if !ok {
		t.Fatal("first probe permit must be granted")
	}
	// The probe fails → circuit re-opens, clock advances past the new window.
	b.complete(staleEpoch, errors.New("probe1 fails"))
	if b.State() != Open {
		t.Fatalf("failed probe = %s, want open", b.State())
	}
	clock.advance(31 * time.Second)
	// A second probe succeeds and closes the circuit.
	if err := b.Do(func() error { return nil }); err != nil {
		t.Fatalf("second probe errored: %v", err)
	}
	if b.State() != Closed {
		t.Fatalf("after second probe = %s, want closed", b.State())
	}
	// Trip the circuit open again, then deliver the stale success.
	_ = b.Do(func() error { return errors.New("x") })
	_ = b.Do(func() error { return errors.New("y") })
	if b.State() != Open {
		t.Fatalf("tripped = %s, want open", b.State())
	}
	b.complete(staleEpoch, nil)
	if b.State() != Open {
		t.Fatalf("stale-epoch success = %s, want still open", b.State())
	}
}

func TestStateChangeCallbackFiresOnTransitions(t *testing.T) {
	var mu sync.Mutex
	var events []string
	b := New("cb", Config{FailureThreshold: 1, OpenTimeout: time.Second}, func(name string, from, to State) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, from.String()+"->"+to.String())
	})
	fake := &fakeClock{t: time.Unix(0, 0)}
	b.SetClock(fake.now)

	_ = b.Do(func() error { return errors.New("a") }) // closed->open
	fake.advance(2 * time.Second)
	_ = b.Do(func() error { return nil }) // open->half_open->closed (probe success)
	mu.Lock()
	defer mu.Unlock()
	want := []string{"closed->open", "open->half_open", "half_open->closed"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

func TestZeroValueConfigFailsClosed(t *testing.T) {
	b := New("defaults", Config{}, nil)
	for i := 0; i < 3; i++ {
		if err := b.Do(func() error { return nil }); err != nil {
			t.Fatalf("default breaker rejected call %d: %v", i, err)
		}
	}
	if b.State() != Closed {
		t.Fatalf("success-only default breaker = %s, want closed", b.State())
	}
}
