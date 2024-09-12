package httpin

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/compressor"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/jademcosta/jiboia/pkg/o11y/tracing"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
)

const version string = "0.0.0"

var llog = logger.NewDummy()
var testTracer = tracing.NewNoopTracer()
var localConf = config.Config{
	API:  config.APIConfig{Port: 9111},
	O11y: config.O11yConfig{TracingEnabled: true},
}

var characters = []rune("abcdefghijklmnopqrstuvwxyz")

func randString(n int) string {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]rune, n)

	for i := range b {
		b[i] = characters[r1.Intn(len(characters))]
	}
	return string(b)
}

type mockDataFlow struct {
	calledWith  [][]byte
	mu          sync.Mutex
	enqueueFunc func([]byte) error
}

func (mockDF *mockDataFlow) Enqueue(data []byte) error {
	mockDF.mu.Lock()
	mockDF.calledWith = append(mockDF.calledWith, data)
	mockDF.mu.Unlock()

	if mockDF.enqueueFunc != nil {
		return mockDF.enqueueFunc(data)
	}
	return nil
}

type dummyAlwaysFailDataFlow struct{}

func (mockDF *dummyAlwaysFailDataFlow) Enqueue(data []byte) error {
	return fmt.Errorf("dummy error")
}

type brokenDataFlow struct{}

func (brokenDF *brokenDataFlow) Enqueue(data []byte) error {
	panic("I always panic")
}

func TestPassesDataFlows(t *testing.T) {

	mockDF := &mockDataFlow{
		calledWith: make([][]byte, 0),
	}

	mockDF2 := &mockDataFlow{
		calledWith: make([][]byte, 0),
	}

	flws := []flow.Flow{
		{
			Name:           "flow-1",
			Entrypoint:     mockDF,
			CircuitBreaker: createCircuitBreaker("flow-1", 10*time.Millisecond),
		},
		{
			Name:           "flow2",
			Entrypoint:     mockDF2,
			CircuitBreaker: createCircuitBreaker("flow2", 10*time.Millisecond),
		},
	}

	api := NewAPI(llog, localConf, prometheus.NewRegistry(), testTracer, version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json", strings.NewReader("helloooooo"))
	assert.NoError(t, err, "error on posting data to flow1", err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200) on flow 1")

	resp, err = http.Post(fmt.Sprintf("%s/flow2/async_ingestion", srvr.URL), "application/json", strings.NewReader("world!"))
	assert.NoError(t, err, "error on posting data to flow2", err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200) on flow 2")

	//Same test, but with v1 on URL
	resp, err = http.Post(fmt.Sprintf("%s/v1/flow-1/async_ingestion", srvr.URL), "application/json", strings.NewReader("helloooooo1"))
	assert.NoError(t, err, "error on posting data to flow1", err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200) on flow 1")

	resp, err = http.Post(fmt.Sprintf("%s/v1/flow2/async_ingestion", srvr.URL), "application/json", strings.NewReader("world2"))
	assert.NoError(t, err, "error on posting data to flow2", err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200) on flow 2")

	assert.Equal(t, [][]byte{[]byte("helloooooo"), []byte("helloooooo1")},
		mockDF.calledWith, "the posted data should have been sent to flow 1")
	assert.Equal(t, [][]byte{[]byte("world!"), []byte("world2")},
		mockDF2.calledWith, "the posted data should have been sent to flow 2")
}

func TestAnswersAnErrorIfNoBodyIsSent(t *testing.T) {

	mockDF := &mockDataFlow{
		calledWith: make([][]byte, 0),
	}

	flws := []flow.Flow{
		{
			Name:           "flow-1",
			Entrypoint:     mockDF,
			CircuitBreaker: createCircuitBreaker("flow-1", 10*time.Millisecond),
		},
	}

	api := NewAPI(llog, localConf, prometheus.NewRegistry(), testTracer, version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json", strings.NewReader(""))
	assert.NoError(t, err, "error on posting data", err)
	resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "status should be Bad Request(400)")
	assert.Lenf(t, mockDF.calledWith, 0, "no data should have been sent to flow")
}

func TestAnswersErrorIfEnqueueingFails(t *testing.T) {

	mockDF := &dummyAlwaysFailDataFlow{}

	flws := []flow.Flow{
		{
			Name:           "flow-1",
			Entrypoint:     mockDF,
			CircuitBreaker: createCircuitBreaker("flow-1", 10*time.Millisecond),
		},
	}

	api := NewAPI(llog, localConf, prometheus.NewRegistry(), testTracer, version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json", strings.NewReader("some data"))
	assert.NoError(t, err, "error on posting data", err)
	resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode, "status should be Internal Server Error(500)")
}

func TestPanicResultInStatus500(t *testing.T) {

	brokenDF := &brokenDataFlow{}

	flws := []flow.Flow{
		{
			Name:           "flow-1",
			Entrypoint:     brokenDF,
			CircuitBreaker: createCircuitBreaker("flow-1", 10*time.Millisecond),
		},
	}
	api := NewAPI(llog, localConf, prometheus.NewRegistry(), testTracer, version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json", strings.NewReader("some data"))
	assert.NoError(t, err, "error on posting data", err)
	resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode,
		"status should be Internal Server Error(500) when panics occur")
}

func TestPayloadSizeLimit(t *testing.T) {
	c := config.Config{API: config.APIConfig{Port: 9111, PayloadSizeLimit: "10"}} //10 bytes limit

	df := &mockDataFlow{calledWith: make([][]byte, 0)}

	flws := []flow.Flow{
		{
			Name:           "flow-1",
			Entrypoint:     df,
			CircuitBreaker: createCircuitBreaker("flow-1", 10*time.Millisecond),
		},
	}

	api := NewAPI(llog, c, prometheus.NewRegistry(), testTracer, version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json",
		strings.NewReader("somedata"))
	assert.NoError(t, err, "error on posting data", err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"status should be Internal Server Ok(200) when size is within limits")

	resp, err = http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json",
		strings.NewReader("1111111111")) //10 bytes
	assert.NoError(t, err, "error on posting data", err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"status should be Internal Server Ok(200) when size is within limits")

	resp, err = http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json",
		strings.NewReader("abcdefghijk")) //11 bytes
	assert.NoError(t, err, "error on posting data", err)
	resp.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode,
		"status should be Internal Server Payload too large(413) when size is above limits (11 bytes)")

	resp, err = http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json",
		strings.NewReader("abcdefghijkl")) //12 bytes
	assert.NoError(t, err, "error on posting data", err)
	resp.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode,
		"status should be Internal Server Payload too large(413) when size is above limits (12 bytes)")

	assert.Equal(t, [][]byte{[]byte("somedata"), []byte("1111111111")}, df.calledWith,
		"only allowed payloads should be ingested")
}

