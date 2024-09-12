package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApiFillDefaults(t *testing.T) {
	sut := APIConfig{}
	sut = sut.fillDefaults()

	assert.Equal(t, 9199, sut.Port, "default port is 9199")
}

func TestApiValidate(t *testing.T) {
	sut := APIConfig{}
	err := sut.validate()
	assert.Error(t, err, "port cannot be zero")

	sut.Port = 1234
	err = sut.validate()
	assert.NoError(t, err, "should not error")

	sut.PayloadSizeLimit = "abc"
	err = sut.validate()
	assert.Error(t, err, "payload size limit needs to be data unit")

	sut.PayloadSizeLimit = "123abc"
	err = sut.validate()
	assert.Error(t, err, "payload size limit needs to be data unit")

	sut.PayloadSizeLimit = "123"
	err = sut.validate()
	assert.NoError(t, err, "should not error")

	sut.PayloadSizeLimit = "123mb"
	err = sut.validate()
	assert.NoError(t, err, "should not error")

	sut.PayloadSizeLimit = "123kb"
	err = sut.validate()
	assert.NoError(t, err, "should not error")

	sut.PayloadSizeLimit = "123gb"
	err = sut.validate()
	assert.NoError(t, err, "should not error")
}
