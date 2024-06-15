package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescompressionConfigFillDefaults(t *testing.T) {
	sut := DescompressionConfig{}
	sut = sut.fillDefaultValues()
	assert.Equal(t, "512", sut.InitialBufferSize, "initial buffer size should be filled with default value")
}

func TestDescompressionConfigValidate(t *testing.T) {
	sut := DescompressionConfig{}
	assert.NoError(t, sut.validate(), "should be valid if active decompressions is nil")

	sut.ActiveDecompressions = []string{"gzip"}
	assert.Error(t, sut.validate(), "should not be valid if initial buffer size is empty")

	sut.InitialBufferSize = "2kb"
	assert.NoError(t, sut.validate(), "should be valid")

	sut.MaxConcurrency = -1
	assert.Error(t, sut.validate(), "max concurrency cannot be < 0")

	sut.MaxConcurrency = 3
	sut.ActiveDecompressions = []string{"gzip", "non-existent"}
	assert.Error(t, sut.validate(), "active decompression should be one from the allowed list")

	sut.ActiveDecompressions = []string{"gzip", "zstd"}
	assert.NoError(t, sut.validate(), "should be valid")

	for _, decompType := range []string{"gzip", "zlib", "deflate", "zstd", "snappy"} {
		sut.ActiveDecompressions = []string{decompType}
		assert.NoError(t, sut.validate(),
			"%s should be a valid decompression type", decompType)
	}
}
