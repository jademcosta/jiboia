package compressor

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zlib"
	"github.com/klauspost/compress/zstd"

	"github.com/jademcosta/jiboia/pkg/config"
)

const (
	GzipType    = "gzip"
	ZlibType    = "zlib"
	DeflateType = "deflate"
	SnappyType  = "snappy"
	ZstdType    = "zstd"
)

type closerAdapter struct {
	wrapped io.Reader
}

func (adapt *closerAdapter) Close() error {
	return nil
}

func (adapt *closerAdapter) Read(p []byte) (int, error) {
	return adapt.wrapped.Read(p)
}

type CompressorReader interface {
	io.Reader
	io.Closer
}

type CompressorWriter interface {
	io.WriteCloser
	Flush() error
}

func NewReader(
	conf *config.CompressionConfig,
	reader io.Reader,
) (CompressorReader, error) {

	var decompressor CompressorReader
	var err error
	switch strings.ToLower(conf.Type) {
	case GzipType:
		decompressor, err = gzip.NewReader(reader)
	case ZlibType:
		decompressor, err = zlib.NewReader(reader)
	case DeflateType:
		decompressor = flate.NewReader(reader)
	case SnappyType:
		decompressor = &closerAdapter{wrapped: snappy.NewReader(reader)}
	case ZstdType:
		d, localErr := zstd.NewReader(reader)
		err = localErr
		if localErr == nil {
			decompressor = &closerAdapter{wrapped: d}
		}
	case "":
		decompressor = NewNoopCompressorReader(reader)
	default:
		decompressor = nil
		err = fmt.Errorf("invalid compression type %s", conf.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating %s reader: %w", conf.Type, err)
	}

	return decompressor, nil
}

func NewWriter(
	conf *config.CompressionConfig,
	writer io.Writer,
) (CompressorWriter, error) {

	var compressor CompressorWriter
	var err error

	levelSet := conf.Level != ""
	var level int
	if levelSet {
		level, err = strconv.Atoi(conf.Level)
		if err != nil {
			return nil, fmt.Errorf("invalid compression level %s: %w", conf.Level, err)
		}
	}

	switch strings.ToLower(conf.Type) {
	case GzipType:
		if levelSet {
			compressor, err = gzip.NewWriterLevel(writer, level)
		} else {
			compressor = gzip.NewWriter(writer)
		}
	case ZlibType:
		if levelSet {
			compressor, err = zlib.NewWriterLevel(writer, level)
		} else {
			compressor = zlib.NewWriter(writer)
		}
	case DeflateType:
		if levelSet {
			compressor, err = flate.NewWriter(writer, level)
		} else {
			compressor, err = flate.NewWriter(writer, flate.DefaultCompression)
		}
	case SnappyType:
		compressor = snappy.NewBufferedWriter(writer)
	case ZstdType:
		if levelSet {
			opts := zstd.WithEncoderLevel(zstd.EncoderLevel(level))
			//TODO: level works differently on this package, we should not allow so many levels
			compressor, err = zstd.NewWriter(writer, opts)
		} else {
			compressor, err = zstd.NewWriter(writer)
		}
	case "":
		compressor = NewNoopCompressorWriter(writer)
	default:
		compressor = nil
		err = fmt.Errorf("invalid compression type %s", conf.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating %s reader: %w", conf.Type, err)
	}

	return compressor, nil
}
