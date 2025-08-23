package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDescompressionConfigFillDefaults(t *testing.T) {
	sut := DescompressionConfig{}
	sut = sut.fillDefaultValues()
	assert.Equal(t, "512", sut.InitialBufferSize, "initial buffer size should be filled with default value")
}

func TestDescompressionConfigValidate(t *testing.T) {
	sut := DescompressionConfig{}
	require.NoError(t, sut.validate(), "should be valid if active decompressions is nil")

	sut.ActiveDecompressions = []string{"gzip"}
	require.Error(t, sut.validate(), "should not be valid if initial buffer size is empty")

	sut.InitialBufferSize = "2kb"
	require.NoError(t, sut.validate(), "should be valid")

	sut.MaxConcurrency = -1
	require.Error(t, sut.validate(), "max concurrency cannot be < 0")

	sut.MaxConcurrency = 3
	sut.ActiveDecompressions = []string{"gzip", "non-existent"}
	require.Error(t, sut.validate(), "active decompression should be one from the allowed list")

	sut.ActiveDecompressions = []string{"gzip", "zstd"}
	require.NoError(t, sut.validate(), "should be valid")

	for _, decompType := range []string{"gzip", "zlib", "deflate", "zstd", "snappy"} {
		sut.ActiveDecompressions = []string{decompType}
		require.NoError(t, sut.validate(),
			"%s should be a valid decompression type", decompType)
	}
}

func TestDescompressionConfigInitialBufferSizeAsBytes(t *testing.T) {
	testCases := []struct {
		input    string
		expected int64
	}{
		{input: "4", expected: 4},
		{input: "345kb", expected: 345 * 1024},
		{input: "1gb", expected: 1 * 1024 * 1024 * 1024},
	}

	for _, tc := range testCases {

		sut := DescompressionConfig{InitialBufferSize: tc.input}
		result, err := sut.InitialBufferSizeAsBytes()
		require.NoError(t, err, "should return no error")
		assert.Equal(t, tc.expected, result,
			"should have returned %d for input %s", tc.expected, tc.input)
	}
}
