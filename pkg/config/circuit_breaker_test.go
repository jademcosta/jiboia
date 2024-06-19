package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCBFillDefaults(t *testing.T) {
	sut := CircuitBreakerConfig{}
	sut = sut.fillDefaultValues()

	assert.Equal(t, int64(100), sut.OpenInterval, "default interval is 100")
}

func TestCBOpenIntervalAsDuration(t *testing.T) {
	sut := CircuitBreakerConfig{OpenInterval: 234}
	assert.Equal(t, time.Duration(234)*time.Millisecond, sut.OpenIntervalAsDuration(),
		"open interval as duration should return the time as the equivalent millisecond duration")
}

func TestCBValidate(t *testing.T) {
	sut := CircuitBreakerConfig{}

	assert.Error(t, sut.validate(), "open interval cannot be zero")

	sut.OpenInterval = 1
	assert.NoError(t, sut.validate(), "should not error")
}
