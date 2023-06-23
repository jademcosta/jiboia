package circuitbreaker

import (
	"sync"
	"time"
)

// TODO: Add half-open state and then implement exponential backoff
type SequentialCircuitBreakerConfig struct {
	OpenInterval       time.Duration
	FailCountThreshold int
}

type SequentialCircuitBreaker struct {
	Name   string
	m      sync.Mutex
	cState circuitState
}

func NewSequentialCircuitBreaker(conf SequentialCircuitBreakerConfig) *SequentialCircuitBreaker {
	return &SequentialCircuitBreaker{
		cState: &circuitClosedState{
			conf: &conf,
		},
	}
}

func (cb *SequentialCircuitBreaker) Call(f func() error) error {

	if cb.Tripped() {
		return ErrorOpenCircuitBreaker
	}

	err := f()
	if err != nil {
		cb.Fail()
	} else {
		cb.Success()
	}
	return err
}

func (cb *SequentialCircuitBreaker) Tripped() bool {
	cb.m.Lock()
	defer cb.m.Unlock()
	return cb.cState.isCallBlocked()
}

func (cb *SequentialCircuitBreaker) Fail() {
	cb.m.Lock()
	defer cb.m.Unlock()
	cb.cState = cb.cState.fail()
}

func (cb *SequentialCircuitBreaker) Success() {
	cb.m.Lock()
	defer cb.m.Unlock()
	cb.cState = cb.cState.success()
}

// Closed state
type circuitClosedState struct {
	failsInARow int
	conf        *SequentialCircuitBreakerConfig
}

func (s *circuitClosedState) success() circuitState {
	s.failsInARow = 0
	return s
}

func (s *circuitClosedState) fail() circuitState {
	s.failsInARow += 1

	if s.failsInARow >= s.conf.FailCountThreshold {
		return &circuitOpenState{
			conf:  s.conf,
			until: time.Now().Add(s.conf.OpenInterval),
		}
	} else {
		return s
	}
}

func (s *circuitClosedState) isCallBlocked() bool {
	return false
}

// Open state
type circuitOpenState struct {
	until time.Time
	conf  *SequentialCircuitBreakerConfig
}

func (s *circuitOpenState) success() circuitState {
	return &circuitClosedState{
		conf: s.conf,
	}
}

func (s *circuitOpenState) fail() circuitState {
	s.until = time.Now().Add(s.conf.OpenInterval)

	return s
}

func (s *circuitOpenState) isCallBlocked() bool {
	expired := time.Now().After(s.until)
	return !expired
}
