package circuitbreaker

import "errors"

var ErrorOpenCircuitBreaker error = errors.New("circuit breaker is open")
