package kinesis

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"go.uber.org/zap"
)

//FIXME: this is not working an will not be included on v0
type Config struct {
	region string
	stream string
}

type Kinesis struct {
	log        *zap.SugaredLogger
	client     *kinesis.Kinesis
	streamName string
}

func New(l *zap.SugaredLogger, c *Config) (*Kinesis, error) {
	sess, err := session.NewSession(&aws.Config{Region: &c.region})
	if err != nil {
		return nil, fmt.Errorf("error creating creating Kinesis session: %w", err)
	}

	k := kinesis.New(sess)

	return &Kinesis{
		log:        l,
		client:     k,
		streamName: c.stream,
	}, nil
}

func (iKinesis *Kinesis) Enqueue(dataPath *string) error {
	// _, err := iKinesis.client.PutRecord(&kinesis.PutRecordInput{
	// 	Data:         []byte(*dataPath),
	// 	PartitionKey: dataPath,
	// 	StreamName:   &iKinesis.streamName,
	// })

	// return err
	return nil
}
