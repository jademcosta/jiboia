package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type Message struct {
	SchemaVersion string `json:"schema_version"`
	Bucket        Bucket `json:"bucket"`
	Object        Object `json:"object"`
}

type Object struct {
	Path        string `json:"path"`
	SizeInBytes int    `json:"size_in_bytes"`
}

type Bucket struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

func main() {
	queueURL := flag.String("q", "", "The URL of the queue")
	flag.Parse()

	if *queueURL == "" {
		fmt.Println("You must supply the URL of a queue (-q QUEUE)")
		os.Exit(1)
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String("us-east-1"),
		},
	}))

	svc := sqs.New(sess)

	msgResult, err := svc.ReceiveMessage(&sqs.ReceiveMessageInput{
		QueueUrl:            queueURL,
		MaxNumberOfMessages: aws.Int64(1),
	})

	if err != nil {
		fmt.Println("Error getting message from SQS: ", err)
		os.Exit(1)
	}

	if len(msgResult.Messages) < 1 {
		fmt.Println("No message returned from SQS")
		os.Exit(1)
	}

	message := msgResult.Messages[0]

	content := &Message{}
	err = json.Unmarshal([]byte(*message.Body), content)

	if err != nil {
		fmt.Println("Failed to parse the SQS body JSON: ", err)
		os.Exit(1)
	}

	expectedBucketName := os.Getenv("JIBOIA_S3_BUCKET")
	if content.Bucket.Name != expectedBucketName {
		fmt.Println("Expected bucket name to be ", expectedBucketName, " but was ", content.Bucket.Name)
		os.Exit(1)
	}

	// if *message.Body != "{\"key1\":\"first-key\", \"key2\":\"another-key!!!\"}" {
	// 	fmt.Println("Message received different from expected: ", *message.Body)
	// 	os.Exit(1)
	// }

	os.Exit(0)
}
