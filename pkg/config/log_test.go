package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogConfigDefaultValues(t *testing.T) {
	sut := LogConfig{}
	sut = sut.fillDefaults()

	assert.Equal(t, "info", sut.Level, "default log level should be info")
}

func TestLogConfigValidate(t *testing.T) {
	sut := LogConfig{}
	sut = sut.fillDefaults()

	assert.NoError(t, sut.validate(), "filled with default values, should be valid")

	sut.Level = "aaa"
	assert.Error(t, sut.validate(), "should be invalid if level is outside of a group of values")

	for _, lvl := range []string{"debug", "info", "warn", "error"} {
		sut.Level = lvl
		assert.NoError(t, sut.validate(), "should be invalid if level is inside of a group of values")
	}
}
