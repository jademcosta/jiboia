package queuemessage

import "github.com/jademcosta/jiboia/pkg/domain"

func NewMessage(flowName string, msg *domain.MessageContext) Message {
	return Message{
		SchemaVersion: domain.MsgSchemaVersion,
		FlowName:      flowName,
		Bucket: Bucket{
			Name:   msg.Bucket,
			Region: msg.Region,
		},
		Object: Object{
			Path:            msg.Path,
			FullURL:         msg.URL,
			SizeInBytes:     msg.SizeInBytes,
			CompressionType: msg.CompressionType,
		},
	}
}

type Message struct {
	SchemaVersion string `json:"schema_version"`
	FlowName      string `json:"flow_name"`
	Bucket        Bucket `json:"bucket"`
	Object        Object `json:"object"`
}

type Object struct {
	Path            string `json:"path"`
	FullURL         string `json:"full_url"`
	SizeInBytes     int    `json:"size_in_bytes"`
	CompressionType string `json:"compression_algorithm,omitempty"`
}

type Bucket struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}
