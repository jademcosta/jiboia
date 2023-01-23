package s3

import (
	"errors"
	"io/ioutil"
	"strings"
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
	err        error
	answer     *s3manager.UploadOutput
}

func (mock *mockedAWSS3Uploader) Upload(input *s3manager.UploadInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	mock.calledWith = append(mock.calledWith, input)
	if mock.err != nil {
		return nil, mock.err
	}

	if mock.answer != nil {
		return mock.answer, nil
	}
	return &s3manager.UploadOutput{Location: mock.location}, nil
}

func (mock *mockedAWSS3Uploader) UploadWithContext(ctx aws.Context, input *s3manager.UploadInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	mock.calledWith = append(mock.calledWith, input)
	if mock.err != nil {
		return nil, mock.err
	}

	if mock.answer != nil {
		return mock.answer, nil
	}
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

func TestItWorksWithDifferentPrefixConfigs(t *testing.T) {
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})

	type testCase struct {
		fixedPrefix   string
		dynamicPrefix string
		key           string
	}

	testCases := []testCase{
		{fixedPrefix: "fixed", dynamicPrefix: "dynamic", key: "key"},
		{fixedPrefix: "", dynamicPrefix: "", key: "key"},
		{fixedPrefix: "f", dynamicPrefix: "", key: "key"},
		{fixedPrefix: "", dynamicPrefix: "d", key: "key"},
		{fixedPrefix: "", dynamicPrefix: "multi/level/prefix", key: "key"},
		{fixedPrefix: "multi/level/prefix", dynamicPrefix: "", key: "key"},
		{fixedPrefix: "multi/level/prefix", dynamicPrefix: "more/levels", key: "key"},
		{fixedPrefix: "multi/level/prefix/", dynamicPrefix: "more/levels/", key: "key"},
		{fixedPrefix: "/multi/level/prefix/", dynamicPrefix: "/more/levels/", key: "key"},
		{fixedPrefix: "multi/level/prefix", dynamicPrefix: "more/levels////", key: "key"},
	}

	for _, tc := range testCases {
		c := &Config{
			Prefix: tc.fixedPrefix,
		}

		sut, err := New(l, c)
		assert.NoError(t, err, "should not error on New")
		mockUploader := &mockedAWSS3Uploader{calledWith: make([]*s3manager.UploadInput, 0)}
		sut.uploader = mockUploader

		workU := &domain.WorkUnit{
			Filename: tc.key,
			Prefix:   tc.dynamicPrefix,
			Data:     []byte("Some data here"),
		}

		sut.Upload(workU)

		assert.Len(t, mockUploader.calledWith, 1, "should have called the uploader with 1 workUnit")
		input := mockUploader.calledWith[0]

		expected := tc.key
		if tc.dynamicPrefix != "" {
			expected = strings.Trim(tc.dynamicPrefix, "/") + "/" + expected
		}

		if tc.fixedPrefix != "" {
			expected = strings.Trim(tc.fixedPrefix, "/") + "/" + expected
		}

		assert.Equal(t, expected, *input.Key, "the file key should be built using prefixes and filename from work unit")
	}
}

func TestReturnsTheUploadError(t *testing.T) {
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})
	c := &Config{Bucket: "some_bucket_name"}

	uploadErr := errors.New("some random error")
	mockUploader := &mockedAWSS3Uploader{calledWith: make([]*s3manager.UploadInput, 0), err: uploadErr}

	sut, err := New(l, c)
	assert.NoError(t, err, "should not error on New")

	sut.uploader = mockUploader

	workU := &domain.WorkUnit{
		Filename: "my_filename",
		Prefix:   "a/rand/om/prefix",
		Data:     []byte("A data for input"),
	}

	_, err = sut.Upload(workU)

	assert.Len(t, mockUploader.calledWith, 1, "should have called the uploader with 1 workUnit")
	assert.Error(t, err, "should return an error")
	assert.ErrorIs(t, err, uploadErr, "should return the error the S3 dependency returned, wrapped")
	assert.ErrorContains(t, err, "some random error", "should return the error the S3 dependency returned, wrapped")
}

func TestReturnsDataBasedOnUploadReturn(t *testing.T) {
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})
	c := &Config{Bucket: "some_bucket_name", Region: "my_region", Prefix: "mypref"}

	mockUploader := &mockedAWSS3Uploader{
		calledWith: make([]*s3manager.UploadInput, 0),
		answer:     &s3manager.UploadOutput{Location: "some_location"},
	}

	sut, err := New(l, c)
	assert.NoError(t, err, "should not error on New")

	sut.uploader = mockUploader

	workU := &domain.WorkUnit{
		Filename: "my_filename",
		Prefix:   "a/rand/om/prefix",
		Data:     []byte("A data for input"),
	}

	upResult, err := sut.Upload(workU)

	assert.Len(t, mockUploader.calledWith, 1, "should have called the uploader with 1 workUnit")
	assert.NoError(t, err, "should not return an error")

	assert.Equal(t, "some_location", upResult.URL, "should have the uploadOutput Location field as URL field")
	assert.Equal(t, "my_region", upResult.Region, "should have the region we provide in config as upload output region")
	assert.Equal(t, "some_bucket_name", upResult.Bucket, "should have the bucket name we provide in config as upload bucket")
	assert.Equal(t, 16, upResult.SizeInBytes, "should calculate the size in bytes based on the data we passed to Upload")
	assert.Equal(t, "mypref/a/rand/om/prefix/my_filename", upResult.Path, "should return the same path the object was uploaded to")
}

// TODO: Test the timeout
