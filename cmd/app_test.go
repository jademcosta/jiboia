package main

import (
	"fmt"
	"io/fs"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
)

const confWithAccumulator = `
log:
  level: error
  format: json

api:
  port: 9099

flow:
  name: "integration_flow"
  type: async
  in_memory_queue_max_size: 4
  max_concurrent_uploads: 2
  max_retries: 3
  timeout: 120
  accumulator:
    size_in_bytes: 20
    separator: "_n_"
    queue_capacity: 10
  external_queue:
    type: noop
    config: ""
  object_storage:
    type: localstorage
    config:
      path: "/tmp/int_test"
`

var testingPath string = "/tmp/int_test"

var letters = []rune("abcdefghijklmnopqrstuvwxyz")

func TestAppIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	conf, err := config.New([]byte(confWithAccumulator))
	assert.NoError(t, err, "should initialize config")
	l := logger.New(conf)

	err = os.MkdirAll(testingPath, os.ModePerm)
	assert.NoError(t, err, "should create subdir to store data")
	if err != nil {
		panic(fmt.Sprintf("error creating the dir to store the data: %v", err))
	}

	app := New(conf, l)
	go app.start()
	time.Sleep(2 * time.Second)

	produceAndValidate(t, testingPath, 20)
	time.Sleep(2 * time.Second)
	app.stop()
	// produceAndValidate(t, testingPath, 12, 5)
	// produceAndValidate(t, testingPath, 18)
}

func produceAndValidate(t *testing.T, path string, stringExemplarSizes ...int) {
	err := os.RemoveAll(testingPath)
	assert.NoError(t, err, "should clear the subdir where we will store data")

	err = os.MkdirAll(testingPath, os.ModePerm)
	assert.NoError(t, err, "should create subdir to store data")

	expectedValues := make([]string, 0, len(stringExemplarSizes))
	resultingValues := make([]string, 0)

	for _, size := range stringExemplarSizes {
		expected := randSeq(size)
		expectedValues = append(expectedValues, expected)

		response, err := http.Post("http://localhost:9099/integration_flow/async_ingestion", "application/json", strings.NewReader(expected))
		assert.NoError(t, err, "enqueueing items should not return error")
		assert.Equal(t, 200, response.StatusCode, "data enqueueing should have been successful")
		response.Body.Close()
	}
	time.Sleep(10 * time.Millisecond)

	err = filepath.WalkDir(testingPath, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return err
		}

		data, err := os.ReadFile(path)
		assert.NoError(t, err, "reading files with sent data should not return error")
		resultingValues = append(resultingValues, string(data))

		return err
	})
	assert.NoError(t, err, "reading files with sent data should not return error")

	assert.ElementsMatch(t, expectedValues, resultingValues, "all the data sent should have been found on the disk")
}

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
