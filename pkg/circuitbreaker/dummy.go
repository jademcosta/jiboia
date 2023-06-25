package circuitbreaker

type DummyCircuitBreaker struct{}

func NewDummyCircuitBreaker() *DummyCircuitBreaker {
	return &DummyCircuitBreaker{}
}

func (cb *DummyCircuitBreaker) Call(f func() error) error {
	return f()
}

func (cb *DummyCircuitBreaker) Tripped() bool {
	return false
}

func (cb *DummyCircuitBreaker) Fail() {
	//Do nothing
}

func (cb *DummyCircuitBreaker) Success() {
	//Do nothing
}
