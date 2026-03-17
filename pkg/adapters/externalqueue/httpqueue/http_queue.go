package httpqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jademcosta/jiboia/pkg/adapters/externalqueue/queuemessage"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"gopkg.in/yaml.v2"
)

const TYPE string = "http"
const defaultTimeoutMilliseconds = 30000

type Config struct {
	URL                 string `yaml:"url"`
	TimeoutMilliseconds int    `yaml:"timeout_milliseconds"`
}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Queue struct {
	log      *slog.Logger
	client   httpClient
	url      string
	flowName string
}

func New(l *slog.Logger, c *Config, flowName string) (*Queue, error) {
	if c.URL == "" {
		return nil, fmt.Errorf("url is required for HTTP queue")
	}

	timeoutMs := c.TimeoutMilliseconds
	if timeoutMs <= 0 {
		timeoutMs = defaultTimeoutMilliseconds
	}

	client := &http.Client{
		Timeout: time.Duration(timeoutMs) * time.Millisecond,
	}

	return &Queue{
		log:      l.With(logger.ExternalQueueTypeKey, TYPE),
		client:   client,
		url:      c.URL,
		flowName: flowName,
	}, nil
}

func ParseConfig(confData []byte) (*Config, error) {
	conf := &Config{}

	err := yaml.Unmarshal(confData, conf)
	if err != nil {
		return conf, fmt.Errorf("error parsing HTTP queue config: %w", err)
	}

	return conf, nil
}

func (q *Queue) Enqueue(msg *domain.MessageContext) error {
	message := queuemessage.NewMessage(q.Name(), msg)

	bodyBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("error marshaling HTTP queue message: %w", err)
	}

	//TODO: once we start using context everywhere, remove the creation of a new one here
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, q.url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	q.log.Debug("sending HTTP queue message", "url", q.url)

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending HTTP queue message: %w", err)
	}
	defer resp.Body.Close()

	failedRequestStatus := resp.StatusCode < 200 || resp.StatusCode >= 300
	if failedRequestStatus {
		return fmt.Errorf("HTTP queue endpoint returned non-2xx status: %d", resp.StatusCode)
	}

	q.log.Debug("HTTP queue message sent successfully", "status_code", resp.StatusCode)

	return nil
}

func (q *Queue) Type() string {
	return TYPE
}

func (q *Queue) Name() string {
	return q.flowName
}
