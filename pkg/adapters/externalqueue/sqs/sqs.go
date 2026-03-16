package sqs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	awsSqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/jademcosta/jiboia/pkg/adapters/externalqueue/queuemessage"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"gopkg.in/yaml.v2"
)

const TYPE string = "sqs"
const startupTimeout = 20 * time.Second

type sqsSendMessageAPI interface {
	SendMessage(context.Context, *awsSqs.SendMessageInput, ...func(*awsSqs.Options)) (*awsSqs.SendMessageOutput, error)
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
	client   sqsSendMessageAPI
	queueURL string
	flowName string
}

func New(l *slog.Logger, c *Config, flowName string) (*Queue, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), startupTimeout)
	defer cancelFunc()

	//TODO: adapt logger
	sdkConfig, err := config.LoadDefaultConfig(
		ctx, config.WithRegion(c.Region), config.WithBaseEndpoint(c.Endpoint),
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't load default AWS configuration: %w", err)
	}

	queueURL := c.URL
	if !validURL(queueURL) {
		return nil, fmt.Errorf("invalid url for SQS %s", queueURL)
	}

	sqsClient := awsSqs.NewFromConfig(sdkConfig)

	return &Queue{
		log:      l.With(logger.ExternalQueueTypeKey, "sqs"),
		client:   sqsClient,
		queueURL: queueURL,
		flowName: flowName,
	}, nil
}

func (internalSqs *Queue) Enqueue(msg *domain.MessageContext) error {
	//TODO: allow to config timeout
	ctx := context.Background()

	message := queuemessage.NewMessage(internalSqs.Name(), msg)

	bodyAsBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	body := string(bodyAsBytes)

	messageInput := &awsSqs.SendMessageInput{
		MessageBody: &body,
		QueueUrl:    &internalSqs.queueURL,
	}

	internalSqs.log.Debug("sending SQS message", "queue_url", internalSqs.queueURL)
	enqueueOutput, err := internalSqs.client.SendMessage(ctx, messageInput)
	if err == nil {
		internalSqs.log.Debug("enqueued message on SQS", "message_id", enqueueOutput.MessageId)
	}

	return err
}

func ParseConfig(confData []byte) (*Config, error) {
	conf := &Config{}

	err := yaml.Unmarshal(confData, conf)
	if err != nil {
		return conf, fmt.Errorf("error parsing SQS config: %w", err)
	}

	return conf, nil
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