func TestApiToken(t *testing.T) {
	token := "whatever token here random maybe bla bla bla"
	token3 := "a different token"

	mockDF := &mockDataFlow{
		calledWith: make([][]byte, 0),
	}
	mockDF2 := &mockDataFlow{
		calledWith: make([][]byte, 0),
	}
	mockDF3 := &mockDataFlow{
		calledWith: make([][]byte, 0),
	}

	flws := []flow.Flow{
		{
			Name:           "flow-1",
			Entrypoint:     mockDF,
			Token:          token,
			CircuitBreaker: createCircuitBreaker("flow-1", 10*time.Millisecond),
		},
		{
			Name:           "flow-2",
			Entrypoint:     mockDF2,
			CircuitBreaker: createCircuitBreaker("flow-2", 10*time.Millisecond),
		},
		{
			Name:           "flow-3",
			Entrypoint:     mockDF3,
			Token:          token3,
			CircuitBreaker: createCircuitBreaker("flow-3", 10*time.Millisecond),
		},
	}

	api := NewAPI(llog, localConf, prometheus.NewRegistry(), testTracer, version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	t.Run("denies if required token and no token provided", func(t *testing.T) {

		req, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL),
			strings.NewReader("payload1"),
		)
		if err != nil {
			assert.Fail(t, "error creating request", err)
		}

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "status should be Unauthorized(401)")
		assert.Lenf(t, mockDF.calledWith, 0, "no data should have been sent to flow1")
	})

	t.Run("denies if required token and wrong token", func(t *testing.T) {

		testCases := []struct {
			header string
			tkn    string
		}{
			{header: "Authorization", tkn: token},
			{header: "Authorization", tkn: "wrong token"},
			{header: "Authorization", tkn: "Bearer some token"},
			{header: "Authorization", tkn: ""},
			{header: "X-Authorization", tkn: fmt.Sprintf("Bearer %s", token)},
			{header: "Authorization", tkn: fmt.Sprintf("Bearer %s1", token)},
			{header: "Authorization", tkn: fmt.Sprintf("Bearer 1%s", token)},
			{header: "Authorization", tkn: fmt.Sprintf("Bearer  %s", token)},
		}

		for _, tc := range testCases {
			req, err := http.NewRequest(
				http.MethodPost,
				fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL),
				strings.NewReader("payload1"),
			)
			if err != nil {
				assert.Fail(t, "error creating request", err)
			}

			req.Header.Set(tc.header, tc.tkn)

			resp, err := http.DefaultClient.Do(req)
			assert.NoError(t, err, "error on posting data", err)
			resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "status should be Unauthorized(401)")
		}

		assert.Lenf(t, mockDF.calledWith, 0, "no data should have been sent to flow1")
	})

	t.Run("accepts if correct token", func(t *testing.T) {

		req, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL),
			strings.NewReader("payload-right"),
		)
		if err != nil {
			assert.Fail(t, "error creating request", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")
		assert.Lenf(t, mockDF.calledWith, 1, "the data should have been sent to flow1")
		assert.Equal(t, [][]byte{[]byte("payload-right")}, mockDF.calledWith, "the correct data should have been sent to flow1")
	})

	t.Run("accepts all when no token required", func(t *testing.T) {

		req, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("%s/flow-2/async_ingestion", srvr.URL),
			strings.NewReader("payload2"),
		)
		if err != nil {
			assert.Fail(t, "error creating request", err)
		}

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")

		req, err = http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("%s/flow-2/async_ingestion", srvr.URL),
			strings.NewReader("payload3"),
		)
		if err != nil {
			assert.Fail(t, "error creating request", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		resp, err = http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")

		req, err = http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("%s/flow-2/async_ingestion", srvr.URL),
			strings.NewReader("payload4"),
		)
		if err != nil {
			assert.Fail(t, "error creating request", err)
		}

		req.Header.Set("Authorization", "any token")

		resp, err = http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")

		assert.Lenf(t, mockDF2.calledWith, 3, "the data should have been sent to flow2")
		assert.Equal(t,
			[][]byte{[]byte("payload2"), []byte("payload3"), []byte("payload4")},
			mockDF2.calledWith,
			"the correct data should have been sent to flow2")
	})

	t.Run("may have different tokens per flow", func(t *testing.T) {

		req, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL),
			strings.NewReader("payload"),
		)
		if err != nil {
			assert.Fail(t, "error creating request", err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")

		req, err = http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("%s/flow-3/async_ingestion", srvr.URL),
			strings.NewReader("payload-to-flow3"),
		)
		if err != nil {
			assert.Fail(t, "error creating request", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token3))

		resp, err = http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200)")

		req, err = http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("%s/flow-3/async_ingestion", srvr.URL),
			strings.NewReader("payload-to-flow3"),
		)
		if err != nil {
			assert.Fail(t, "error creating request", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		resp, err = http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "status should be Unauthorized(401)")

		assert.Lenf(t, mockDF3.calledWith, 1, "the data should have been sent to flow3")
		assert.Equal(t,
			[][]byte{[]byte("payload-to-flow3")},
			mockDF3.calledWith,
			"the correct data should have been sent to flow3")
	})

	assert.Lenf(t, mockDF.calledWith, 2, "the data should have been sent to flow1")
	assert.Equal(t, [][]byte{[]byte("payload-right"), []byte("payload")}, mockDF.calledWith, "the correct data should have been sent to flow1")

	assert.Lenf(t, mockDF2.calledWith, 3, "the data should have been sent to flow2")
	assert.Equal(t,
		[][]byte{[]byte("payload2"), []byte("payload3"), []byte("payload4")},
		mockDF2.calledWith,
		"the correct data should have been sent to flow2")
}

