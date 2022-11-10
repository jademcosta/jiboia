package s3

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"go.uber.org/zap"
)

const TYPE string = "s3"

type S3Bucket struct {
	name        string
	region      string
	fixedPrefix string
	uploader    *s3manager.Uploader
	log         *zap.SugaredLogger
}

func New(logger *zap.SugaredLogger, c *config.ObjectStorageConfig) (*S3Bucket, error) {
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
		uploader:    uploader,
		log:         logger,
		name:        c.Bucket,
		region:      c.Region,
		fixedPrefix: c.Prefix}, nil
}

func (bucket *S3Bucket) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {
	key := mergeParts(bucket.fixedPrefix, workU.Prefix, workU.Filename)

	uploadInput := &s3manager.UploadInput{
		Bucket: &bucket.name,
		Key:    &key,
		Body:   bytes.NewReader(workU.Data),
	}

	// TODO: uploader claims to be concurrency-safe
	uploadInfo, err := bucket.uploader.Upload(uploadInput)
	if err != nil {
		return nil, err
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
