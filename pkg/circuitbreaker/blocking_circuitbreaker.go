package circuitbreaker

import (
	"context"
	"sync"
)

type BlockingCircuitBreaker struct {
	semaphor         chan struct{}
	tripped          bool
	mu               sync.Mutex
	o11y             *CBObservability
	failsInARowLimit int
	failsInARow      int
}

func NewBlockingCircuitBreaker(o11y *CBObservability, failsInARowLimit int) *BlockingCircuitBreaker {
	sema := make(chan struct{})
	defer close(sema)

	return &BlockingCircuitBreaker{
		semaphor: sema,
		o11y:     o11y,
	}
}

func (cb *BlockingCircuitBreaker) Tripped() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.tripped
}

func (cb *BlockingCircuitBreaker) Fail() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.fail()
}

func (cb *BlockingCircuitBreaker) Success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.success()
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

func (cb *BlockingCircuitBreaker) fail() {
	if cb.tripped {
		return
	}

	cb.failsInARow += 1
	if cb.failsInARow >= cb.failsInARowLimit {
		cb.tripped = true
		cb.semaphor = make(chan struct{})
	}
}

func (cb *BlockingCircuitBreaker) success() {
	cb.failsInARow = 0
	if cb.tripped {
		defer close(cb.semaphor)
	}
	cb.tripped = false
}
