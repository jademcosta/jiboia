package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressionFillDefaults(t *testing.T) {
	sut := CompressionConfig{}

	sut = sut.fillDefaultValues()
	assert.Equal(t, 2, sut.PreallocSlicePercentage, "default value of prealloc slice percentage is 2")
}

func TestCompressionValidate(t *testing.T) {
	sut := CompressionConfig{}
	require.NoError(t, sut.validate(),
		"empty conf should be valid, as it is the case where no compression is used")

	sut.Type = "aaaaa"
	sut.PreallocSlicePercentage = 4
	require.Error(t, sut.validate(),
		"compression type should be one of allowed values")

	for _, compType := range []string{"gzip", "zlib", "deflate", "zstd", "snappy"} {
		sut.Type = compType
		require.NoError(t, sut.validate(),
			"%s should be a valid compression type", compType)
	}

	sut.PreallocSlicePercentage = -1
	require.Error(t, sut.validate(),
		"prealloc should be > 0")

	sut.PreallocSlicePercentage = 0
	require.Error(t, sut.validate(),
		"prealloc should be > 0")

	sut.PreallocSlicePercentage = 101
	require.Error(t, sut.validate(),
		"prealloc should be < 100")

	sut.PreallocSlicePercentage = 100
	sut.Level = "7"
	require.NoError(t, sut.validate(),
		"conf should be valid")
}
