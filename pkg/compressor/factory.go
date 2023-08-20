package compressor

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	GZIP_TYPE    = "gzip"
	ZLIB_TYPE    = "zlib"
	DEFLATE_TYPE = "deflate"
)

type CompressorReader interface {
	io.ReadCloser
}

type CompressorWriter interface {
	io.WriteCloser
}

func NewReader(
	l *zap.SugaredLogger,
	metricRegistry *prometheus.Registry,
	conf *config.Compression,
	reader io.Reader,
) (CompressorReader, error) {

	var compressor CompressorReader
	var err error
	switch strings.ToLower(conf.Type) {
	case GZIP_TYPE:
		compressor, err = gzip.NewReader(reader)
	case ZLIB_TYPE:
		compressor, err = zlib.NewReader(reader)
	case DEFLATE_TYPE:
		compressor = flate.NewReader(reader)
	case "":
		compressor = NewNoopCompressorReader(reader)
	default:
		compressor = nil
		err = fmt.Errorf("invalid compression type %s", conf.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating %s reader: %w", conf.Type, err)
	}

	return compressor, nil
}

func NewWriter(
	conf *config.Compression,
	writer io.Writer,
) (CompressorWriter, error) {

	var compressor CompressorWriter
	var err error

	levelSet := conf.Level != ""
	level, err := strconv.Atoi(conf.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid compression level %s: %w", conf.Level, err)
	}

	switch strings.ToLower(conf.Type) {
	case GZIP_TYPE:
		if levelSet {
			compressor, err = gzip.NewWriterLevel(writer, level)
		} else {
			compressor = gzip.NewWriter(writer)
		}
	case ZLIB_TYPE:
		if levelSet {
			compressor, err = zlib.NewWriterLevel(writer, level)
		} else {
			compressor = zlib.NewWriter(writer)
		}
	case DEFLATE_TYPE:
		if levelSet {
			compressor, err = flate.NewWriter(writer, level)
		} else {
			compressor, err = flate.NewWriter(writer, flate.DefaultCompression)
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
