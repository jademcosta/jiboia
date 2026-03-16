package queuemessage_test

import (
	"testing"

	"github.com/jademcosta/jiboia/pkg/adapters/externalqueue/queuemessage"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/stretchr/testify/assert"
)

func TestNewMessageMapsAllFieldsFromMessageContext(t *testing.T) {
	msgContext := &domain.MessageContext{
		Bucket:          "my-bucket",
		Region:          "us-east-1",
		Path:            "some/path/file.txt",
		URL:             "https://my-bucket.s3.amazonaws.com/some/path/file.txt",
		SizeInBytes:     1234,
		CompressionType: "gzip",
	}

	msg := queuemessage.NewMessage("my-flow", msgContext)

	assert.Equal(t, domain.MsgSchemaVersion, msg.SchemaVersion)
	assert.Equal(t, "my-flow", msg.FlowName)
	assert.Equal(t, msgContext.Bucket, msg.Bucket.Name)
	assert.Equal(t, msgContext.Region, msg.Bucket.Region)
	assert.Equal(t, msgContext.Path, msg.Object.Path)
	assert.Equal(t, msgContext.URL, msg.Object.FullURL)
	assert.Equal(t, msgContext.SizeInBytes, msg.Object.SizeInBytes)
	assert.Equal(t, msgContext.CompressionType, msg.Object.CompressionType)
}
