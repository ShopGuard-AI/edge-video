package circuit

import (
	"fmt"
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
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

type Breaker struct {
	name              string
	maxFailures       int64
	resetTimeout      time.Duration
	halfOpenSuccesses int
	
	mu            sync.RWMutex
	state         State
	failures      int64
	successes     int64
	lastFailTime  time.Time
	lastStateTime time.Time
}

func NewBreaker(name string, maxFailures int64, resetTimeout time.Duration) *Breaker {
	return &Breaker{
		name:              name,
		maxFailures:       maxFailures,
		resetTimeout:      resetTimeout,
		halfOpenSuccesses: 3,
		state:             StateClosed,
		lastStateTime:     time.Now(),
	}
}

func (cb *Breaker) Call(fn func() error) error {
	if !cb.Allow() {
		return fmt.Errorf("circuit breaker %s aberto", cb.name)
	}
	
	err := fn()
	
	if err != nil {
		cb.RecordFailure()
		return err
	}
	
	cb.RecordSuccess()
	return nil
}

func (cb *Breaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	switch cb.state {
	case StateClosed:
		return true
		
	case StateOpen:
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.setState(StateHalfOpen)
			return true
		}
		return false
		
	case StateHalfOpen:
		return true
		
	default:
		return false
	}
}

func (cb *Breaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.successes++
	
	switch cb.state {
	case StateClosed:
		cb.failures = 0
		
	case StateHalfOpen:
		if cb.successes >= int64(cb.halfOpenSuccesses) {
			cb.setState(StateClosed)
			cb.failures = 0
			cb.successes = 0
		}
	}
}

func (cb *Breaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.failures++
	cb.lastFailTime = time.Now()
	cb.successes = 0
	
	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.maxFailures {
			cb.setState(StateOpen)
		}
		
	case StateHalfOpen:
		cb.setState(StateOpen)
	}
}

func (cb *Breaker) setState(newState State) {
	if cb.state != newState {
		oldState := cb.state
		cb.state = newState
		cb.lastStateTime = time.Now()
		fmt.Printf("Circuit breaker %s: %s -> %s (falhas: %d)\n",
			cb.name, oldState, newState, cb.failures)
	}
}

func (cb *Breaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *Breaker) Stats() BreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return BreakerStats{
		Name:            cb.name,
		State:           cb.state,
		Failures:        cb.failures,
		Successes:       cb.successes,
		MaxFailures:     cb.maxFailures,
		LastFailTime:    cb.lastFailTime,
		LastStateChange: cb.lastStateTime,
	}
}

func (cb *Breaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.lastStateTime = time.Now()
}

type BreakerStats struct {
	Name            string
	State           State
	Failures        int64
	Successes       int64
	MaxFailures     int64
	LastFailTime    time.Time
	LastStateChange time.Time
}

func (bs BreakerStats) String() string {
	return fmt.Sprintf("Circuit[%s]: %s, Failures: %d/%d, Successes: %d",
		bs.Name, bs.State, bs.Failures, bs.MaxFailures, bs.Successes)
}
