package adapters

import (
	"path/filepath"

	"github.com/jademcosta/jiboia/pkg/domain"
)

func ContentEncodingFromFileName(fileName string) string {
	extension := getExtension(fileName)

	switch extension {
	case domain.CompressionGzipType:
		return "gzip"
	case domain.CompressionZstdType:
		return "zstd"
	case domain.CompressionSnappyType:
		return "snappy"
	case domain.CompressionDeflateType:
		return "deflate"
	case domain.CompressionZlibType:
		return "zlib"
	default:
		return ""
	}
}

func getExtension(fileName string) string {
	ext := filepath.Ext(fileName)
	if len(ext) > 0 {
		return ext[1:] // remove the dot
	}
	return ""
}
