package circuitbreaker

import (
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const DEFAULT_OPEN_INTERVAL_DURATION = 100 * time.Millisecond
const TURN_ON_KEY = "turn_on"
const OPEN_INTERVAL_KEY = "open_interval_in_ms"
const FIXED_FAIL_COUNT_THRESHOLD = 1

func FromConfig(log *zap.SugaredLogger, registry *prometheus.Registry, cbConfig map[string]string, name string, flow string) (CircuitBreaker, error) {
	noConfig := len(cbConfig) == 0 || cbConfig == nil
	if noConfig {
		defaultCircuitBreaker := NewSequentialCircuitBreaker(
			SequentialCircuitBreakerConfig{
				FailCountThreshold: FIXED_FAIL_COUNT_THRESHOLD,
				OpenInterval:       DEFAULT_OPEN_INTERVAL_DURATION,
			},
			createCBO11y(log, registry, name, flow),
		)
		return defaultCircuitBreaker, nil
	}

	turnOnVal, turnOnDefined := cbConfig[TURN_ON_KEY]
	if turnOnDefined {
		turnOn, err := strconv.ParseBool(turnOnVal)
		if err != nil {
			return nil, fmt.Errorf("%s key has a non boolean value", TURN_ON_KEY)
		}

		if !turnOn {
			log.Warnf("circuit breaker not being used on specific case", "name", name, "flow", flow)
			return NewDummyCircuitBreaker(), nil
		}
	}

	return createSequentialCircuitBreaker(log, registry, cbConfig, name, flow)
}

func createSequentialCircuitBreaker(log *zap.SugaredLogger, registry *prometheus.Registry, cbConf map[string]string, name string, flow string) (*SequentialCircuitBreaker, error) {

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
			FailCountThreshold: FIXED_FAIL_COUNT_THRESHOLD,
			OpenInterval:       interval,
		},
		createCBO11y(log, registry, name, flow),
	), nil
}

func createCBO11y(log *zap.SugaredLogger, registry *prometheus.Registry, name string, flow string) *CBObservability {
	return NewObservability(registry, log, name, flow)
}
