package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type Message struct {
	Bucket Bucket `json:"bucket"`
	Object Object `json:"object"`
}

type Object struct {
	Path string `json:"path"`
}

type Bucket struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

func main() {
	queueURL := flag.String("q", "", "The URL of the queue")
	expected := flag.String("e", "", "Expected content of the file")
	flag.Parse()

	if *queueURL == "" {
		fmt.Println("You must supply the URL of a queue (-q QUEUE)")
		os.Exit(1)
	}

	fmt.Println("Starting validator...")
	fmt.Println("Expected content: ", expected)
	fmt.Println("Important: This test does not works well if running it in parallel other instance of itself. It expects a single message on SQS!")

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String("us-east-1"),
		},
	}))

	svc := sqs.New(sess)

	msgResult, err := svc.ReceiveMessage(
		&sqs.ReceiveMessageInput{
			QueueUrl:            queueURL,
			MaxNumberOfMessages: aws.Int64(1),
			WaitTimeSeconds:     aws.Int64(15),
		},
	)

	if err != nil {
		fmt.Println("Error getting message from SQS: ", err)
		os.Exit(1)
	}

	if len(msgResult.Messages) < 1 {
		fmt.Println("No message returned from SQS")
		os.Exit(1)
	}

	fmt.Printf("Get %d messages from SQS\n", len(msgResult.Messages))

	message := msgResult.Messages[0]
	fmt.Println("The first message from SQS is: ", message)

	_, err = svc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      queueURL,
		ReceiptHandle: message.ReceiptHandle,
	})

	if err != nil {
		fmt.Println("Error deleting the message from queue. This might generate future runs of this test to fail. Err:  ", err)
		os.Exit(1)
	}

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

	downloader := s3manager.NewDownloader(sess)
	buf := aws.NewWriteAtBuffer([]byte{})
	_, err = downloader.Download(buf,
		&s3.GetObjectInput{
			Bucket: aws.String(content.Bucket.Name),
			Key:    aws.String(content.Object.Path),
		})

	if err != nil {
		fmt.Println("Failed to download the S3 file, err: ", err)
		os.Exit(1)
	}

	fmt.Println("Downloaded file content: ", string(buf.Bytes()))

	if string(buf.Bytes()) != *expected {
		fmt.Printf("String inside S3 file is not the expected one. Expected: %s\nGot: %s\n", *expected, string(buf.Bytes()))
		os.Exit(1)
	} else {
		fmt.Println("Expected content is correct!")
		os.Exit(0)
	}
}
