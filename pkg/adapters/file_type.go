package adapters

import (
	"path/filepath"

	"github.com/jademcosta/jiboia/pkg/domain"
)

func ContentEncodingFromFileName(fileName string) string {
	extension := getExtension(fileName)

	switch extension {
	case domain.GzipType:
		return "gzip"
	case domain.ZstdType:
		return "zstd"
	case domain.SnappyType:
		return "snappy"
	case domain.DeflateType:
		return "deflate"
	case domain.ZlibType:
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
