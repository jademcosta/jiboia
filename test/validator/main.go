// nolint: forbidigo
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/jademcosta/jiboia/pkg/compression"
	"github.com/jademcosta/jiboia/pkg/config"
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

	ctx, cancelFunc := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelFunc()

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

	sdkConfig, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion("us-east-1"))
	if err != nil {
		fmt.Println("couldn't load default AWS configuration")
		fmt.Println(err)
		os.Exit(1)
	}

	queueReader := sqs.NewFromConfig(sdkConfig)
	downloader := s3.NewFromConfig(sdkConfig)

	msgResult, err := queueReader.ReceiveMessage(
		ctx,
		&sqs.ReceiveMessageInput{
			QueueUrl:            queueURL,
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     20,
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

	fmt.Printf("Get %d message(s) from SQS\n", len(msgResult.Messages))

	message := msgResult.Messages[0]
	fmt.Println("The first message from SQS is: ", message)

	_, err = queueReader.DeleteMessage(
		ctx,
		&sqs.DeleteMessageInput{
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

	result, err := downloader.GetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(content.Bucket.Name),
			Key:    aws.String(content.Object.Path),
		})
	if err != nil {
		fmt.Println("Failed to download the S3 file, err: ", err)
		os.Exit(1)
	}
	defer result.Body.Close()

	downloadedContent, err := io.ReadAll(result.Body)
	if err != nil {
		fmt.Println("Failed to read the S3 file content, err: ", err)
		os.Exit(1)
	}

	decompressor, err := compression.NewReader(&config.CompressionConfig{Type: compression.GzipType}, bytes.NewReader(downloadedContent))
	if err != nil {
		fmt.Println("Failed to create decompressor: ", err)
		os.Exit(1)
	}

	decompressedContent, err := io.ReadAll(decompressor)
	if err != nil {
		fmt.Println("Failed to decompress the content: ", err)
		os.Exit(1)
	}

	expected := fmt.Sprint(expected1, "__n__", expected2)

	if string(decompressedContent) != expected {
		fmt.Printf("String inside S3 file is not the expected one. Expected: %s\nGot: %s\n",
			expected, string(downloadedContent))
		os.Exit(1)
	}
	fmt.Println("Expected content is correct!")
	os.Exit(0)
}
