package adapters

import (
	"path/filepath"

	"github.com/jademcosta/jiboia/pkg/domain"
)

func ContentTypeFromFileName(fileName string) string {
	extension := getExtension(fileName)

	switch extension {
	case domain.GzipType:
		return "application/gzip"
	case domain.ZstdType:
		return "application/zstd"
	case domain.SnappyType:
		return "application/x-snappy"
	case domain.DeflateType:
		return "application/zlib"
	case domain.ZlibType:
		return "application/zlib"
	default:
		return "application/octet-stream"
	}
}

func getExtension(fileName string) string {
	ext := filepath.Ext(fileName)
	if len(ext) > 0 {
		return ext[1:] // remove the dot
	}
	return ""
}
