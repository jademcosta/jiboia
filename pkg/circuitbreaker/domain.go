package circuitbreaker

type CircuitBreaker interface {
	Call(func() error) error
	Tripped() bool
	Fail()
	Success()
}
