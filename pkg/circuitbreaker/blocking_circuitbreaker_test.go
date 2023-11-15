package circuitbreaker_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/stretchr/testify/assert"
)

func TestABrandNewBlockingCBStartsWithClosedState(t *testing.T) {
	sut := circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, 1000*time.Millisecond)

	for i := 0; i <= 5; i++ {
		err := sut.CallBlockingWithTimeout(context.Background(), alwaysSuccessfulFn)
		assert.NoError(t, err, "should return no error")
	}
}

func TestBlockingCBSuccessCallsOnClosedStateKeepItClosed(t *testing.T) {
	sut := circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, 60000*time.Millisecond)
	ctx := context.Background()

	t1 := time.Now()
	sut.Success()
	err := sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error after success calls")
	assert.WithinDuration(t, t1, time.Now(), 500*time.Millisecond, "should not have blocked the call")

	for i := 0; i < 11; i++ {
		sut.Success()
	}
	t1 = time.Now()
	err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error after several success calls")
	assert.WithinDuration(t, t1, time.Now(), 500*time.Millisecond, "should not have blocked the call")

	for i := 0; i < 11; i++ {
		_ = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	}
	t1 = time.Now()
	err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error after several success calls")
	assert.WithinDuration(t, t1, time.Now(), 500*time.Millisecond, "should not have blocked the call")
}

func TestBlockingCBOneSuccessResetsTheFailStreak(t *testing.T) {
	sut := circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 11, 60000*time.Millisecond)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		sut.Fail()
	}

	sut.Success()
	sut.Fail()
	sut.Fail()
	sut.Fail()
	t1 := time.Now()
	err := sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
	assert.WithinDuration(t, t1, time.Now(), 500*time.Millisecond, "should not have blocked the call")

	sut = circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 11, 60000*time.Millisecond)

	for i := 0; i < 10; i++ {
		_ = sut.CallBlockingWithTimeout(ctx, alwaysFailFn)
	}

	t1 = time.Now()
	err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
	assert.WithinDuration(t, t1, time.Now(), 500*time.Millisecond, "should not have blocked the call")

	_ = sut.CallBlockingWithTimeout(ctx, alwaysFailFn)
	_ = sut.CallBlockingWithTimeout(ctx, alwaysFailFn)
	_ = sut.CallBlockingWithTimeout(ctx, alwaysFailFn)
	t1 = time.Now()
	err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
	assert.WithinDuration(t, t1, time.Now(), 500*time.Millisecond, "should not have blocked the call")
}

func TestBlockingCBBecomesClosedAfterThreshold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test test")
	}

	openInterval := 500 * time.Millisecond
	ctx := context.Background()
	sut := circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, openInterval)

	sut.Fail()
	assert.True(t, sut.Tripped(), "CB should be tripped")
	t1 := time.Now()
	err := sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	elapsed := time.Since(t1)
	assert.GreaterOrEqual(t, elapsed, openInterval-100*time.Millisecond, "should have waited for the CB to be open")
	assert.InDelta(t, elapsed, openInterval, float64(500*time.Millisecond), "should not wait more than the open interval provided")
	assert.False(t, sut.Tripped(), "CB should not be tripped")
	assert.NoError(t, err, "should return no error")

	err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
	assert.False(t, sut.Tripped(), "CB should not be tripped")

	sut = circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, openInterval)

	_ = sut.CallBlockingWithTimeout(ctx, alwaysFailFn)

	assert.True(t, sut.Tripped(), "CB should be tripped")
	t1 = time.Now()
	err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	elapsed = time.Since(t1)
	assert.GreaterOrEqual(t, elapsed, openInterval-100*time.Millisecond, "should have waited for the CB to be open")
	assert.InDelta(t, elapsed, openInterval, float64(500*time.Millisecond), "should not wait more than the open interval provided")
	assert.NoError(t, err, "should return no error")
	assert.False(t, sut.Tripped(), "CB should not be tripped")

	err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
	assert.False(t, sut.Tripped(), "CB should not be tripped")
}

func TestBlockingCBBlocksAllRoutinesWhenOpen(t *testing.T) {
	ctx := context.Background()
	openInterval := 2 * time.Second
	sut := circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 3, openInterval)

	t1 := time.Now()
	sut.Fail()
	sut.Fail()
	err := sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn) //Resets the fail counter
	assert.NoError(t, err, "should return no error")
	err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.WithinDuration(t, t1, time.Now(), openInterval)
	assert.NoError(t, err, "should return no error")

	t1 = time.Now()
	sut.Fail()
	sut.Fail()
	sut.Fail()
	err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	elapsed := time.Since(t1)
	assert.GreaterOrEqual(t, elapsed, openInterval-500*time.Millisecond, "should have waited for the CB to be open")
	assert.InDelta(t, elapsed, openInterval, float64(1*time.Second), "should not wait more than the open interval provided")
	assert.NoError(t, err, "should return no error (because it was blocked waiting the CB to close)")
}

func TestBlockingCBCanCloseAndOpenMiultipleTimes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test test")
	}

	openInterval := 100 * time.Millisecond
	ctx := context.Background()
	sut := circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, openInterval)

	for i := 0; i <= 17; i++ {
		sut.Fail()
		assert.True(t, sut.Tripped(), "should be tripped")
		err := sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
		assert.NoError(t, err, "should return no error")
		assert.False(t, sut.Tripped(), "should not be tripped")

		t1 := time.Now()
		err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
		assert.WithinDuration(t, t1, time.Now(), 50*time.Millisecond, "should not wait for open interval as the CB is closed")
		assert.NoError(t, err, "should return no error")
		assert.False(t, sut.Tripped(), "should not be tripped")
	}

	sut = circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, openInterval)

	for i := 0; i <= 17; i++ {
		sut.CallBlockingWithTimeout(ctx, alwaysFailFn)

		assert.True(t, sut.Tripped(), "should be tripped")
		err := sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
		assert.NoError(t, err, "should return no error")
		assert.False(t, sut.Tripped(), "should not be tripped")

		t1 := time.Now()
		err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
		assert.WithinDuration(t, t1, time.Now(), 50*time.Millisecond, "should not wait for open interval as the CB is closed")
		assert.NoError(t, err, "should return no error")
		assert.False(t, sut.Tripped(), "should not be tripped")
	}
}

