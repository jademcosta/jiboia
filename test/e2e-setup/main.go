package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
)

func main() {
	queue := flag.String("q", "", "The name of the queue")
	flag.Parse()

	if *queue == "" {
		fmt.Println("You must supply a queue name (-q QUEUE")
		return
	}

	fmt.Printf("Creating queue %s\n", *queue)
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String("http://localhost:4566"),
		S3ForcePathStyle: aws.Bool(true),
	})

	if err != nil {
		fmt.Println("Failed to create AWS session: ", err)
		os.Exit(1)
	}

	sqsSvc := sqs.New(sess)

	_, err = sqsSvc.CreateQueue(&sqs.CreateQueueInput{
		QueueName: queue,
		Attributes: map[string]*string{
			"DelaySeconds":           aws.String("10"),
			"MessageRetentionPeriod": aws.String("120"),
		},
	})

	if err != nil {
		fmt.Println("Failed to create queue: ", err)
		os.Exit(1)
	}

	s3Svc := s3.New(sess)

	_, err = s3Svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String("jiboia-local-bucket"),
	})

	if err != nil {
		fmt.Println("Failed to create bucket: ", err)
		os.Exit(1)
	}

	// err = s3Svc.WaitUntilBucketExists(&s3.HeadBucketInput{
	// 	Bucket: aws.String("jiboia-local-bucket"),
	// })

	// if err != nil {
	// 	fmt.Println("Failed to create bucket")
	// 	os.Exit(1)
	// }

	os.Exit(0)
}
