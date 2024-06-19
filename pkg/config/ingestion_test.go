package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIngestionConfigValidate(t *testing.T) {
	sut := IngestionConfig{}
	sut = sut.fillDefaultValues()

	assert.NoError(t, sut.validate(), "filled with defaults, intestion conf should be valid")
}