func TestBlockingCBWhenOpenASuccessCallMakesClosed(t *testing.T) {
	openInterval := 5 * time.Second
	ctx := context.Background()
	sut := circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, openInterval)

	sut.Fail()
	assert.True(t, sut.Tripped(), "should be tripped")

	sut.Success()
	assert.False(t, sut.Tripped(), "should not be tripped")
	err := sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")

	_ = sut.CallBlockingWithTimeout(ctx, alwaysFailFn)
	assert.True(t, sut.Tripped(), "should be tripped")

	sut.Success()
	assert.False(t, sut.Tripped(), "should not be tripped")
	err = sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
}

func TestBlockingCBCallingFailWhenOpenIncreasesThreshold(t *testing.T) {
	openInterval := 500 * time.Millisecond
	ctx := context.Background()
	sut := circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, openInterval)

	answers := make(chan error, 1)

	sut.Fail()

	go func() {
		answers <- sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	}()

	assert.True(t, sut.Tripped(), "should be tripped")
	time.Sleep(60 * time.Millisecond)
	sut.Fail()
	time.Sleep(170 * time.Millisecond)
	assert.True(t, sut.Tripped(), "should be tripped")
	sut.Fail()
	time.Sleep(260 * time.Millisecond)
	assert.True(t, sut.Tripped(), "should be tripped")
	sut.Fail()
	time.Sleep(170 * time.Millisecond)
	assert.True(t, sut.Tripped(), "should be tripped")
	assert.Len(t, answers, 0, "should not have unblocked any blocked caller")
	time.Sleep(380 * time.Millisecond)
	assert.False(t, sut.Tripped(), "should not be tripped")

	sut.CallBlockingWithTimeout(ctx, alwaysFailFn)
	assert.True(t, sut.Tripped(), "should be tripped")
	time.Sleep(60 * time.Millisecond)
	sut.Fail()
	time.Sleep(170 * time.Millisecond)
	assert.True(t, sut.Tripped(), "should be tripped")
	time.Sleep(380 * time.Millisecond)
	assert.False(t, sut.Tripped(), "should not be tripped")
}

func TestReturnsContextDeadlineErrorIfContextCancelled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test test")
	}

	answer := make(chan error, 1)
	openInterval := 500 * time.Millisecond
	ctx := context.Background()
	sut := circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, openInterval)

	sut.Fail()
	assert.True(t, sut.Tripped(), "should be tripped")

	ctx, cancelFn := context.WithCancel(ctx)
	var err error
	go func() {
		answer <- sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	}()
	time.Sleep(100 * time.Millisecond)
	cancelFn()
	time.Sleep(50 * time.Millisecond)
	err = <-answer

	assert.Error(t, err, "should have returned error")
	assert.ErrorIs(t, err, context.DeadlineExceeded, "the error should be deadline exceeded")

	sut = circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, openInterval)
	ctx = context.Background()
	ctx, cancelFn = context.WithCancel(ctx)

	sut.CallBlockingWithTimeout(ctx, alwaysFailFn)
	assert.True(t, sut.Tripped(), "should be tripped")

	go func() {
		answer <- sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	}()
	time.Sleep(100 * time.Millisecond)
	cancelFn()
	time.Sleep(50 * time.Millisecond)
	err = <-answer

	assert.Error(t, err, "should have returned error")
	assert.ErrorIs(t, err, context.DeadlineExceeded, "the error should be deadline exceeded")

	sut = circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, openInterval)
	ctx = context.Background()
	ctx, cancelFn = context.WithTimeout(ctx, 200*time.Millisecond)

	sut.CallBlockingWithTimeout(ctx, alwaysFailFn)
	assert.True(t, sut.Tripped(), "should be tripped")

	go func() {
		answer <- sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
	}()
	time.Sleep(100 * time.Millisecond)
	cancelFn()
	time.Sleep(50 * time.Millisecond)
	err = <-answer

	assert.Error(t, err, "should have returned error")
	assert.ErrorIs(t, err, context.DeadlineExceeded, "the error should be deadline exceeded")
}

func TestSeveralGoroutinesCanBeBlocked(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test test")
	}

	var wg sync.WaitGroup
	goroutinesCount := 23
	answer := make(chan error, goroutinesCount)
	openInterval := 500 * time.Millisecond
	ctx := context.Background()
	sut := circuitbreaker.NewBlockingCircuitBreaker(dummyO11y, 1, openInterval)

	t1 := time.Now()
	sut.Fail()
	assert.True(t, sut.Tripped(), "should be tripped")

	wg.Add(goroutinesCount)
	for i := 0; i < goroutinesCount; i++ {
		go func() {
			answer <- sut.CallBlockingWithTimeout(ctx, alwaysSuccessfulFn)
			wg.Done()
		}()
	}

	wg.Wait()

	assert.InDelta(t, time.Since(t1), 500*time.Millisecond, float64(100*time.Millisecond),
		"the time that it took for goroutines to finish should be close to the open interval")

	close(answer)
	for err := range answer {
		copyErr := err
		assert.NoError(t, copyErr, "no call should have returned an error")
	}
}
