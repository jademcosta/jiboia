package s3

import (
	"io/ioutil"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
)

type mockedAWSS3Uploader struct {
	calledWith []*s3manager.UploadInput
	location   string
}

func (mock *mockedAWSS3Uploader) Upload(input *s3manager.UploadInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	mock.calledWith = append(mock.calledWith, input)
	return &s3manager.UploadOutput{Location: mock.location}, nil
}

func (mock *mockedAWSS3Uploader) UploadWithContext(ctx aws.Context, input *s3manager.UploadInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	mock.calledWith = append(mock.calledWith, input)
	return &s3manager.UploadOutput{Location: mock.location}, nil
}

func TestItParsesWorkUnitIntoUploadInput(t *testing.T) {
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})
	c := &Config{Bucket: "some_bucket_name"}

	sut, err := New(l, c)
	assert.NoError(t, err, "should not error on New")
	mockUploader := &mockedAWSS3Uploader{calledWith: make([]*s3manager.UploadInput, 0)}
	sut.uploader = mockUploader

	workU := &domain.WorkUnit{
		Filename: "my_filename",
		Prefix:   "a/rand/om/prefix",
		Data:     []byte("A data for input"),
	}

	sut.Upload(workU)

	assert.Len(t, mockUploader.calledWith, 1, "should have called the uploader with 1 workUnit")
	input := mockUploader.calledWith[0]

	buf, err := ioutil.ReadAll(input.Body)
	assert.NoError(t, err, "reading the sent body should not error")
	assert.Equal(t, workU.Data, buf, "the data sent to S3 should be equals to the one in workUnit")

	assert.Equal(t, workU.Prefix+"/"+workU.Filename, *input.Key, "the file key should be built using prefixes and filename from work unit")

	assert.Equal(t, c.Bucket, *input.Bucket, "the bucket name should be the same from config")
}
