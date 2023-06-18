package circuitbreaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUsesConfigValues(t *testing.T) {
	type testCase struct {
		config   map[string]string
		expected time.Duration
	}

	testCases := []testCase{
		{map[string]string{"turn_on": "true", "open_interval_in_ms": "1234"}, 1234 * time.Millisecond},
		{map[string]string{"turn_on": "true", "open_interval_in_ms": "101"}, 101 * time.Millisecond},
		{map[string]string{"open_interval_in_ms": "101"}, 101 * time.Millisecond},
		{map[string]string{"open_interval_in_ms": "101", "anykey232": "any value"}, 101 * time.Millisecond},
		{map[string]string{"open_interval_in_ms": "11", "anykey232": "any value"}, 11 * time.Millisecond},
		{map[string]string{"open_interval_in_ms": "1", "anykey232": "any value"}, 1 * time.Millisecond},
		{map[string]string{"open_interval_in_ms": "99", "anykey232": "any value"}, 99 * time.Millisecond},
		{map[string]string{"open_interval_in_ms": "123456", "anykey232": "any value"}, 123456 * time.Millisecond},
		{map[string]string{"turn_on": "true", "anykey": "anyval", "open_interval_in_ms": "1234"}, 1234 * time.Millisecond},
	}

	for _, tc := range testCases {
		cb, err := FromConfig(tc.config)
		sequentialCB := cb.(*SequentialCircuitBreaker)
		state := sequentialCB.cState.(*circuitClosedState)
		assert.Equalf(t, tc.expected, state.conf.OpenInterval, "The default Open interval should be %v", tc.expected)
		assert.NoError(t, err, "%v should return no error", tc.config)
	}
}

func TestDefaultValuesOfSequentialCB(t *testing.T) {
	type testCase struct {
		config map[string]string
	}

	testCases := []testCase{
		{nil},
		{make(map[string]string)},
		{map[string]string{}},
		{map[string]string{"anykey": "anyval"}},
		{map[string]string{"turn_on": "true"}},
		{map[string]string{"turn_on": "true", "anykey": "anyval"}},
	}

	defaultOpenInterval := 100 * time.Millisecond

	for _, tc := range testCases {
		cb, err := FromConfig(tc.config)
		sequentialCB := cb.(*SequentialCircuitBreaker)
		state := sequentialCB.cState.(*circuitClosedState)
		assert.NoError(t, err, "%v should return no error", tc.config)
		assert.Equalf(t, defaultOpenInterval, state.conf.OpenInterval, "The default Open interval should be %v", defaultOpenInterval)
	}
}
