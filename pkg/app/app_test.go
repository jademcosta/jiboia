package app_test

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

	"github.com/jademcosta/jiboia/pkg/app"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const confWithAccumulator = `
log:
  level: warn
  format: json

api:
  port: 9099

flows:
  - name: "integration_flow"
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
  - name: "flow_without_accumulator"
    type: async
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 2
    max_retries: 3
    timeout: 120
    external_queue:
      type: noop
      config: ""
    object_storage:
      type: localstorage
      config:
        path: "/tmp/int_test2"
`

var testingPath2 string = "/tmp/int_test2"
var testingPath string = "/tmp/int_test"

var l *zap.SugaredLogger
var characters = []rune("abcdefghijklmnopqrstuvwxyz")

func TestAppIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	conf, err := config.New([]byte(confWithAccumulator))
	assert.NoError(t, err, "should initialize config")
	l = logger.New(conf)

	deleteDir(t, testingPath)
	deleteDir(t, testingPath2)

	t.Run("with size bigger than accumulator limit", func(t *testing.T) {
		testWithBatchSize(t, conf, 21)
	})

	t.Run("with size exactly with the accumulator limit", func(t *testing.T) {
		testWithBatchSize(t, conf, 20)
	})

	t.Run("with size below the accumulator limit", func(t *testing.T) {
		testWithBatchSize(t, conf, 18)
	})

	t.Run("with size way below the accumulator limit", func(t *testing.T) {
		testWithBatchSize(t, conf, 10)
	})

	t.Run("with 2 items smaller than accumulator limit", func(t *testing.T) {
		testWithBatchSize(t, conf, 11, 5)
	})

	t.Run("with 2 items which result in exactly the accumulator limit", func(t *testing.T) {
		testWithBatchSize(t, conf, 12, 5)
	})

	t.Run("with 2 items which result in above the accumulator limit", func(t *testing.T) {
		testWithBatchSize(t, conf, 13, 5)
	})

	t.Run("with multiple items", func(t *testing.T) {
		testWithBatchSize(t, conf,
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21)
	})
}

func testWithBatchSize(t *testing.T, conf *config.Config,
	stringExemplarSizes ...int) {

	deleteDir(t, testingPath)
	deleteDir(t, testingPath2)

	createDir(t, testingPath)
	createDir(t, testingPath2)

	sut := app.New(conf, l)
	go sut.Start()
	time.Sleep(2 * time.Second)

	generatedValues := make([]string, 0, len(stringExemplarSizes))

	for _, size := range stringExemplarSizes {
		expected := randSeq(size)
		generatedValues = append(generatedValues, expected)

		response, err := http.Post("http://localhost:9099/integration_flow/async_ingestion", "application/json", strings.NewReader(expected))
		assert.NoError(t, err, "enqueueing items should not return error")
		assert.Equal(t, 200, response.StatusCode, "data enqueueing should have been successful")
		response.Body.Close()

		response, err = http.Post("http://localhost:9099/flow_without_accumulator/async_ingestion", "application/json", strings.NewReader(expected))
		assert.NoError(t, err, "enqueueing items should not return error")
		assert.Equal(t, 200, response.StatusCode, "data enqueueing should have been successful")
		response.Body.Close()
	}
	time.Sleep(10 * time.Millisecond)

	sut.Stop()
	time.Sleep(1 * time.Second)

	resultingValuesFlow1 := readFilesFromDir(t, testingPath)
	resultingValuesFlow2 := readFilesFromDir(t, testingPath2)
	expectedValues := assembleResult(20, "_n_", generatedValues)

	assert.ElementsMatch(t, expectedValues, resultingValuesFlow1, "all the data from first flow should have been found on the disk")
	assert.ElementsMatch(t, generatedValues, resultingValuesFlow2, "all the data from second flow should have been found on the disk")

	deleteDir(t, testingPath)
}

func randSeq(n int) string {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]rune, n)

	for i := range b {
		b[i] = characters[r1.Intn(len(characters))]
	}
	return string(b)
}

func deleteDir(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	assert.NoError(t, err, "should clear the subdir where we will store data")
	if err != nil {
		panic(fmt.Sprintf("error deleting the dir where stored data is: %v", err))
	}
}

func createDir(t *testing.T, dir string) {
	err := os.MkdirAll(dir, os.ModePerm)
	assert.NoError(t, err, "should create subdir to store data")
	if err != nil {
		panic(fmt.Sprintf("error creating the dir to store the data: %v", err))
	}
}

func readFilesFromDir(t *testing.T, dir string) []string {
	resultingValues := make([]string, 0)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return err
		}

		data, err := os.ReadFile(path)
		assert.NoError(t, err, "reading files with sent data should not return error")
		resultingValues = append(resultingValues, string(data))

		return err
	})
	assert.NoError(t, err, "reading files with sent data should not return error")
	return resultingValues
}

func assembleResult(accumulatorMaxSize int, separator string, generatedValues []string) []string {
	var currentBuffer string
	result := make([]string, 0)
	for _, value := range generatedValues {
		currentValueLenIsTooBig := len(value) >= accumulatorMaxSize

		if currentValueLenIsTooBig {
			if len(currentBuffer) > 0 {
				result = append(result, currentBuffer)
				currentBuffer = ""
			}
			result = append(result, value)
			continue
		}

		tooBigForBuffer := len(currentBuffer)+len(separator)+len(value) > accumulatorMaxSize
		if tooBigForBuffer {
			if len(currentBuffer) > 0 {
				result = append(result, currentBuffer)
			}
			currentBuffer = value
		} else {
			if len(currentBuffer) != 0 {
				currentBuffer += separator
			}
			currentBuffer += value
		}

		if len(currentBuffer) == accumulatorMaxSize {
			result = append(result, currentBuffer)
			currentBuffer = ""
		}
	}

	if len(currentBuffer) > 0 {
		result = append(result, currentBuffer)
	}

	return result
}
