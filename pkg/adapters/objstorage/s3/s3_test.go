package s3

import (
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
	c := &Config{}

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

	assert.Len(t, mockUploader.calledWith, 1, "should have called the uploader with 1")

}
