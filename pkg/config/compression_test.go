package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompressionFillDefaults(t *testing.T) {
	sut := CompressionConfig{}

	sut = sut.fillDefaultValues()
	assert.Equal(t, 2, sut.PreallocSlicePercentage, "default value of prealloc slice percentage is 2")
}

func TestCompressionValidate(t *testing.T) {
	sut := CompressionConfig{}
	assert.NoError(t, sut.validate(),
		"empty conf should be valid, as it is the case where no compression is used")

	sut.Type = "aaaaa"
	sut.PreallocSlicePercentage = 4
	assert.Error(t, sut.validate(),
		"compression type should be one of allowed values")

	for _, compType := range []string{"gzip", "zlib", "deflate", "lzw", "zstd", "snappy"} {
		sut.Type = compType
		assert.NoError(t, sut.validate(),
			"%s should be a valid compression type", compType)
	}

	sut.PreallocSlicePercentage = -1
	assert.Error(t, sut.validate(),
		"prealloc should be > 0")

	sut.PreallocSlicePercentage = 0
	assert.Error(t, sut.validate(),
		"prealloc should be > 0")

	sut.PreallocSlicePercentage = 101
	assert.Error(t, sut.validate(),
		"prealloc should be < 100")

	sut.PreallocSlicePercentage = 100
	sut.Level = "7"
	assert.NoError(t, sut.validate(),
		"conf should be valid")
}
