package s3_test

import (
	"testing"

	"github.com/jademcosta/jiboia/pkg/adapters/objstorage/s3"
	"github.com/stretchr/testify/assert"
)

const configYaml = `
timeout_milliseconds: 1234
bucket: some-s3-bucket-name
region: some-s3-region
endpoint: my-s3-endpoint
access_key: "access s3!"
secret_key: "secret s3!"
`

func TestParseConfig(t *testing.T) {
	s3Config, err := s3.ParseConfig([]byte(configYaml))
	assert.NoError(t, err, "should not return error when parsing s3 config")

	assert.Equal(t, int64(1234), s3Config.TimeoutInMillis, "bucket timeout_milliseconds doesn't match")
	assert.Equal(t, "some-s3-bucket-name", s3Config.Bucket, "bucket name doesn't match")
	assert.Equal(t, "some-s3-region", s3Config.Region, "bucket region doesn't match")
	assert.Equal(t, "my-s3-endpoint", s3Config.Endpoint, "bucket endpoint doesn't match")
	assert.Equal(t, "access s3!", s3Config.AccessKey, "bucket access_key doesn't match")
	assert.Equal(t, "secret s3!", s3Config.SecretKey, "bucket secret_key doesn't match")
}
