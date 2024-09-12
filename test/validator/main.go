// nolint: forbidigo
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"

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

var characters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = characters[rand.Intn(len(characters))]
	}
	return string(b)
}

func main() {
	queueURL := flag.String("q", "", "The URL of the queue")
	flag.Parse()

	if *queueURL == "" {
		fmt.Println("You must supply the URL of a queue (-q QUEUE)")
		os.Exit(1)
	}

	expected1 := randSeq(20)
	expected2 := randSeq(25)

	fmt.Println("Sending POST request...")

	response, err := http.Post("http://localhost:9099/jiboia-flow/async_ingestion", "application/json", strings.NewReader(expected1))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	response.Body.Close()

	response, err = http.Post("http://localhost:9099/jiboia-flow/async_ingestion", "application/json", strings.NewReader(expected2))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	response.Body.Close()

	// This is going to be "ignored", as it will stay in memory waiting for more data to flow in
	response, err = http.Post("http://localhost:9099/jiboia-flow/async_ingestion", "application/json", strings.NewReader(randSeq(2)))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	response.Body.Close()

	fmt.Println("Starting validator...")
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
			WaitTimeSeconds:     aws.Int64(20),
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

	expected := fmt.Sprint(expected1, "__n__", expected2)

	if string(buf.Bytes()) != expected {
		fmt.Printf("String inside S3 file is not the expected one. Expected: %s\nGot: %s\n", expected, string(buf.Bytes()))
		os.Exit(1)
	}
	fmt.Println("Expected content is correct!")
	os.Exit(0)
}
