package circuit

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

type StateChange struct {
	From    State
	To      State
	Reason  string
	RouteID string
	Time    time.Time
}

type Breaker struct {
	mu            sync.RWMutex
	failures      int
	state        State
	lastFailure  time.Time
	openedAt     time.Time
	halfOpenProbes int

	config        Config
	onStateChange func(StateChange)
}

type Config struct {
	Failures    int
	Cooldown   time.Duration
	HalfOpenMax int
}

func New(cfg Config, onStateChange func(StateChange)) *Breaker {
	if cfg.Failures == 0 {
		cfg.Failures = 5
	}
	if cfg.Cooldown == 0 {
		cfg.Cooldown = 30 * time.Second
	}
	if cfg.HalfOpenMax == 0 {
		cfg.HalfOpenMax = 3
	}
	return &Breaker{
		config:        cfg,
		state:        StateClosed,
		onStateChange: onStateChange,
	}
}

func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateHalfOpen:
		b.state = StateClosed
		b.failures = 0
		b.halfOpenProbes = 0
		b.emit(StateClosed, "probe succeeded")
	case StateClosed:
		b.failures = 0
	case StateOpen:
	}
}

func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures++
	b.lastFailure = time.Now()

	switch b.state {
	case StateClosed:
		if b.failures >= b.config.Failures {
			b.state = StateOpen
			b.openedAt = time.Now()
			b.emit(StateOpen, "consecutive failures")
		}
	case StateHalfOpen:
		b.state = StateOpen
		b.halfOpenProbes = 0
		b.emit(StateOpen, "half-open probe failed")
	case StateOpen:
	}
}

func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(b.openedAt) >= b.config.Cooldown {
			b.state = StateHalfOpen
			b.halfOpenProbes = 0
			b.emit(StateHalfOpen, "cooldown expired")
			return true
		}
		return false
	case StateHalfOpen:
		// Allow up to HalfOpenMax probes
		if b.halfOpenProbes < b.config.HalfOpenMax {
			b.halfOpenProbes++
			return true
		}
		return false
	}
	return false
}

func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *Breaker) Failures() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.failures
}

func (b *Breaker) Stats() (state State, failures int, lastFailure time.Time) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state, b.failures, b.lastFailure
}

func (b *Breaker) emit(to State, reason string) {
	if b.onStateChange != nil {
		b.onStateChange(StateChange{
			From:   b.state,
			To:     to,
			Reason: reason,
			Time:   time.Now(),
		})
	}
}

type Registry struct {
	mu       sync.RWMutex
	breakers map[string]*Breaker
	config   Config
	log      func(StateChange)
}

func NewRegistry(cfg Config, log func(StateChange)) *Registry {
	return &Registry{
		breakers: make(map[string]*Breaker),
		config:  cfg,
		log:     log,
	}
}

func (r *Registry) Get(routeID string) *Breaker {
	r.mu.RLock()
	b, ok := r.breakers[routeID]
	r.mu.RUnlock()

	if ok {
		return b
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if b, ok = r.breakers[routeID]; ok {
		return b
	}

	b = New(r.config, r.log)
	r.breakers[routeID] = b
	return b
}

func (r *Registry) Remove(routeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.breakers, routeID)
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.breakers)
}