package circuitbreaker

import (
	"fmt"
	"strconv"
	"time"
)

const DEFAULT_OPEN_INTERVAL_DURATION = 100 * time.Millisecond
const TURN_ON_KEY = "turn_on"
const OPEN_INTERVAL_KEY = "open_interval_in_ms"

func FromConfig(cbConfig map[string]string) (CircuitBreaker, error) {
	noConfig := len(cbConfig) == 0 || cbConfig == nil
	if noConfig {
		return defaultCircuitBreaker(), nil
	}

	turnOnVal, turnOnDefined := cbConfig[TURN_ON_KEY]
	if turnOnDefined {
		turnOn, err := strconv.ParseBool(turnOnVal)
		if err != nil {
			return nil, fmt.Errorf("%s key has a non boolean value", TURN_ON_KEY)
		}

		if !turnOn {
			return NewDummyCircuitBreaker(), nil
		}
	}

	return createSequentialCircuitBreaker(cbConfig)
}

func defaultCircuitBreaker() *SequentialCircuitBreaker {
	return NewSequentialCircuitBreaker(
		SequentialCircuitBreakerConfig{
			FailCountThreshold: 1,
			OpenInterval:       DEFAULT_OPEN_INTERVAL_DURATION,
		},
	)
}

func createSequentialCircuitBreaker(cbConf map[string]string) (*SequentialCircuitBreaker, error) {

	intervalVal, intervalDefined := cbConf[OPEN_INTERVAL_KEY]
	var interval time.Duration = DEFAULT_OPEN_INTERVAL_DURATION

	if intervalDefined {
		intervalInMs, err := strconv.Atoi(intervalVal)
		if err != nil {
			return nil, fmt.Errorf("%s key has a non integer value", OPEN_INTERVAL_KEY)
		}

		if intervalInMs == 0 {
			//TODO: this validation should be moved to the CB itself
			return nil, fmt.Errorf("%s key cannot be zero", OPEN_INTERVAL_KEY)
		}

		interval = time.Duration(intervalInMs) * time.Millisecond
	}

	return NewSequentialCircuitBreaker(
		SequentialCircuitBreakerConfig{
			FailCountThreshold: 1,
			OpenInterval:       interval,
		},
	), nil
}
