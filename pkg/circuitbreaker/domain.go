package circuitbreaker

type CircuitBreaker interface {
	Call(func() error) error
	Tripped() bool
	Fail()
	Success()
}

type circuitState interface {
	fail() circuitState
	success() circuitState
	blockCall() bool
}
