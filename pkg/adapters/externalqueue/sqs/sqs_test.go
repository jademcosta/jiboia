package sqs

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
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

const fakeFlowName = "some-flow-name"

type mockedSendMsgs struct {
	msgs      []*sqs.SendMessageInput
	returnFor func(*sqs.SendMessageInput) *sqs.SendMessageOutput
	err       error
}

func (mock *mockedSendMsgs) SendMessage(
	_ context.Context,
	input *sqs.SendMessageInput,
	_ ...func(*sqs.Options),
) (*sqs.SendMessageOutput, error) {
	mock.msgs = append(mock.msgs, input)

	if mock.err != nil {
		return nil, mock.err
	}

	if mock.returnFor != nil {
		return mock.returnFor(input), nil
	}

	return &sqs.SendMessageOutput{}, nil
}
func TestMessageContainsTheData(t *testing.T) {
	queueURL := "some-queue-url"
	c := &Config{
		URL:    queueURL,
		Region: "us-east-1",
	}
	flowName := "this-flow-name"
	l := logger.NewDummy()

	sut, err := New(l, c, flowName)
	mockSQS := &mockedSendMsgs{msgs: make([]*sqs.SendMessageInput, 0)}
	sut.client = mockSQS

	if err != nil {
		assert.Fail(t, "failed to create SQS struct")
	}

	err = sut.Enqueue(&domain.MessageContext{
		Bucket:          "my-bucket1",
		Region:          "region-a",
		Path:            "filepath",
		URL:             "some_url",
		SizeInBytes:     1111,
		CompressionType: "some-compression"})
	assert.NoError(t, err, "should not err on enqueue")

	jsonMsg := "{\"schema_version\":\"0.0.1\",\"flow_name\":\"this-flow-name\",\"bucket\":{\"name\":\"my-bucket1\",\"region\":\"region-a\"},\"object\":{\"path\":\"filepath\",\"full_url\":\"some_url\",\"size_in_bytes\":1111,\"compression_algorithm\":\"some-compression\"}}"

	expected := &sqs.SendMessageInput{
		QueueUrl:    &queueURL,
		MessageBody: &jsonMsg}

	assert.Lenf(t, mockSQS.msgs, 1, "1 message should have been sent to SQS client")
	assert.Equal(t, expected, mockSQS.msgs[0], "the correct message format should've been enqueued")
}

func TestReturnsTheErrorOnEnqueueingError(t *testing.T) {
	queueURL := "some-queue-url"
	c := &Config{
		URL:    queueURL,
		Region: "us-east-1",
	}

	l := logger.NewDummy()

	sut, err := New(l, c, fakeFlowName)
	mockErr := errors.New("mock error")
	mockSQS := &mockedSendMsgs{msgs: make([]*sqs.SendMessageInput, 0), err: mockErr}
	sut.client = mockSQS

	if err != nil {
		assert.Fail(t, "failed to create SQS struct")
	}

	err = sut.Enqueue(
		&domain.MessageContext{
			Bucket:          "my-bucket1",
			Region:          "region-a",
			Path:            "filepath",
			URL:             "some_url",
			SizeInBytes:     1111,
			CompressionType: "some-compression"},
	)
	assert.Error(t, err, "should have error on enqueue")

	assert.Lenf(t, mockSQS.msgs, 1, "1 message should have been sent to SQS client")
	assert.Same(t, mockErr, err, "the underlying SQS error should have been sent as return")
}

func TestParseConfig(t *testing.T) {
	confResult, err := ParseConfig([]byte(configYaml))

	assert.NoError(t, err, "should not return error from config parsing")
	assert.Equal(t, "sqs-queue-url-here", confResult.URL, "queue URL doesn't match")
	assert.Equal(t, "aws-sqs-region-here", confResult.Region, "queue region doesn't match")
	assert.Equal(t, "access sqs!", confResult.AccessKey, "queue access_key doesn't match")
	assert.Equal(t, "secret sqs!", confResult.SecretKey, "queue secret_key doesn't match")
}
