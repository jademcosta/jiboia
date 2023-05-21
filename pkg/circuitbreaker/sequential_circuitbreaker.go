package circuitbreaker

import (
	"sync"
	"time"
)

// type FlowCircuitBreaker[T any] struct {
// 	Name string
// }

// func NewFlowCircuitBreaker[T any]() *FlowCircuitBreaker[T] {
// 	return &FlowCircuitBreaker[T]{}
// }

// func (cb *FlowCircuitBreaker[T]) CallW(f func() (T, error)) (T, error) {
// 	t, err := f()
// 	if err != nil {
// 		panic("aaaaaa") //TODO: call cb fail
// 	}
// 	return t, err
// }

// const (
// 	OpenState = iota
// 	ClosedState
// )

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

// FIXME: validate config, like to not allow zero threshold
func NewSequentialCircuitBreaker(conf SequentialCircuitBreakerConfig) *SequentialCircuitBreaker {
	return &SequentialCircuitBreaker{
		cState: &circuitClosedState{
			conf: &conf,
		},
	}
}

func (cb *SequentialCircuitBreaker) Call(f func() error) error {
	cb.m.Lock()
	defer cb.m.Unlock()

	if cb.tripped() {
		return ErrorOpenCircuitBreaker
	}
	//TODO: Test that the mutex in this fn works
	err := f()
	if err != nil {
		cb.fail()
	} else {
		cb.success()
	}
	return err
}

// TODO: (jademcosta) I don't like the "ready" work. i need to find a better name
// TODO: if this is going to be kept public tests need to be made
func (cb *SequentialCircuitBreaker) Tripped() bool {
	cb.m.Lock()
	defer cb.m.Unlock()
	return cb.tripped()
}

func (cb *SequentialCircuitBreaker) Fail() {
	cb.m.Lock()
	defer cb.m.Unlock()
	cb.fail()
}

func (cb *SequentialCircuitBreaker) Success() {
	cb.m.Lock()
	defer cb.m.Unlock()
	cb.success()
}

func (cb *SequentialCircuitBreaker) tripped() bool {
	return cb.cState.blockCall()
}

func (cb *SequentialCircuitBreaker) fail() {
	cb.cState = cb.cState.fail()
}

func (cb *SequentialCircuitBreaker) success() {
	cb.cState = cb.cState.success()
}

// TODO: implement forceclose and forceopen

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

func (s *circuitClosedState) blockCall() bool {
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

func (s *circuitOpenState) blockCall() bool {
	expired := time.Now().After(s.until)
	return !expired
}

//TODO: create the half-open state, which is a state right after the "until" period has expired,
// and in where if the call fails, it enters again in open state
