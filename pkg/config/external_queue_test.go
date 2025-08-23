package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalQueueConfigFillDefaultValues(t *testing.T) {
	sut := ExternalQueueConfig{}
	sut = sut.fillDefaultValues()
	assert.Equal(t, "noop", sut.Type, "default type is `noop`")
}

func TestExternalQueueConfigValidate(t *testing.T) {
	sut := ExternalQueueConfig{}

	err := sut.validate()
	require.Error(t, err, "empty type is invalid")

	sut.Type = "anything"
	err = sut.validate()
	require.Error(t, err, "random type type is invalid")

	for _, queueType := range []string{"noop", "sqs"} {
		sut.Type = queueType
		err := sut.validate()
		assert.NoError(t, err, "empty type %s is valid")
	}
}
