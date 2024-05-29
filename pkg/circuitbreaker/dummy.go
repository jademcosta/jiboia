package circuitbreaker

// A Circuitbreaker that accepts all requests and never trips. Useful in cases where you want to
// not have a CB.
type DummyTwoStepCircuitBreaker struct{}

func NewDummyTwoStepCircuitBreaker() *DummyTwoStepCircuitBreaker {
	return &DummyTwoStepCircuitBreaker{}
}

func (cb *DummyTwoStepCircuitBreaker) Allow() (func(bool), error) {
	return func(_ bool) {}, nil
}

// A Circuitbreaker that accepts all requests and never trips. Useful in cases where you want to
// not have a CB.
type DummyCircuitBreaker struct{}

func NewDummyCircuitBreaker() *DummyCircuitBreaker {
	return &DummyCircuitBreaker{}
}

func (cb *DummyCircuitBreaker) Execute(f func() (interface{}, error)) (interface{}, error) {
	return f()
}
