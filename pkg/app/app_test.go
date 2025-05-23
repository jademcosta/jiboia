package app

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/compression"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
)

var testingPathNoAcc = "/tmp/int_test2"

var characters = []rune("abcdefghijklmnopqrstuvwxyz")
var logg *slog.Logger

func TestAccumulatorCircuitBreaker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	storageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer storageServer.Close()

	confFull := strings.Replace(confForCBYaml, "{{OBJ_STORAGE_URL}}",
		fmt.Sprintf("%s/%s", storageServer.URL, "whatever"), 1)

	conf, err := config.New([]byte(confFull))
	assert.NoError(t, err, "should initialize config")
	logg = logger.New(&conf.O11y.Log)

	app := New(conf, logg)
	go app.Start()
	time.Sleep(2 * time.Second)

	payload := randSeq(20)

	for i := 0; i <= 11; i++ {
		//why 11? 3 payload on workers, 3 on queue from uploader to workers, 2 on uploader queue,
		// 2 on accumulator queue, 1 on accumulator "current"
		response, err := http.Post("http://localhost:9098/cb_flow/async_ingestion", "application/json", strings.NewReader(payload))
		assert.NoError(t, err, "ingesting items should not return error on cb_flow")
		// The status might not be 2XX here, as it is dependant on the order at which parallel
		// components are running
		response.Body.Close()
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	response, err := http.Post("http://localhost:9098/cb_flow/async_ingestion", "application/json", strings.NewReader(payload))
	assert.NoError(t, err, "ingesting items should not return error on cb_flow")
	assert.Equal(t, 500, response.StatusCode,
		"data ingestion should have errored on cb_flow, as the CB is open and queues should be full")
	response.Body.Close()

	stopDone := app.Stop()
	<-stopDone
}

func TestPayloadSizeLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Needed for the other ingestion endpoints
	createDir(t, testingPathNoAcc)
	defer func() {
		deleteDir(t, testingPathNoAcc)
	}()

	var mu sync.Mutex
	objStorageReceived := make([][]byte, 0)

	storageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		objStorageReceived = append(objStorageReceived, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer storageServer.Close()

	confFull := strings.Replace(confYaml, "http://non-existent-27836178236.com",
		fmt.Sprintf("%s/%s", storageServer.URL, "%s"), 1)

	conf, err := config.New([]byte(confFull))
	assert.NoError(t, err, "should initialize config")
	logg = logger.New(&conf.O11y.Log)

	app := New(conf, logg)
	go app.Start()
	time.Sleep(2 * time.Second)

	validPayload := randSeq(120)
	invalidPayload := randSeq(121)

	req, err := http.NewRequest(
		http.MethodPost,
		"http://localhost:9099/int_flow3/async_ingestion",
		strings.NewReader(validPayload),
	)
	assert.NoError(t, err, "error creating request")

	req.Header.Set("Authorization", "Bearer some secure token")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err, "error on posting data")
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")

	req, err = http.NewRequest(
		http.MethodPost,
		"http://localhost:9099/int_flow3/async_ingestion",
		strings.NewReader(invalidPayload),
	)
	assert.NoError(t, err, "error creating request")

	req.Header.Set("Authorization", "Bearer some secure token")
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err, "ingesting items should not return error on int_flow3")
	resp.Body.Close()

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode, "status should be Request entity too large(413)")

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	assert.Equal(t, [][]byte{[]byte(validPayload)}, objStorageReceived,
		"only the valid payload should be ingested")
	mu.Unlock()

	stopDone := app.Stop()
	<-stopDone
}

func TestApiToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Needed for the other ingestion endpoints
	createDir(t, testingPathNoAcc)
	defer func() {
		deleteDir(t, testingPathNoAcc)
	}()

	var mu sync.Mutex
	objStorageReceived := make([][]byte, 0)

	storageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		objStorageReceived = append(objStorageReceived, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer storageServer.Close()

	confFull := strings.Replace(confYaml, "http://non-existent-27836178236.com",
		fmt.Sprintf("%s/%s", storageServer.URL, "%s"), 1)

	conf, err := config.New([]byte(confFull))
	assert.NoError(t, err, "should initialize config")
	logg = logger.New(&conf.O11y.Log)

	app := New(conf, logg)
	go app.Start()
	time.Sleep(2 * time.Second)

	ingestedPayload := randSeq(120)
	nonIngestedPayload := randSeq(121)

	req, err := http.NewRequest(
		http.MethodPost,
		"http://localhost:9099/int_flow3/async_ingestion",
		strings.NewReader(nonIngestedPayload),
	)
	assert.NoError(t, err, "error creating request")

	resp, err := http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.NoError(t, err, "error posting data")

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "status should be Unauthorized(401)")

	req, err = http.NewRequest(
		http.MethodPost,
		"http://localhost:9099/int_flow3/async_ingestion",
		strings.NewReader(ingestedPayload),
	)
	assert.NoError(t, err, "error creating request")

	req.Header.Set("Authorization", "Bearer some secure token")
	resp, err = http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.NoError(t, err, "error posting data")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	assert.Equal(t, [][]byte{[]byte(ingestedPayload)}, objStorageReceived,
		"only the valid payload should be ingested")
	mu.Unlock()

	stopDone := app.Stop()
	<-stopDone
}

