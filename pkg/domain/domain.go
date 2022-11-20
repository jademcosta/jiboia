package domain

const MESSAGE_SCHEMA_VERSION string = "0.0.1"

type UploadResult struct {
	Bucket      string
	Region      string
	Path        string
	URL         string
	SizeInBytes int
}

type Message struct {
	SchemaVersion string `json:"schema_version"`
	Bucket        Bucket `json:"bucket"`
	Object        Object `json:"object"`
}

type Object struct {
	Path        string `json:"path"`
	FullURL     string `json:"full_url"`
	SizeInBytes int    `json:"size_in_bytes"`
}

type Bucket struct {
	Name   string `json:"name"`
	Region string `json:"region"`
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
