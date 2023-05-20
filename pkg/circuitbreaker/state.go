package circuitbreaker

type circuitState interface {
	fail() circuitState
	success() circuitState
	blockCall() bool
}
