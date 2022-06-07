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
	c := &config.Config{
		Log: config.LogConfig{Level: "warn", Format: "json"},
		Flow: config.FlowConfig{
			ExternalQueue: config.ExternalQueue{
				Config: config.ExternalQueueConfig{
					URL:    queueUrl,
					Region: "us-east-1",
				},
			},
		},
	}

	l := logger.New(c)

	sut, err := New(l, &c.Flow.ExternalQueue.Config)
	mockSQS := &mockedSendMsgs{msgs: make([]*sqs.SendMessageInput, 0)}
	sut.client = mockSQS

	if err != nil {
		assert.Fail(t, "failed to create SQS struct")
	}

	sut.Enqueue(&domain.UploadResult{
		Bucket:      "my-bucket1",
		Region:      "region-a",
		Path:        "filepath",
		SizeInBytes: 1111})

	jsonMsg := "{\"schema_version\":\"0.0.1\",\"bucket\":{\"name\":\"my-bucket1\",\"region\":\"region-a\"},\"object\":{\"path\":\"filepath\",\"size_in_bytes\":1111}}"

	expected := &sqs.SendMessageInput{
		QueueUrl:    &queueUrl,
		MessageBody: &jsonMsg}

	assert.Lenf(t, mockSQS.msgs, 1, "1 message should have been sent to SQS client")
	assert.Equal(t, expected, mockSQS.msgs[0], "the correct message format should've been enqueued")
}
