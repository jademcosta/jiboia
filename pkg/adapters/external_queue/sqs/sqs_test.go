package sqs

import (
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
)

const configYaml = `
url: sqs-queue-url-here
region: aws-sqs-region-here
access_key: "access sqs!"
secret_key: "secret sqs!"
`

type mockedSendMsgs struct {
	sqsiface.SQSAPI
	msgs      []*sqs.SendMessageInput
	mu        sync.Mutex
	returnFor func(*sqs.SendMessageInput) *sqs.SendMessageOutput
}

func (mock *mockedSendMsgs) SendMessage(input *sqs.SendMessageInput) (*sqs.SendMessageOutput, error) {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	mock.msgs = append(mock.msgs, input)

	if mock.returnFor != nil {
		return mock.returnFor(input), nil
	}

	return &sqs.SendMessageOutput{}, nil
}
func TestMessageContainsTheData(t *testing.T) {
	queueUrl := "some-queue-url"
	c := &Config{
		URL:    queueUrl,
		Region: "us-east-1",
	}

	l := logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})

	sut, err := New(l, c)
	mockSQS := &mockedSendMsgs{msgs: make([]*sqs.SendMessageInput, 0)}
	sut.client = mockSQS

	if err != nil {
		assert.Fail(t, "failed to create SQS struct")
	}

	err = sut.Enqueue(&domain.UploadResult{
		Bucket:      "my-bucket1",
		Region:      "region-a",
		Path:        "filepath",
		URL:         "some_url",
		SizeInBytes: 1111})
	assert.NoError(t, err, "should not err on enqueue")

	jsonMsg := "{\"schema_version\":\"0.0.1\",\"bucket\":{\"name\":\"my-bucket1\",\"region\":\"region-a\"},\"object\":{\"path\":\"filepath\",\"full_url\":\"some_url\",\"size_in_bytes\":1111}}"

	expected := &sqs.SendMessageInput{
		QueueUrl:    &queueUrl,
		MessageBody: &jsonMsg}

	assert.Lenf(t, mockSQS.msgs, 1, "1 message should have been sent to SQS client")
	assert.Equal(t, expected, mockSQS.msgs[0], "the correct message format should've been enqueued")
}

func TestParseConfig(t *testing.T) {
	confResult, err := ParseConfig([]byte(configYaml))

	assert.NoError(t, err, "should not return error from config parsing")
	assert.Equal(t, "sqs-queue-url-here", confResult.URL, "queue URL doesn't match")
	assert.Equal(t, "aws-sqs-region-here", confResult.Region, "queue region doesn't match")
	assert.Equal(t, "access sqs!", confResult.AccessKey, "queue access_key doesn't match")
	assert.Equal(t, "secret sqs!", confResult.SecretKey, "queue secret_key doesn't match")
}
