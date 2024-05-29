package s3

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"gopkg.in/yaml.v2"
)

const TYPE string = "s3"

type Config struct {
	TimeoutInMillis int64  `yaml:"timeout_milliseconds"`
	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	Endpoint        string `yaml:"endpoint"`
	AccessKey       string `yaml:"access_key"`
	SecretKey       string `yaml:"secret_key"`
	Prefix          string `yaml:"prefix"`
	ForcePathStyle  bool   `yaml:"force_path_style"`
}

type S3Bucket struct {
	name            string
	region          string
	fixedPrefix     string
	timeoutInMillis int64
	uploader        s3manageriface.UploaderAPI
	log             *slog.Logger
}

func New(l *slog.Logger, c *Config) (*S3Bucket, error) {
	//TODO: the session is safe to be read concurrently, can we use a single one?

	// TODO: expore the configs:
	// HTTPClient *http.Client
	// LogLevel *LogLevelType
	// Logger Logger
	session, err := session.NewSession(&aws.Config{
		Region:           aws.String(c.Region),
		Endpoint:         aws.String(c.Endpoint),
		S3ForcePathStyle: aws.Bool(c.ForcePathStyle),
	})

	if err != nil {
		return nil, fmt.Errorf("error creating S3 session: %w", err)
	}

	// TODO: configure concurrency on the uploader
	uploader := s3manager.NewUploader(session)

	return &S3Bucket{
		uploader:        uploader,
		log:             l.With(logger.OBJ_STORAGE_TYPE_KEY, "s3"),
		name:            c.Bucket,
		region:          c.Region,
		fixedPrefix:     c.Prefix,
		timeoutInMillis: c.TimeoutInMillis,
	}, nil
}

func ParseConfig(confData []byte) (*Config, error) {
	conf := &Config{}

	err := yaml.Unmarshal(confData, conf)
	if err != nil {
		return conf, fmt.Errorf("error parsing S3 config: %w", err)
	}

	return conf, nil
}

func (bucket *S3Bucket) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {
	key := mergeParts(bucket.fixedPrefix, workU.Prefix, workU.Filename)

	uploadInput := &s3manager.UploadInput{
		Bucket: &bucket.name,
		Key:    &key,
		Body:   bytes.NewReader(workU.Data),
	}

	uploadInfo, err := bucket.doUpload(uploadInput)
	if err != nil {
		return nil, fmt.Errorf("error when uploading to S3: %w", err)
	}

	result := &domain.UploadResult{
		Bucket:      bucket.name,
		Region:      bucket.region,
		Path:        key,
		URL:         uploadInfo.Location,
		SizeInBytes: len(workU.Data),
	}

	return result, nil
}

func (bucket *S3Bucket) Type() string {
	return TYPE
}

func (bucket *S3Bucket) Name() string {
	return bucket.name
}

func mergeParts(fixedPrefix string, dynamicPrefix string, key string) string {
	//TODO: this is probably not very perf. Explore other ideas.
	result := strings.Trim(fixedPrefix, "/") + "/" + strings.Trim(dynamicPrefix, "/")
	result = strings.Trim(result, "/")

	result = "/" + result + "/" + strings.Trim(key, "/")

	return strings.Trim(result, "/")
}

func (bucket *S3Bucket) doUpload(input *s3manager.UploadInput) (*s3manager.UploadOutput, error) {
	// TODO: uploader claims to be concurrency-safe
	hasTimeout := bucket.timeoutInMillis != 0

	if hasTimeout {
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(bucket.timeoutInMillis)*time.Millisecond)
		defer cancelFunc()
		return bucket.uploader.UploadWithContext(ctx, input)
	} else {
		return bucket.uploader.Upload(input)
	}
}
