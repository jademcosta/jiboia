package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProfilingConfigFillDefaults(t *testing.T) {
	sut := ProfilingConfig{}
	sut = sut.fillDefaults()
	assert.False(t, sut.Enabled, "default enabled should be false for ProfilingConfig")
}

func TestProfilingConfigValidate(t *testing.T) {
	sut := ProfilingConfig{}
	assert.NoError(t, sut.validate(), "validate should return no error for ProfilingConfig")
	sut.Enabled = true
	assert.NoError(t, sut.validate(), "validate should return no error for ProfilingConfig")
}
