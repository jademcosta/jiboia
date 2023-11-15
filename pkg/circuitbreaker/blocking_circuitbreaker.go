package circuitbreaker

import (
	"context"
	"sync"
	"time"
)

type BlockingCircuitBreaker struct {
	semaphor         chan struct{}
	mu               sync.Mutex
	o11y             *CBObservability
	failsInARowLimit int
	failsInARow      int
	openInterval     time.Duration
	timer            *time.Timer
}

func NewBlockingCircuitBreaker(o11y *CBObservability, failsInARowLimit int, openInterval time.Duration) *BlockingCircuitBreaker {
	sema := make(chan struct{})
	defer close(sema)

	return &BlockingCircuitBreaker{
		semaphor:         sema,
		o11y:             o11y,
		failsInARowLimit: failsInARowLimit,
		openInterval:     openInterval,
	}
}

func (cb *BlockingCircuitBreaker) Tripped() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.tripped()
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
		return context.DeadlineExceeded
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
	if cb.tripped() {
		stopCallWorked := cb.timer.Stop()
		if stopCallWorked {
			cb.timer.Reset(cb.openInterval)
		}
		return
	}

	cb.failsInARow += 1
	if cb.failsInARow >= cb.failsInARowLimit {
		cb.semaphor = make(chan struct{})
		cb.timer = time.AfterFunc(cb.openInterval, cb.Success)
	}
}

func (cb *BlockingCircuitBreaker) success() {
	cb.failsInARow = 0
	if cb.tripped() {
		cb.timer.Stop()
		close(cb.semaphor)
	}
}

func (cb *BlockingCircuitBreaker) tripped() bool {
	select {
	case <-cb.semaphor:
		return false
	default:
		return true
	}
}