func TestVersionEndpointInformsTheVersion(t *testing.T) {

	mockDF := &dummyAlwaysFailDataFlow{}

	flws := []flow.Flow{
		{
			Name:           "flow-1",
			Entrypoint:     mockDF,
			CircuitBreaker: createCircuitBreaker("flow-1", 10*time.Millisecond),
		},
	}

	api := NewAPI(llog, localConf, prometheus.NewRegistry(), testTracer, version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Get(fmt.Sprintf("%s/version", srvr.URL))
	assert.NoError(t, err, "error on GETting data", err)

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	assert.NoError(t, err, "should not err when reading body from http")
	defer resp.Body.Close()
	body := buf.String()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be Ok(200)")
	assert.Equal(t, fmt.Sprintf("{\"version\":\"%s\"}", version), body, "version informed should be the current one")
}

func TestDecompressionOnIngestion(t *testing.T) {

	t.Run("Happy path", func(t *testing.T) {
		testCases := []struct {
			algorithms []string
		}{
			{algorithms: []string{"snappy"}},
			{algorithms: []string{"gzip"}},
			{algorithms: []string{"zstd"}},
			{algorithms: []string{"deflate"}},
			{algorithms: []string{"zlib"}},
			{algorithms: []string{"zlib", "gzip"}},
			{algorithms: []string{"snappy", "gzip", "zstd"}},
			{algorithms: []string{"snappy", "gzip", "zstd", "deflate", "zlib"}},
		}

		for _, tc := range testCases {
			df := &mockDataFlow{calledWith: make([][]byte, 0)}

			flws := []flow.Flow{
				{
					Name:                    "flow-1",
					Entrypoint:              df,
					DecompressionAlgorithms: tc.algorithms,
					CircuitBreaker:          createCircuitBreaker("flow-1", 10*time.Millisecond),
				},
			}

			api := NewAPI(llog, localConf, prometheus.NewRegistry(), testTracer, version, flws)
			srvr := httptest.NewServer(api.mux)
			defer srvr.Close()

			expected := make([][]byte, 0)

			for _, algorithm := range tc.algorithms {

				decompressedData1 := strings.Repeat("ab", 90)
				expected = append(expected, []byte(decompressedData1))

				buf1 := &bytes.Buffer{}
				writer, err := compressor.NewWriter(&config.CompressionConfig{Type: algorithm}, buf1)
				assert.NoError(t, err, "error on compressor writer creation", err)
				_, err = writer.Write([]byte(decompressedData1))
				assert.NoError(t, err, "error on compressing data", err)
				err = writer.Close()
				assert.NoError(t, err, "error on compressor Close", err)

				assert.NotEqual(t, decompressedData1, buf1.Bytes(), "the data should have been compressed before sending")

				req, err := http.NewRequest(http.MethodPost,
					fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), buf1)
				assert.NoError(t, err, "error on request creation", err)
				req.Header.Add("Content-Encoding", algorithm)

				resp, err := http.DefaultClient.Do(req)
				assert.NoError(t, err, "error on posting data", err)
				resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode,
					"status should be Internal Server Ok(200)")

				decompressedData2 := randString(300)
				expected = append(expected, []byte(decompressedData2))
				buf2 := &bytes.Buffer{}
				writer, err = compressor.NewWriter(&config.CompressionConfig{Type: algorithm}, buf2)
				assert.NoError(t, err, "error on compressor writer creation", err)
				_, err = writer.Write([]byte(decompressedData2))
				assert.NoError(t, err, "error on compressing data", err)
				err = writer.Close()
				assert.NoError(t, err, "error on compressor Close", err)

				assert.NotEqual(t, decompressedData2, buf2.Bytes(), "the data should have been compressed before sending")

				req, err = http.NewRequest(http.MethodPost,
					fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), buf2)
				assert.NoError(t, err, "error on request creation", err)
				req.Header.Add("Content-Encoding", algorithm)

				resp, err = http.DefaultClient.Do(req)
				assert.NoError(t, err, "error on posting data", err)
				resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode,
					"status should be Internal Server Ok(200)")
			}

			df.mu.Lock()
			assert.Equal(t, expected, df.calledWith,
				"payloads should be decompressed before being sent to dataflow")
			df.mu.Unlock()
		}
	})

	t.Run("Unsupported compressed data is ingested without decompression", func(t *testing.T) {

		df := &mockDataFlow{calledWith: make([][]byte, 0)}

		flws := []flow.Flow{
			{
				Name:                    "flow-1",
				Entrypoint:              df,
				DecompressionAlgorithms: []string{"gzip"},
				CircuitBreaker:          createCircuitBreaker("flow-1", 10*time.Millisecond),
			},
		}

		api := NewAPI(llog, localConf, prometheus.NewRegistry(), testTracer, version, flws)
		srvr := httptest.NewServer(api.mux)
		defer srvr.Close()

		expectedNotCompressed := randString(100)

		bufExpected := &bytes.Buffer{}
		writer, err := compressor.NewWriter(&config.CompressionConfig{Type: "gzip"}, bufExpected)
		assert.NoError(t, err, "error on compressor writer creation", err)
		_, err = writer.Write([]byte(expectedNotCompressed))
		assert.NoError(t, err, "error on compressing data", err)
		err = writer.Close()
		assert.NoError(t, err, "error on compressor Close", err)

		assert.NotEqual(t, expectedNotCompressed, bufExpected.Bytes(), "the data should have been compressed before sending")

		req, err := http.NewRequest(http.MethodPost,
			fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), bufExpected)
		assert.NoError(t, err, "error on request creation", err)
		req.Header.Add("Content-Encoding", "gzip")

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"status should be Internal Server Ok(200) for GZIP")

		expectedCompressed := randString(100)
		bufUnexpected := &bytes.Buffer{}
		writer, err = compressor.NewWriter(&config.CompressionConfig{Type: "snappy"}, bufUnexpected)
		assert.NoError(t, err, "error on compressor writer creation", err)
		_, err = writer.Write([]byte(expectedCompressed))
		assert.NoError(t, err, "error on compressing data", err)
		err = writer.Close()
		assert.NoError(t, err, "error on compressor Close", err)

		assert.NotEqual(t, expectedCompressed, bufUnexpected.Bytes(), "the data should have been compressed before sending")
		secondPayloadBytes := bufUnexpected.Bytes()

		req, err = http.NewRequest(http.MethodPost,
			fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), bufUnexpected)
		assert.NoError(t, err, "error on request creation", err)
		req.Header.Add("Content-Encoding", "snappy")

		resp, err = http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"status should be Internal Server Ok(200)")

		assert.Equal(t, [][]byte{[]byte(expectedNotCompressed), secondPayloadBytes}, df.calledWith,
			"only payloads with supported algorithm should be accepted")
	})
}

