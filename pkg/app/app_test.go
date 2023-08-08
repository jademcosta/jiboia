package app

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const confYaml = `
log:
  level: warn
  format: json

api:
  port: 9099

flows:
  - name: "int_flow"
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
  - name: "int_flow2"
    type: async
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 3
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

const confForCBYaml = `
log:
  level: error
  format: json

api:
  port: 9099

flows:
  - name: "cb_flow"
    type: async
    in_memory_queue_max_size: 2
    max_concurrent_uploads: 3
    max_retries: 3
    timeout: 120
    accumulator:
      size_in_bytes: 5
      separator: "_n_"
      queue_capacity: 2
    external_queue:
      type: noop
      config: ""
    object_storage:
      type: httpstorage
      config:
        url: "{{OBJ_STORAGE_URL}}"
`

var testingPathAcc string = "/tmp/int_test"
var testingPathNoAcc string = "/tmp/int_test2"

var characters = []rune("abcdefghijklmnopqrstuvwxyz")
var l *zap.SugaredLogger

type metricForTests struct {
	name   string
	labels map[string]string
	value  string
}

func TestAccumulatorCircuitBreaker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	storageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer storageServer.Close()

	confFull := strings.Replace(confForCBYaml, "{{OBJ_STORAGE_URL}}",
		fmt.Sprintf("%s/%s", storageServer.URL, "%s"), 1)

	conf, err := config.New([]byte(confFull))
	assert.NoError(t, err, "should initialize config")
	l = logger.New(conf)

	app := New(conf, l)
	go app.Start()
	time.Sleep(2 * time.Second)

	validateMetricValue(
		t,
		"http://localhost:9099",
		metricForTests{
			name:   "jiboia_circuitbreaker_open",
			labels: map[string]string{"flow": "cb_flow", "name": "accumulator"},
			value:  "0",
		},
	)

	payload := randSeq(20)

	for i := 0; i <= 8; i++ {
		//why 8? 3 payload on workers, 2 on uploader queue, 2 on accumulator queue,
		// 1 on accumulator "current"
		response, err := http.Post("http://localhost:9099/cb_flow/async_ingestion", "application/json", strings.NewReader(payload))
		assert.NoError(t, err, "ingesting items should not return error on cb_flow")
		// The status might not be 2XX here, as it is dependant on the order at which parallel
		// components are running
		response.Body.Close()
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	response, err := http.Post("http://localhost:9099/cb_flow/async_ingestion", "application/json", strings.NewReader(payload))
	assert.NoError(t, err, "ingesting items should not return error on cb_flow")
	assert.Equal(t, 500, response.StatusCode,
		"data ingestion should have errored on cb_flow, as the CB is open and queues should be full")
	response.Body.Close()

	validateMetricValue(
		t,
		"http://localhost:9099",
		metricForTests{
			name:   "jiboia_circuitbreaker_open",
			labels: map[string]string{"flow": "cb_flow", "name": "accumulator"},
			value:  "1",
		},
	)

	validateMetricValue(
		t,
		"http://localhost:9099",
		metricForTests{
			name:   "jiboia_circuitbreaker_open_total",
			labels: map[string]string{"flow": "cb_flow", "name": "accumulator"},
			value:  "1",
		},
	)

	stopDone := app.Stop()
	<-stopDone
}

func TestAppIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	conf, err := config.New([]byte(confYaml))
	assert.NoError(t, err, "should initialize config")
	l = logger.New(conf)

	deleteDir(t, testingPathAcc)
	deleteDir(t, testingPathNoAcc)

	t.Run("with not enough data to hit the limit", func(t *testing.T) {
		testWithBatchSize(t, conf, 1)
		testWithBatchSize(t, conf, 1, 2)
		testWithBatchSize(t, conf, 1, 1, 1)
	})

	t.Run("single entry", func(t *testing.T) {
		testWithBatchSize(t, conf, 10)
		testWithBatchSize(t, conf, 18)
		testWithBatchSize(t, conf, 19)
		testWithBatchSize(t, conf, 20)
		testWithBatchSize(t, conf, 21)
		testWithBatchSize(t, conf, 55)
	})

	t.Run("dual entries", func(t *testing.T) {
		testWithBatchSize(t, conf, 11, 5)
		testWithBatchSize(t, conf, 11, 6)
		testWithBatchSize(t, conf, 10, 6)
		testWithBatchSize(t, conf, 12, 5)
		testWithBatchSize(t, conf, 13, 5)
		testWithBatchSize(t, conf, 55, 66)
	})

	t.Run("only big entries", func(t *testing.T) {
		testWithBatchSize(t, conf,
			66, 67, 119)
	})

	t.Run("multiple entries", func(t *testing.T) {
		testWithBatchSize(t, conf,
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27)
	})
}

func testWithBatchSize(t *testing.T, conf *config.Config, stringExemplarSizes ...int) {

	deleteDir(t, testingPathAcc)
	deleteDir(t, testingPathNoAcc)
	createDir(t, testingPathAcc)
	createDir(t, testingPathNoAcc)

	app := New(conf, l)
	go app.Start()
	time.Sleep(2 * time.Second)

	generatedValuesWithAcc := make([]string, 0, len(stringExemplarSizes))
	generatedValuesWithoutAcc := make([]string, 0, len(stringExemplarSizes))

	for _, size := range stringExemplarSizes {
		expected := randSeq(size)
		generatedValuesWithAcc = append(generatedValuesWithAcc, expected)

		response, err := http.Post("http://localhost:9099/int_flow/async_ingestion", "application/json", strings.NewReader(expected))
		assert.NoError(t, err, "enqueueing items should not return error on int_flow")
		assert.Equal(t, 200, response.StatusCode, "data enqueueing should have been successful on int_flow2")
		response.Body.Close()

		// expected = randSeq(size)
		generatedValuesWithoutAcc = append(generatedValuesWithoutAcc, expected)

		response, err = http.Post("http://localhost:9099/int_flow2/async_ingestion", "application/json", strings.NewReader(expected))
		assert.NoError(t, err, "enqueueing items should not return error on int_flow2")
		assert.Equal(t, 200, response.StatusCode, "data enqueueing should have been successful on int_flow2")
		response.Body.Close()

		time.Sleep(1 * time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)

	stopDone := app.Stop()
	<-stopDone

	resultingValuesWithAcc := readFilesFromDir(t, testingPathAcc)
	resultingValuesWithoutAcc := readFilesFromDir(t, testingPathNoAcc)

	var expectedValues []string
	expectedValues = assembleResult(20, "_n_", generatedValuesWithAcc)
	assert.ElementsMatch(t, expectedValues, resultingValuesWithAcc,
		"all the data sent should have been found on the disk for with accumulator case")

	expectedValues = generatedValuesWithoutAcc
	assert.ElementsMatch(t, expectedValues, resultingValuesWithoutAcc,
		"all the data sent should have been found on the disk for without accumulator case")

	deleteDir(t, testingPathAcc)
	deleteDir(t, testingPathNoAcc)
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

func validateMetricValue(t *testing.T, serverUrl string, expectedMetric metricForTests) {
	response, err := http.Get(fmt.Sprintf("%s/metrics", serverUrl))
	assert.NoError(t, err, "getting /metrics should return no error")
	data, err := io.ReadAll(response.Body)
	assert.NoError(t, err, "reading /metrics body should return no error")
	defer response.Body.Close()

	metricsByLine := bytes.Split(data, []byte("\n"))

	metricByLineWithoutHelpLines := make([][]byte, 0)
	for _, line := range metricsByLine {
		notHelpLine := !strings.HasPrefix(string(line), "#")
		if notHelpLine {
			metricByLineWithoutHelpLines = append(metricByLineWithoutHelpLines, line)
		}
	}

	metricLines := filterMetricsLineByName(metricByLineWithoutHelpLines, expectedMetric.name)
	if len(metricLines) == 0 {
		assert.Failf(t, "no metric with given name was found", "name: %s", expectedMetric.name)
		return
	}

	metricLinesBefore := metricLines
	metricLines = filterMetricLinesByValue(metricLines, expectedMetric.value)
	if len(metricLines) == 0 {
		assert.Failf(t, "no metric named with given value was found",
			"value: %s, metrics: %q", expectedMetric.value, metricLinesBefore)
		return
	}

	metricLinesBefore = metricLines
	metricLines = filterMetricLinesByLabels(metricLines, expectedMetric.labels)
	if len(metricLines) > 1 {
		assert.Failf(t,
			"found too many lines that matched all the parameters",
			"%d lines matched %v, metrics: %q", len(metricLines), expectedMetric.labels, metricLines)
		return
	}
	if len(metricLines) == 0 {
		assert.Failf(t, "found no metric that matched the labels", "labels: %v, metrics: %q",
			expectedMetric.labels, metricLinesBefore)
		return
	}
}

func filterMetricsLineByName(metricsLines [][]byte, metricName string) [][]byte {
	metricName = metricName + "{" // '{' marks the end of the name and start of the labels
	found := make([][]byte, 0)
	for _, metricLine := range metricsLines {
		metric := strings.TrimSpace(string(metricLine))
		if strings.HasPrefix(metric, metricName) {
			found = append(found, metricLine)
		}
	}
	return found
}

func filterMetricLinesByValue(metricsLines [][]byte, metricValue string) [][]byte {
	found := make([][]byte, 0)
	for _, metricLine := range metricsLines {
		metric := strings.TrimSpace(string(metricLine))
		if strings.HasSuffix(metric, metricValue) {
			found = append(found, metricLine)
		}
	}
	return found
}

func filterMetricLinesByLabels(metricsLines [][]byte, labels map[string]string) [][]byte {
	found := make([][]byte, 0)
	for _, metricLine := range metricsLines {
		accepted := true

		for labelName, labelValue := range labels {
			matched, _ := regexp.MatchString(
				fmt.Sprintf("%s=\"%s\"", labelName, labelValue), string(metricLine))
			if !matched {
				accepted = false
				break
			}
		}

		if accepted {
			found = append(found, metricLine)
		}
	}
	return found
}
