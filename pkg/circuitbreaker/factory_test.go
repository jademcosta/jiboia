package circuitbreaker_test

import (
	"reflect"
	"testing"

	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/stretchr/testify/assert"
)

func TestFromConfigReturnsTheCorrectTypes(t *testing.T) {
	dummy := &circuitbreaker.DummyCircuitBreaker{}
	sequential := &circuitbreaker.SequentialCircuitBreaker{}

	type testCase struct {
		config       map[string]string
		expectedType interface{}
	}

	testCases := []testCase{
		{map[string]string{"turn_on": "false"}, dummy},
		{map[string]string{"turn_on": "false", "open_interval_in_ms": "1234"}, dummy},
		{map[string]string{"turn_on": "false", "open_interval_in_ms": "1234", "anykey": "anyval"}, dummy},
		{map[string]string{"turn_on": "false", "open_interval_in_ms": "wrong type", "anykey": "anyval"}, dummy},
		{nil, sequential},
		{make(map[string]string), sequential},
		{map[string]string{}, sequential},
		{map[string]string{"anykey": "anyval"}, sequential},
		{map[string]string{"turn_on": "true", "open_interval_in_ms": "1234"}, sequential},
		{map[string]string{"turn_on": "true", "open_interval_in_ms": "100"}, sequential},
		{map[string]string{"turn_on": "true"}, sequential},
		{map[string]string{"open_interval_in_ms": "100"}, sequential},
		{map[string]string{"open_interval_in_ms": "100", "anykey232": "any value"}, sequential},
		{map[string]string{"turn_on": "true", "anykey": "anyval"}, sequential},
	}

	for _, tc := range testCases {
		result, err := circuitbreaker.FromConfig(tc.config)
		assert.IsType(t, tc.expectedType, result,
			"when config is %v the CB should be %v", tc.config,
			reflect.TypeOf(tc.expectedType))
		assert.NoError(t, err, "%v should return no error", tc.config)
	}
}

func TestErrorsOnZeroInterval(t *testing.T) {

	conf := map[string]string{"open_interval_in_ms": "0"}
	_, err := circuitbreaker.FromConfig(conf)
	assert.Errorf(t, err, "should return error when interval is zero")
}

func TestErrorsWhenKeysDoNotHaveCorrectTypes(t *testing.T) {

	type testCase struct {
		config map[string]string
	}

	testCases := []testCase{
		{map[string]string{"turn_on": ""}},
		{map[string]string{"turn_on": "aaaaaa"}},
		{map[string]string{"turn_on": "2"}},
		{map[string]string{"open_interval_in_ms": ""}},
		{map[string]string{"open_interval_in_ms": "aaaaaa"}},
		{map[string]string{"open_interval_in_ms": "true"}},
		{map[string]string{"open_interval_in_ms": "1e3"}},
	}

	for _, tc := range testCases {
		_, err := circuitbreaker.FromConfig(tc.config)
		assert.Errorf(t, err,
			"should return error when config is %v", tc.config)
	}
}
