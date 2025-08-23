package domain

import "context"

const MsgSchemaVersion string = "0.0.1"

type UploadResult struct {
	Bucket      string
	Region      string
	Path        string
	URL         string
	SizeInBytes int
}

type MessageContext struct {
	Bucket          string
	Region          string
	Path            string
	URL             string
	SizeInBytes     int
	CompressionType string
	SavedAt         int64
}

type DataEnqueuer interface {
	Enqueue(*WorkUnit) error
}

// It is the internal representation of data flowing through the flows
type WorkUnit struct {
	Filename string
	Prefix   string
	Data     []byte
	Context  context.Context
}

// FIXME: add tests
func (w *WorkUnit) DataLen() int {
	if w == nil || w.Data == nil {
		return 0
	}
	return len(w.Data)
}

type FilePathProvider interface {
	Filename() *string
	Prefix() *string
}
