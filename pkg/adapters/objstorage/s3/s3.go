package s3

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"gopkg.in/yaml.v2"
)

const TYPE string = "s3"
const startupTimeout = 20 * time.Second

type s3UploaderAPI interface {
	Upload(context.Context, *s3.PutObjectInput, ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

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

type Bucket struct {
	name            string
	region          string
	fixedPrefix     string
	timeoutInMillis int64
	uploader        s3UploaderAPI
	log             *slog.Logger
}

func New(l *slog.Logger, c *Config) (*Bucket, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), startupTimeout)
	defer cancelFunc()

	//TODO: adapt logger
	sdkConfig, err := config.LoadDefaultConfig(
		ctx, config.WithRegion(c.Region), config.WithBaseEndpoint(c.Endpoint),
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't load default AWS configuration: %w", err)
	}

	s3Cli := s3.NewFromConfig(sdkConfig, func(o *s3.Options) {
		o.UsePathStyle = c.ForcePathStyle
		if c.Endpoint != "" {
			o.BaseEndpoint = &c.Endpoint
		}
	})
	uploader := manager.NewUploader(s3Cli)

	return &Bucket{
		uploader:        uploader,
		log:             l.With(logger.ObjStorageTypeKey, "s3"),
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

func (bucket *Bucket) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {
	key := mergeParts(bucket.fixedPrefix, workU.Prefix, workU.Filename)

	uploadInput := &s3.PutObjectInput{
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

func (bucket *Bucket) Type() string {
	return TYPE
}

func (bucket *Bucket) Name() string {
	return bucket.name
}

func mergeParts(fixedPrefix string, dynamicPrefix string, key string) string {
	//TODO: this is probably not very perf. Explore other ideas.
	result := strings.Trim(fixedPrefix, "/") + "/" + strings.Trim(dynamicPrefix, "/")
	result = strings.Trim(result, "/")

	result = "/" + result + "/" + strings.Trim(key, "/")

	return strings.Trim(result, "/")
}

func (bucket *Bucket) doUpload(input *s3.PutObjectInput) (*manager.UploadOutput, error) {
	hasTimeout := bucket.timeoutInMillis != 0

	ctx := context.Background()
	if hasTimeout {
		ctx2, cancelFunc := context.WithTimeout(ctx, time.Duration(bucket.timeoutInMillis)*time.Millisecond)
		defer cancelFunc()
		ctx = ctx2
	}

	return bucket.uploader.Upload(ctx, input)
}
