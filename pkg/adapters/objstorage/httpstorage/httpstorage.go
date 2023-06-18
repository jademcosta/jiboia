package httpstorage

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jademcosta/jiboia/pkg/domain"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

const TYPE string = "httpstorage"

type Config struct {
	Url string `yaml:"url"`
}

type HttpStorage struct {
	url    string
	log    *zap.SugaredLogger
	client *http.Client
}

func New(l *zap.SugaredLogger, c *Config) (*HttpStorage, error) {
	url, err := validateAndFormatUrl(c.Url)
	if err != nil {
		return nil, fmt.Errorf("error creating httpstorage: %w", err)
	}

	client := &http.Client{
		Timeout: 60 * time.Second, //TODO: allow to be configured
		Transport: &http.Transport{
			MaxConnsPerHost: 10,               //FIXME: this needs to be the size of workers
			IdleConnTimeout: 10 * time.Second, //TODO: is this reasonable?
		},
	}

	return &HttpStorage{url: url, log: l, client: client}, nil
}

func ParseConfig(confData []byte) (*Config, error) {
	conf := &Config{}

	err := yaml.Unmarshal(confData, conf)
	if err != nil {
		return conf, fmt.Errorf("error parsing httpstorage config: %w", err)
	}

	return conf, nil
}

func (storage *HttpStorage) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {

	url, path := assembleUrl(storage.url, workU.Prefix, workU.Filename)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(workU.Data))
	if err != nil {
		return nil, fmt.Errorf("error creating http request: %w", err)
	}

	resp, err := storage.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error doing http request: %w", err)
	}
	//TODO: Do I need to read the whole body to be able to use keep-alive?
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return &domain.UploadResult{
		Bucket:      "httpstorage",
		Path:        path,
		URL:         url,
		SizeInBytes: len(workU.Data),
	}, nil
}

func (storage *HttpStorage) Type() string {
	return "httpstorage"
}

func (storage *HttpStorage) Name() string {
	return "httpstorage"
}

func validateAndFormatUrl(url string) (string, error) {

	if !strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "https") {
		return "", fmt.Errorf("the url should start with http or https")
	}

	multiplePlaceholders := strings.Count(url, "%s") > 1
	if multiplePlaceholders {
		return "", fmt.Errorf("multiple %%s detected on URL, only 1 is allowed")
	}

	placeholderNotPreceededBySlash :=
		strings.Contains(url, "%s") && !strings.Contains(url, "/%s")

	if placeholderNotPreceededBySlash {
		return "", fmt.Errorf("the %%s should be preceeded by a / on URL")
	}

	return url, nil
}

func assembleUrl(url string, prefix string, filename string) (string, string) {
	if !strings.Contains(url, "%s") {
		return url, ""
	}

	url = strings.TrimPrefix(url, "/")
	prefix = strings.Trim(prefix, "/")
	filename = strings.Trim(filename, "/")

	path := fmt.Sprintf("%s/%s", prefix, filename)
	url = strings.Replace(url, "%s", path, 1)

	return url, path
}
