package compressor_test

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/compressor"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/stretchr/testify/assert"
)

func randSeq(n int) string {
	characters := []rune("abcdefghijklmnopqrstuvwxyz")
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]rune, n)

	for i := range b {
		b[i] = characters[r1.Intn(len(characters))]
	}
	return string(b)
}

func TestDecompressionAfterCompressionKeepsDataUnchanged(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dataSize := 5242880 // 500KB
	data := randSeq(dataSize)

	testCases := []struct {
		conf config.Compression
	}{
		{conf: config.Compression{Type: "gzip"}},
		{conf: config.Compression{Type: "gzip", Level: "1"}},
		{conf: config.Compression{Type: "gzip", Level: "9"}},
		{conf: config.Compression{Type: "zlib"}},
		{conf: config.Compression{Type: "zlib", Level: "9"}},
		{conf: config.Compression{Type: "zlib", Level: "1"}},
		{conf: config.Compression{Type: "deflate"}},
		{conf: config.Compression{Type: "deflate", Level: "9"}},
		{conf: config.Compression{Type: "deflate", Level: "1"}},
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

	dataSize := 5242880 // 500KB
	data := randSeq(dataSize)

	conf := config.Compression{}
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
