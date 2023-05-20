package circuitbreaker_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/stretchr/testify/assert"
)

func alwaysSuccessfulFn() error {
	return nil
}

func alwaysFailFn() error {
	return fmt.Errorf("always an error")
}

func TestABrandNewCBStartsWithClosedState(t *testing.T) {
	sut := circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval: 1000 * time.Millisecond,
		})

	for i := 0; i <= 5; i++ {
		err := sut.Call(alwaysSuccessfulFn)
		assert.NoError(t, err, "should return no error")
	}
}

func TestSuccessCallsOnClosedStateKeepItClosed(t *testing.T) {
	sut := circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       60000 * time.Millisecond,
			FailCountThreshold: 1,
		})

	sut.Success()

	err := sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error after success calls")

	for i := 0; i < 11; i++ {
		sut.Success()
	}
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error after several success calls")

	for i := 0; i < 11; i++ {
		_ = sut.Call(alwaysSuccessfulFn)
	}
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error after several success calls")
}

func TestOneSuccessResetsTheFailStreak(t *testing.T) {
	sut := circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       60000 * time.Millisecond,
			FailCountThreshold: 11,
		})

	for i := 0; i < 10; i++ {
		sut.Fail()
	}

	sut.Success()
	sut.Fail()
	sut.Fail()
	sut.Fail()
	err := sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")

	sut = circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       60000 * time.Millisecond,
			FailCountThreshold: 11,
		})

	for i := 0; i < 10; i++ {
		_ = sut.Call(alwaysFailFn)
	}

	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
	_ = sut.Call(alwaysFailFn)
	_ = sut.Call(alwaysFailFn)
	_ = sut.Call(alwaysFailFn)
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
}

func TestBecomesClosedAfterThreshold(t *testing.T) {
	interval := 100 * time.Millisecond
	sut := circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       interval,
			FailCountThreshold: 1,
		})

	sut.Fail()
	err := sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "should return error")
	time.Sleep(interval)
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")

	sut = circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       interval,
			FailCountThreshold: 1,
		})

	_ = sut.Call(alwaysFailFn)

	err = sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "should return error")
	time.Sleep(interval)
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
}

func TestErrorTypeIsOpenCircuitBreaker(t *testing.T) {
	sut := circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       6000 * time.Millisecond,
			FailCountThreshold: 3,
		})

	sut.Fail()
	sut.Fail()
	err := sut.Call(alwaysSuccessfulFn) //Resets the fail counter
	assert.NoError(t, err, "should return no error")
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")

	sut.Fail()
	sut.Fail()
	sut.Fail()
	err = sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "an CB with threshold of 1 (and enough time of open interval) should return error after a failure ")
	assert.ErrorIs(t, err, circuitbreaker.ErrorOpenCircuitBreaker, "error type should be ErrorCircuitBreakerOpen")

	sut = circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       60000 * time.Millisecond,
			FailCountThreshold: 3,
		})

	_ = sut.Call(alwaysFailFn)
	_ = sut.Call(alwaysFailFn)
	err = sut.Call(alwaysSuccessfulFn) // Resets the counter
	assert.NoError(t, err, "should return no error")
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")

	_ = sut.Call(alwaysFailFn)
	_ = sut.Call(alwaysFailFn)
	_ = sut.Call(alwaysFailFn)
	err = sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "an CB with threshold of 1 (and enough time of open interval) should return error after a failure ")
	assert.ErrorIs(t, err, circuitbreaker.ErrorOpenCircuitBreaker, "error type should be ErrorCircuitBreakerOpen")
}

func TestItCanCloseAndOpenMiultipleTimes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test test")
	}
	interval := 100 * time.Millisecond
	sut := circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       interval,
			FailCountThreshold: 1,
		})

	for i := 0; i <= 37; i++ {
		sut.Fail()
		err := sut.Call(alwaysSuccessfulFn)
		assert.Error(t, err, "should return error")
		time.Sleep(interval)
		err = sut.Call(alwaysSuccessfulFn)
		assert.NoError(t, err, "should return no error")
	}

	for i := 0; i <= 37; i++ {
		_ = sut.Call(alwaysFailFn)

		err := sut.Call(alwaysSuccessfulFn)
		assert.Error(t, err, "should return error")
		time.Sleep(interval)
		err = sut.Call(alwaysSuccessfulFn)
		assert.NoError(t, err, "should return no error")
	}
}

func TestWhenOpenSuccessCallMakesClosed(t *testing.T) {
	sut := circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       30 * time.Second,
			FailCountThreshold: 1,
		})

	sut.Fail()
	err := sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "should return error")

	sut.Success()
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")

	_ = sut.Call(alwaysFailFn)
	err = sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "should return error")
	sut.Success()
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
}

func TestCallingFailWhenOpenIncreasesThreshold(t *testing.T) {
	sut := circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       100 * time.Millisecond,
			FailCountThreshold: 1,
		})

	sut.Fail()
	err := sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "should return error")
	time.Sleep(60 * time.Millisecond)

	sut.Fail()
	err = sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "should return error")
	time.Sleep(70 * time.Millisecond)
	err = sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "should return error")
	time.Sleep(31 * time.Millisecond)
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")

	// The same doesn't happen when is a fn call
	_ = sut.Call(alwaysFailFn)
	err = sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "should return error")
	time.Sleep(60 * time.Millisecond)
	err = sut.Call(alwaysSuccessfulFn)
	assert.Error(t, err, "should return error")
	time.Sleep(41 * time.Millisecond)
	err = sut.Call(alwaysSuccessfulFn)
	assert.NoError(t, err, "should return no error")
}

func TestTripped(t *testing.T) {
	sut := circuitbreaker.NewSequentialCircuitBreaker(
		circuitbreaker.SequentialCircuitBreakerConfig{
			OpenInterval:       100 * time.Millisecond,
			FailCountThreshold: 1,
		})

	assert.False(t, sut.Tripped(), "should not be tripped")
	_ = sut.Call(alwaysSuccessfulFn)
	assert.False(t, sut.Tripped(), "should not be tripped")
	sut.Success()
	assert.False(t, sut.Tripped(), "should not be tripped")

	sut.Fail()
	assert.True(t, sut.Tripped(), "should be tripped")
	time.Sleep(101 * time.Millisecond)
	assert.False(t, sut.Tripped(), "should not be tripped")
}
