package compressor_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/jademcosta/jiboia/pkg/compressor"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestDecompressionAfterCompressionKeepsDataUnchanged(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	data := strings.Repeat("a", 20480) // 20KB
	dataSize := len(data)

	testCases := []struct {
		conf config.CompressionConfig
	}{
		{conf: config.CompressionConfig{Type: "gzip"}},
		{conf: config.CompressionConfig{Type: "gzip", Level: "1"}},
		{conf: config.CompressionConfig{Type: "gzip", Level: "9"}},
		{conf: config.CompressionConfig{Type: "zlib"}},
		{conf: config.CompressionConfig{Type: "zlib", Level: "9"}},
		{conf: config.CompressionConfig{Type: "zlib", Level: "1"}},
		{conf: config.CompressionConfig{Type: "deflate"}},
		{conf: config.CompressionConfig{Type: "deflate", Level: "9"}},
		{conf: config.CompressionConfig{Type: "deflate", Level: "1"}},
		{conf: config.CompressionConfig{Type: "snappy"}},
		{conf: config.CompressionConfig{Type: "snappy", Level: "9"}},
		{conf: config.CompressionConfig{Type: "snappy", Level: "1"}},
	}

	for _, tc := range testCases {
		buf := &bytes.Buffer{}

		compressorWriter, err := compressor.NewWriter(&tc.conf, buf)
		assert.NoError(t, err, "compression writer creation should return no error")

		_, err = compressorWriter.Write([]byte(data))
		assert.NoError(t, err, "compression writer Write call should return no error")
		err = compressorWriter.Close()
		assert.NoError(t, err, "compression writer Close call should return no error")

		compressedData := buf.Bytes()
		assert.NotEqual(t, []byte(data), compressedData,
			"the compressed data should be different from the original")

		compressorReader, err := compressor.NewReader(&tc.conf, buf)
		assert.NoError(t, err, "compression reader creation should return no error")

		result, err := io.ReadAll(compressorReader)
		assert.NoError(t, err, "compression reader Read should return no error")

		assert.Equal(t, dataSize, len(result), "the decompression result should have the same size as the original")
		assert.Equal(t, data, string(result), "the decompression result be the same as the original")
	}
}

func TestEmptyConfigDoesNotApplyCompression(t *testing.T) {

	data := strings.Repeat("a", 20480) // 20KB
	dataSize := len(data)

	conf := config.CompressionConfig{}
	buf := &bytes.Buffer{}

	compressorWriter, err := compressor.NewWriter(&conf, buf)
	assert.NoError(t, err, "compression writer creation should return no error")

	_, err = compressorWriter.Write([]byte(data))
	assert.NoError(t, err, "compression writer Write call should return no error")
	err = compressorWriter.Close()
	assert.NoError(t, err, "compression writer Close call should return no error")

	compressedData := buf.Bytes()
	assert.Equal(t, []byte(data), compressedData,
		"the compressed data should be equal to the original")
	assert.Equal(t, dataSize, len(compressedData), "the compressed data should have the same size as the original")

	compressorReader, err := compressor.NewReader(&conf, buf)
	assert.NoError(t, err, "compression reader creation should return no error")

	result, err := io.ReadAll(compressorReader)
	assert.NoError(t, err, "compression reader Read should return no error")

	assert.Equal(t, dataSize, len(result), "the decompression result should have the same size as the original")
	assert.Equal(t, data, string(result), "the decompression result be the same as the original")
}

//TODO: teste that unrecognized compression leads to error