func TestIngestionDecompression(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Needed for the other ingestion endpoints
	createDir(t, testingPathNoAcc)
	defer func() {
		deleteDir(t, testingPathNoAcc)
	}()

	var mu sync.Mutex
	objStorageReceived := make([][]byte, 0)

	storageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		objStorageReceived = append(objStorageReceived, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer storageServer.Close()

	confFull := strings.Replace(confYaml, "http://non-existent-flow4.com",
		fmt.Sprintf("%s/%s", storageServer.URL, "%s"), 1)

	conf, err := config.New([]byte(confFull))
	assert.NoError(t, err, "should initialize config")
	logg = logger.New(&conf.O11y.Log)

	app := New(conf, logg)
	go app.Start()
	time.Sleep(2 * time.Second)

	payload := randSeq(100)
	buf1 := &bytes.Buffer{}
	writer, err := compression.NewWriter(&config.CompressionConfig{Type: "snappy"}, buf1)
	assert.NoError(t, err, "error on compressor writer creation", err)
	_, err = writer.Write([]byte(payload))
	assert.NoError(t, err, "error compressing data")
	err = writer.Close()
	assert.NoError(t, err, "error closing compressor")

	highlyCompressRatioPayload := strings.Repeat("ab", 50)
	buf2 := &bytes.Buffer{}
	writer, err = compression.NewWriter(&config.CompressionConfig{Type: "gzip"}, buf2)
	assert.NoError(t, err, "error on compressor writer creation", err)
	_, err = writer.Write([]byte(highlyCompressRatioPayload))
	assert.NoError(t, err, "error compressing data")
	err = writer.Close()
	assert.NoError(t, err, "error closing compressor")

	nonCompressedPayload := randSeq(120)

	req, err := http.NewRequest(
		http.MethodPost,
		"http://localhost:9099/int_flow4/async_ingestion",
		buf1,
	)
	assert.NoError(t, err, "error creating request")

	req.Header.Add("Content-Encoding", "snappy")
	resp, err := http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.NoError(t, err, "error posting data")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be Ok(200)")

	time.Sleep(100 * time.Millisecond)

	req, err = http.NewRequest(
		http.MethodPost,
		"http://localhost:9099/int_flow4/async_ingestion",
		buf2,
	)
	assert.NoError(t, err, "error creating request")

	req.Header.Add("Content-Encoding", "gzip")
	resp, err = http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.NoError(t, err, "error posting data")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")

	time.Sleep(100 * time.Millisecond)

	req, err = http.NewRequest(
		http.MethodPost,
		"http://localhost:9099/int_flow4/async_ingestion",
		strings.NewReader(nonCompressedPayload),
	)
	assert.NoError(t, err, "error creating request")

	resp, err = http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.NoError(t, err, "error posting data")

	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	assert.Equal(
		t,
		[][]byte{[]byte(payload), []byte(highlyCompressRatioPayload), []byte(nonCompressedPayload)},
		objStorageReceived,
		"all payloads should be decompressed, in case it needs, and ingested",
	)
	mu.Unlock()

	stopDone := app.Stop()
	<-stopDone
}

func TestCompression(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	var mu sync.Mutex
	objStorageReceived := make([][]byte, 0)
	filenames := make([]string, 0)

	storageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		objStorageReceived = append(objStorageReceived, body)

		urlChuncks := strings.Split(r.RequestURI, "/")
		filename := urlChuncks[len(urlChuncks)-1]
		filenames = append(filenames, filename)
		w.WriteHeader(http.StatusOK)
	}))
	defer storageServer.Close()

	confFull := strings.Replace(confForCompressionYaml, "http://non-existent-4.com",
		fmt.Sprintf("%s/%s", storageServer.URL, "%s"), 1)

	conf, err := config.New([]byte(confFull))
	assert.NoError(t, err, "should initialize config")
	logg = logger.New(&conf.O11y.Log)

	app := New(conf, logg)
	go app.Start()
	time.Sleep(2 * time.Second)

	ingestedPayload := strings.Repeat("a", 10240) // 10KB

	resp, err := http.Post(
		"http://localhost:9099/int_flow4/async_ingestion",
		"application/json",
		strings.NewReader(ingestedPayload),
	)
	assert.NoError(t, err, "POSTing should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	assert.Len(t, objStorageReceived, 1,
		"should have sent the data to obj storage")
	assert.NotEqual(t, []byte(ingestedPayload), objStorageReceived[0],
		"the data should have been compressed")

	compressorReader, err := compression.NewReader(&conf.Flows[1].Compression, bytes.NewReader(objStorageReceived[0]))
	assert.NoError(t, err, "compression reader creation should return no error")

	decompressed, err := io.ReadAll(compressorReader)
	assert.NoError(t, err, "compression reader Read should return no error")

	assert.Equal(t, len(ingestedPayload), len(decompressed), "the decompression result should have the same size as the original")
	assert.Equal(t, ingestedPayload, string(decompressed), "the decompression result be the same as the original")

	assert.Truef(t, strings.HasSuffix(filenames[0], fmt.Sprintf(".%s", "gzip")),
		"should add the compression algorithm suffix (%s) on filename (%s)", "gzip", filenames[0])
	mu.Unlock()

	stopDone := app.Stop()
	<-stopDone
}

func TestAppIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	deleteDir(t, testingPathNoAcc)

	t.Run("with not enough data to hit the limit", func(t *testing.T) {
		testWithBatchSize(t, confYaml, 1)
		testWithBatchSize(t, confYaml, 1, 2)
		testWithBatchSize(t, confYaml, 1, 1, 1)
	})

	t.Run("single entry", func(t *testing.T) {
		testWithBatchSize(t, confYaml, 10)
		testWithBatchSize(t, confYaml, 18)
		testWithBatchSize(t, confYaml, 19)
		testWithBatchSize(t, confYaml, 20)
		testWithBatchSize(t, confYaml, 21)
		testWithBatchSize(t, confYaml, 55)
	})

	t.Run("dual entries", func(t *testing.T) {
		testWithBatchSize(t, confYaml, 11, 5)
		testWithBatchSize(t, confYaml, 11, 6)
		testWithBatchSize(t, confYaml, 10, 6)
		testWithBatchSize(t, confYaml, 12, 5)
		testWithBatchSize(t, confYaml, 13, 5)
		testWithBatchSize(t, confYaml, 55, 66)
	})

	t.Run("only big entries", func(t *testing.T) {
		testWithBatchSize(t, confYaml,
			66, 67, 119)
	})

	t.Run("multiple entries", func(t *testing.T) {
		testWithBatchSize(t, confYaml,
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27)
	})
}

func testWithBatchSize(t *testing.T, confYaml string, stringExemplarSizes ...int) {

	var mu sync.Mutex
	objStorageReceivedFlow1 := make([][]byte, 0)
	// var wgStorage1 sync.WaitGroup
	// FIXME: implementar o wg pra esperar os dados, assim poupa tempo

	storageServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		objStorageReceivedFlow1 = append(objStorageReceivedFlow1, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer storageServer1.Close()

	confFull := strings.Replace(confYaml, "http://non-existent-flow1.com",
		fmt.Sprintf("%s/%s", storageServer1.URL, "%s"), 1)

	conf, err := config.New([]byte(confFull))
	assert.NoError(t, err, "should initialize config")
	logg = logger.New(&conf.O11y.Log)

	deleteDir(t, testingPathNoAcc)
	createDir(t, testingPathNoAcc)

	app := New(conf, logg)
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

		generatedValuesWithoutAcc = append(generatedValuesWithoutAcc, expected)

		response, err = http.Post("http://localhost:9099/v1/int_flow2/async_ingestion", "application/json", strings.NewReader(expected))
		assert.NoError(t, err, "enqueueing items should not return error on int_flow2")
		assert.Equal(t, 200, response.StatusCode, "data enqueueing should have been successful on int_flow2")
		response.Body.Close()

		time.Sleep(1 * time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)

	//FIXME: esperar pelo sync.WG terminar aqui

	stopDone := app.Stop()
	<-stopDone

	resultingValuesWithoutAcc := readFilesFromDir(t, testingPathNoAcc)

	var expectedValues []string
	receivedValues := make([]string, 0, len(objStorageReceivedFlow1))
	for _, data := range objStorageReceivedFlow1 {
		receivedValues = append(receivedValues, string(data))
	}
	expectedValues = assembleResult(20, "_n_", generatedValuesWithAcc)
	assert.ElementsMatch(t, expectedValues, receivedValues,
		"all the data sent should have been found on the disk for flow 1 (with accumulator)")

	expectedValues = generatedValuesWithoutAcc
	assert.ElementsMatch(t, expectedValues, resultingValuesWithoutAcc,
		"all the data sent should have been found on the disk for flow 2 (without accumulator)")

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
