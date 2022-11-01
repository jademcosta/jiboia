package sqs

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsSqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"go.uber.org/zap"
)

const TYPE string = "sqs"

type sqsRep struct {
	log      *zap.SugaredLogger
	client   sqsiface.SQSAPI
	queueUrl string
	alias    string
}

func New(l *zap.SugaredLogger, c *config.ExternalQueueConfig) (*sqsRep, error) {
	//TODO: session claims to be safe to read concurrently. Can we use a single one?
	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String(c.Region),
		Endpoint: aws.String(c.Endpoint),
	})
	if err != nil {
		return nil, fmt.Errorf("error creating creating SQS session: %w", err)
	}

	queueUrl := c.URL
	if !validUrl(queueUrl) {
		return nil, fmt.Errorf("invalid url for SQS %s", queueUrl)
	}

	sqsClient := awsSqs.New(sess)

	return &sqsRep{
		log:      l,
		client:   sqsClient,
		queueUrl: queueUrl,
		alias:    "mainFlow", //TODO: make this dynamic, from config
	}, nil
}

func (internalSqs *sqsRep) Enqueue(uploadResult *domain.UploadResult) error {
	//TODO: SQS should not know about "uploadResult"
	message := domain.Message{
		SchemaVersion: domain.SCHEMA_VERSION,
		Bucket: domain.Bucket{
			Name:   uploadResult.Bucket,
			Region: uploadResult.Region,
		},
		Object: domain.Object{
			Path:        uploadResult.Path,
			SizeInBytes: uploadResult.SizeInBytes,
		},
	}

	bodyAsBytes, err := json.Marshal(message)
	body := string(bodyAsBytes)

	if err != nil {
		return err
	}

	enqueueOutput, err := internalSqs.client.SendMessage(&awsSqs.SendMessageInput{
		MessageBody: &body,
		QueueUrl:    &internalSqs.queueUrl,
	}) //TODO: test error case

	if err == nil {
		internalSqs.log.Debug("enqueued message on SQS", "message_id", enqueueOutput.MessageId)
	}

	return err
}

func validUrl(url string) bool {
	return len(url) > 0
	// TODO: validate the format here
}

func (s *sqsRep) Type() string {
	return TYPE
}

func (s *sqsRep) Name() string {
	return s.alias
}
