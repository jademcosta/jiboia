package sqs

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsSqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"gopkg.in/yaml.v2"
)

const TYPE string = "sqs"

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

type Config struct {
	URL       string `yaml:"url"`
	Region    string `yaml:"region"`
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type Queue struct {
	log      *slog.Logger
	client   sqsiface.SQSAPI
	queueURL string
	flowName string
}

func New(l *slog.Logger, c *Config, flowName string) (*Queue, error) {
	//TODO: session claims to be safe to read concurrently. Can I use a single one?
	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String(c.Region),
		Endpoint: aws.String(c.Endpoint),
		LogLevel: aws.LogLevel(aws.LogDebug),
		Logger: aws.LoggerFunc(func(args ...interface{}) {
			l.Debug("AWS sdk log", "aws-msg", fmt.Sprint(args))
		}),
	})

	if err != nil {
		return nil, fmt.Errorf("error creating creating SQS session: %w", err)
	}

	queueURL := c.URL
	if !validURL(queueURL) {
		return nil, fmt.Errorf("invalid url for SQS %s", queueURL)
	}

	sqsClient := awsSqs.New(sess)

	return &Queue{
		log:      l.With(logger.ExternalQueueTypeKey, "sqs"),
		client:   sqsClient,
		queueURL: queueURL,
		flowName: flowName,
	}, nil
}

func ParseConfig(confData []byte) (*Config, error) {
	conf := &Config{}

	err := yaml.Unmarshal(confData, conf)
	if err != nil {
		return conf, fmt.Errorf("error parsing SQS config: %w", err)
	}

	return conf, nil
}

func (internalSqs *Queue) Enqueue(msg *domain.MessageContext) error {
	message := Message{
		SchemaVersion: domain.MsgSchemaVersion,
		FlowName:      internalSqs.Name(),
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

	bodyAsBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	body := string(bodyAsBytes)

	messageInput := &awsSqs.SendMessageInput{
		MessageBody: &body,
		QueueUrl:    &internalSqs.queueURL,
	}

	err = messageInput.Validate()
	if err != nil {
		return fmt.Errorf("error on SQS message validation: %w", err)
	}

	internalSqs.log.Debug("sending SQS message", "queue_url", internalSqs.queueURL)
	enqueueOutput, err := internalSqs.client.SendMessage(messageInput)
	if err == nil {
		internalSqs.log.Debug("enqueued message on SQS", "message_id", enqueueOutput.MessageId)
	}

	return err
}

func validURL(url string) bool {
	return len(url) > 0
	// TODO: validate the format here
}

func (internalSqs *Queue) Type() string {
	return TYPE
}

func (internalSqs *Queue) Name() string {
	return internalSqs.flowName
}
