package circuitbreaker

type TwoStepCircuitBreaker interface {
	Allow() (func(bool), error)
}

type CircuitBreaker interface {
	Execute(func() (interface{}, error)) (interface{}, error)
}