func TestDecompressionOnIngestionConcurrencyLimit(t *testing.T) {

	t.Run("when the load is smaller than the limit", func(t *testing.T) {
		algorithm := "snappy"

		df := &mockDataFlow{calledWith: make([][]byte, 0)}

		flws := []flow.Flow{
			{
				Name:                        "flow-1",
				Entrypoint:                  df,
				DecompressionAlgorithms:     []string{algorithm},
				DecompressionMaxConcurrency: 10,
				CircuitBreaker:              createCircuitBreaker("flow-1", 10*time.Millisecond),
			},
		}

		api := NewAPI(llog, localConf, prometheus.NewRegistry(), testTracer, version, flws)
		srvr := httptest.NewServer(api.mux)
		defer srvr.Close()

		expected := make([][]byte, 0)

		decompressedData1 := strings.Repeat("ab", 90)
		expected = append(expected, []byte(decompressedData1))

		buf1 := &bytes.Buffer{}
		writer, err := compressor.NewWriter(&config.CompressionConfig{Type: algorithm}, buf1)
		assert.NoError(t, err, "error on compressor writer creation", err)
		_, err = writer.Write([]byte(decompressedData1))
		assert.NoError(t, err, "error on compressing data", err)
		err = writer.Close()
		assert.NoError(t, err, "error on compressor Close", err)

		req, err := http.NewRequest(http.MethodPost,
			fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), buf1)
		assert.NoError(t, err, "error on request creation", err)
		req.Header.Add("Content-Encoding", algorithm)

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"status should be Internal Server Ok(200)")

		decompressedData2 := randString(300)
		expected = append(expected, []byte(decompressedData2))
		buf2 := &bytes.Buffer{}
		writer, err = compressor.NewWriter(&config.CompressionConfig{Type: algorithm}, buf2)
		assert.NoError(t, err, "error on compressor writer creation", err)
		_, err = writer.Write([]byte(decompressedData2))
		assert.NoError(t, err, "error on compressing data", err)
		err = writer.Close()
		assert.NoError(t, err, "error on compressor Close", err)

		req, err = http.NewRequest(http.MethodPost,
			fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), buf2)
		assert.NoError(t, err, "error on request creation", err)
		req.Header.Add("Content-Encoding", algorithm)

		resp, err = http.DefaultClient.Do(req)
		assert.NoError(t, err, "error on posting data", err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"status should be Internal Server Ok(200)")

		df.mu.Lock()
		assert.Equal(t, expected, df.calledWith, "all data should have been ingested")
		df.mu.Unlock()
	})

	t.Run("when the load is higher than the limit", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping slow test")
		}

		algorithm := "snappy"
		delayFunc := func(_ []byte) error {
			time.Sleep(1 * time.Second)
			return nil
		}

		var wg sync.WaitGroup
		df := &mockDataFlow{calledWith: make([][]byte, 0), enqueueFunc: delayFunc}
		maxConcurrency := 5

		flws := []flow.Flow{
			{
				Name:                        "flow-1",
				Entrypoint:                  df,
				DecompressionAlgorithms:     []string{algorithm},
				DecompressionMaxConcurrency: maxConcurrency,
				CircuitBreaker:              createCircuitBreaker("flow-1", 10*time.Millisecond),
			},
		}

		api := NewAPI(llog, localConf, prometheus.NewRegistry(), testTracer, version, flws)
		srvr := httptest.NewServer(api.mux)
		defer srvr.Close()

		expected := make([][]byte, 0)

		decompressedData1 := strings.Repeat("ab", 90)

		for i := 0; i < maxConcurrency; i++ {
			expected = append(expected, []byte(decompressedData1))
			wg.Add(1)

			go func() {
				buf1 := &bytes.Buffer{}
				writer, err := compressor.NewWriter(&config.CompressionConfig{Type: algorithm}, buf1)
				assert.NoError(t, err, "error on compressor writer creation", err)
				_, err = writer.Write([]byte(decompressedData1))
				assert.NoError(t, err, "error on compressing data", err)
				err = writer.Close()
				assert.NoError(t, err, "error on compressor Close", err)

				req, err := http.NewRequest(http.MethodPost,
					fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), buf1)
				assert.NoError(t, err, "error on request creation", err)
				req.Header.Add("Content-Encoding", algorithm)

				resp, err := http.DefaultClient.Do(req)
				assert.NoError(t, err, "error on posting data", err)
				resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode,
					"status should be Internal Server Ok(200)")
				wg.Done()
			}()
		}

		decompressedData2 := randString(300)
		expected = append(expected, []byte(decompressedData2))
		wg.Add(1)

		go func() {
			buf2 := &bytes.Buffer{}
			writer, err := compressor.NewWriter(&config.CompressionConfig{Type: algorithm}, buf2)
			assert.NoError(t, err, "error on compressor writer creation", err)
			_, err = writer.Write([]byte(decompressedData2))
			assert.NoError(t, err, "error on compressing data", err)
			err = writer.Close()
			assert.NoError(t, err, "error on compressor Close", err)

			req, err := http.NewRequest(http.MethodPost,
				fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), buf2)
			assert.NoError(t, err, "error on request creation", err)
			req.Header.Add("Content-Encoding", algorithm)

			resp, err := http.DefaultClient.Do(req)
			assert.NoError(t, err, "error on posting data", err)
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode,
				"status should be Internal Server Ok(200)")
			wg.Done()
		}()

		wg.Wait()

		df.mu.Lock()
		expectedStr := byteslcToStringslc(expected)
		sort.Strings(expectedStr)
		resultStr := byteslcToStringslc(df.calledWith)
		sort.Strings(resultStr)
		assert.Equal(t, expectedStr,
			resultStr, "all data should have been ingested")
		assert.Lenf(t, df.calledWith, maxConcurrency+1, "all data should have been ingested")
		df.mu.Unlock()
	})
}

func byteslcToStringslc(in [][]byte) []string {
	result := make([]string, 0, len(in))
	for _, btSlc := range in {
		result = append(result, string(btSlc))
	}
	return result
}

func createCircuitBreaker(name string, timeout time.Duration) *gobreaker.TwoStepCircuitBreaker {
	return gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{
		Name:        fmt.Sprintf("%s_ingestion_cb_TEST", name),
		MaxRequests: 1,
		Timeout:     timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return true
		},
	})
}

//TODO: test the graceful shutdown
