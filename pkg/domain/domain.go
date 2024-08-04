package domain

const MESSAGE_SCHEMA_VERSION string = "0.0.1"

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

type DataFlow interface {
	// TODO: change this to Send, and the interface name to something related, like data receiver, or something like that
	Enqueue([]byte) error
}

type WorkUnit struct {
	Filename string
	Prefix   string
	Data     []byte
}

type FilePathProvider interface {
	Filename() *string
	Prefix() *string
}
