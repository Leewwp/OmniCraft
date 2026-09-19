// Package breaker implements the SP-24 R5 three-state circuit breaker
// (polyu ModelHealthStore blueprint): consecutive failures open the circuit
// for a configured timeout, after which a single HALF_OPEN probe permit is
// handed out via CAS — stale probe results from an earlier epoch can never
// close a freshly re-opened circuit.
package breaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrOpen is returned by Do when the circuit is open (or the half-open probe
// permit is held elsewhere). Callers treat it like any dependency failure
// and walk their existing fallback chain; the degraded-marker semantics at
// each mount point stay untouched.
var ErrOpen = errors.New("circuit breaker open")

// State is the circuit state. The zero value is Closed so an unconfigured
// breaker fails closed (calls flow).
type State int32

const (
	Closed State = iota
	Open
	HalfOpen
)

func (s State) String() string {
	switch s {
	case Open:
		return "open"
	case HalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

// Config carries the two tunables every mount shares. Defaults mirror the
// polyu blueprint: 2 consecutive failures, 30s open timeout.
type Config struct {
	FailureThreshold int
	OpenTimeout      time.Duration
}

func (c Config) withDefaults() Config {
	if c.FailureThreshold < 1 {
		c.FailureThreshold = 2
	}
	if c.OpenTimeout <= 0 {
		c.OpenTimeout = 30 * time.Second
	}
	return c
}

// Breaker is safe for concurrent use.
type Breaker struct {
	name   string
	config Config
	now    func() time.Time

	state atomic.Int32

	// mu guards failures/openedAt; the state word itself is atomic so Do's
	// fast path (closed circuit) never takes the lock.
	mu       sync.Mutex
	failures int
	openedAt time.Time

	// epoch increments on every HalfOpen entry; probeTaken is the single
	// half-open probe permit (CAS). A result carrying an old epoch is
	// dropped: it belongs to a previous half-open window and must not close
	// (or re-open) the current circuit.
	epoch      atomic.Int64
	probeTaken atomic.Bool

	onChange func(name string, from, to State)
}

// New builds a breaker; onChange (optional) fires once per state transition
// with the dependency name, for WARN logging and metrics at the mount.
func New(name string, config Config, onChange func(name string, from, to State)) *Breaker {
	return &Breaker{name: name, config: config.withDefaults(), now: time.Now, onChange: onChange}
}

func (b *Breaker) transition(to State) {
	from := State(b.state.Swap(int32(to)))
	if from != to && b.onChange != nil {
		b.onChange(b.name, from, to)
	}
}

// State snapshots the current circuit state.
func (b *Breaker) State() State {
	return State(b.state.Load())
}

// Do runs fn under the circuit. It returns ErrOpen without calling fn when
// the circuit is open and no half-open probe permit is available; otherwise
// fn's result is recorded (success resets the failure streak, a failure
// advances it) and returned unchanged.
func (b *Breaker) Do(fn func() error) error {
	epoch, ok := b.acquire()
	if !ok {
		return ErrOpen
	}
	err := fn()
	b.complete(epoch, err)
	return err
}

// acquire returns the half-open probe epoch when the call may proceed.
func (b *Breaker) acquire() (int64, bool) {
	if State(b.state.Load()) == Closed {
		return 0, true
	}
	b.mu.Lock()
	switch State(b.state.Load()) {
	case Closed:
		b.mu.Unlock()
		return 0, true
	case Open:
		if b.now().Sub(b.openedAt) < b.config.OpenTimeout {
			b.mu.Unlock()
			return 0, false
		}
		b.transition(HalfOpen)
		b.epoch.Add(1)
		b.probeTaken.Store(false)
		b.mu.Unlock()
	case HalfOpen:
		b.mu.Unlock()
	}
	if !b.probeTaken.CompareAndSwap(false, true) {
		return 0, false
	}
	return b.epoch.Load(), true
}

// complete records a call outcome against the epoch acquire returned.
func (b *Breaker) complete(epoch int64, err error) {
	if err == nil {
		b.recordSuccess(epoch)
		return
	}
	b.recordFailure(epoch)
}

func (b *Breaker) recordSuccess(epoch int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch State(b.state.Load()) {
	case HalfOpen:
		if b.epoch.Load() != epoch {
			// Stale probe from an earlier half-open window: never closes
			// the current circuit.
			return
		}
		b.failures = 0
		b.transition(Closed)
	case Closed:
		b.failures = 0
	case Open:
		// A success racing the open transition (call started while closed):
		// no effect — the recorded failures already tripped the circuit.
	}
}

func (b *Breaker) recordFailure(epoch int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch State(b.state.Load()) {
	case HalfOpen:
		if b.epoch.Load() != epoch {
			return
		}
		b.openedAt = b.now()
		b.transition(Open)
	case Closed:
		b.failures++
		if b.failures >= b.config.FailureThreshold {
			b.openedAt = b.now()
			b.transition(Open)
		}
	case Open:
		// Late failure from a call that started before the circuit opened:
		// keep the original openedAt so the half-open clock does not reset.
	}
}

// SetClock overrides the time source (tests only).
func (b *Breaker) SetClock(now func() time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.now = now
}
