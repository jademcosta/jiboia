package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlowConfigFillDefaults(t *testing.T) {
	sut := FlowConfig{}
	sut = sut.fillDefaultValues()

	assert.Equal(t, 50, sut.MaxConcurrentUploads, "default value for max uploads is 50")
	assert.Equal(t, 1, sut.PathPrefixCount, "default value for path prefix is 1")
}

func TestFlowConfigValidate(t *testing.T) {
	sut := FlowConfig{
		Name:         "somename",
		QueueMaxSize: 5,
	}

	sut = sut.fillDefaultValues()
	assert.NoError(t, sut.validate(), "should be valid")

	sut.Name = "some naa mee"
	assert.Error(t, sut.validate(), "name should not have spaces")

	sut.Name = ""
	assert.Error(t, sut.validate(), "name cannot be empty")

	sut.Name = "some"
	sut.PathPrefixCount = 0
	assert.Error(t, sut.validate(), "path prefix count cannot be zero")
}
