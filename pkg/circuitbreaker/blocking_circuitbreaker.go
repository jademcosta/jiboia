package circuitbreaker

import (
	"context"
	"sync"
)

type BlockingCircuitBreaker struct {
	semaphor chan struct{}
	mu       sync.Mutex
	cState   circuitState
}

func NewBlockingCircuitBreaker(o11y *CBObservability) *BlockingCircuitBreaker {
	sema := make(chan struct{})
	defer close(sema)

	return &BlockingCircuitBreaker{
		semaphor: sema,
		cState: &circuitClosedState{
			o11y: o11y,
		},
	}
}

func (cb *BlockingCircuitBreaker) Tripped() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.cState.isCallBlocked()
}

func (cb *BlockingCircuitBreaker) Fail() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.cState = cb.cState.fail()
}

func (cb *BlockingCircuitBreaker) Success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.cState = cb.cState.success()
}

func (cb *BlockingCircuitBreaker) CallBlockingWithTimeout(ctx context.Context, f func() error) error {
	select {
	case <-cb.semaphor:

	case <-ctx.Done():
	}

	err := f()

	if err != nil {
		cb.Fail()
	} else {
		cb.Success()
	}

	return err
}
