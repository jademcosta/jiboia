package http_in

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

const version string = "0.0.0"

type mockDataFlow struct {
	calledWith [][]byte
	mu         sync.Mutex
}

func (mockDF *mockDataFlow) Enqueue(data []byte) error {
	mockDF.mu.Lock()
	defer mockDF.mu.Unlock()
	mockDF.calledWith = append(mockDF.calledWith, data)
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
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "error", Format: "json"}})
	c := config.ApiConfig{Port: 9111}

	mockDF := &mockDataFlow{
		calledWith: make([][]byte, 0),
	}

	mockDF2 := &mockDataFlow{
		calledWith: make([][]byte, 0),
	}

	flws := []flow.Flow{
		{
			Name:       "flow-1",
			Entrypoint: mockDF,
		},
		{
			Name:       "flow2",
			Entrypoint: mockDF2,
		},
	}

	api := New(l, c, prometheus.NewRegistry(), version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json", strings.NewReader("helloooooo"))
	if err != nil {
		assert.Fail(t, "error on posting data to api", err)
	}
	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200) on flow 1")

	resp, err = http.Post(fmt.Sprintf("%s/flow2/async_ingestion", srvr.URL), "application/json", strings.NewReader("world!"))
	if err != nil {
		assert.Fail(t, "error on posting data to flow 2", err)
	}
	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be OK(200) on flow 2")

	assert.Equal(t, [][]byte{[]byte("helloooooo")}, mockDF.calledWith, "the posted data should have been sent to flow 1")
	assert.Equal(t, [][]byte{[]byte("world!")}, mockDF2.calledWith, "the posted data should have been sent to flow 2")
}

func TestAnswersAnErrorIfNoBodyIsSent(t *testing.T) {
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "error", Format: "json"}})
	c := config.ApiConfig{Port: 9111}

	mockDF := &mockDataFlow{
		calledWith: make([][]byte, 0),
	}

	flws := []flow.Flow{
		{
			Name:       "flow-1",
			Entrypoint: mockDF,
		},
	}

	api := New(l, c, prometheus.NewRegistry(), version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json", strings.NewReader(""))

	if err != nil {
		assert.Fail(t, "error on posting data", err)
	}

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "status should be Bad Request(400)")
	assert.Lenf(t, mockDF.calledWith, 0, "no data should have been sent to flow")
}

func TestAnswersErrorIfEnqueueingFails(t *testing.T) {
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "error", Format: "json"}})
	c := config.ApiConfig{Port: 9111}

	mockDF := &dummyAlwaysFailDataFlow{}

	flws := []flow.Flow{
		{
			Name:       "flow-1",
			Entrypoint: mockDF,
		},
	}

	api := New(l, c, prometheus.NewRegistry(), version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json", strings.NewReader("some data"))

	if err != nil {
		assert.Fail(t, "error on posting data", err)
	}

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode, "status should be Internal Server Error(500)")
}

func TestPanicResultInStatus500(t *testing.T) {
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "error", Format: "json"}})
	c := config.ApiConfig{Port: 9111}

	brokenDF := &brokenDataFlow{}

	flws := []flow.Flow{
		{
			Name:       "flow-1",
			Entrypoint: brokenDF,
		},
	}
	api := New(l, c, prometheus.NewRegistry(), version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json", strings.NewReader("some data"))
	if err != nil {
		assert.Fail(t, "error on posting data", err)
	}

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode,
		"status should be Internal Server Error(500) when panics occur")
}

func TestPayloadSizeLimit(t *testing.T) {
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "error", Format: "json"}})
	c := config.ApiConfig{Port: 9111, PayloadSizeLimit: "10"} //10 bytes limit

	df := &mockDataFlow{calledWith: make([][]byte, 0)}

	flws := []flow.Flow{
		{
			Name:       "flow-1",
			Entrypoint: df,
		},
	}

	api := New(l, c, prometheus.NewRegistry(), version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json",
		strings.NewReader("somedata"))
	if err != nil {
		assert.Fail(t, "error on posting data", err)
	}
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"status should be Internal Server Ok(200) when size is within limits")

	resp, err = http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json",
		strings.NewReader("1111111111")) //10 bytes
	if err != nil {
		assert.Fail(t, "error on posting data", err)
	}
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"status should be Internal Server Ok(200) when size is within limits")

	resp, err = http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json",
		strings.NewReader("abcdefghijk")) //11 bytes
	if err != nil {
		assert.Fail(t, "error on posting data", err)
	}
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode,
		"status should be Internal Server Payload too large(413) when size is above limits (11 bytes)")

	resp, err = http.Post(fmt.Sprintf("%s/flow-1/async_ingestion", srvr.URL), "application/json",
		strings.NewReader("abcdefghijkl")) //12 bytes
	if err != nil {
		assert.Fail(t, "error on posting data", err)
	}
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode,
		"status should be Internal Server Payload too large(413) when size is above limits (12 bytes)")

	assert.Equal(t, [][]byte{[]byte("somedata"), []byte("1111111111")}, df.calledWith,
		"only allowed payloads should be ingested")
}

func TestVersionEndpointInformsTheVersion(t *testing.T) {
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "error", Format: "json"}})
	c := config.ApiConfig{Port: 9111}

	mockDF := &dummyAlwaysFailDataFlow{}

	flws := []flow.Flow{
		{
			Name:       "flow-1",
			Entrypoint: mockDF,
		},
	}

	api := New(l, c, prometheus.NewRegistry(), version, flws)
	srvr := httptest.NewServer(api.mux)
	defer srvr.Close()

	resp, err := http.Get(fmt.Sprintf("%s/version", srvr.URL))

	if err != nil {
		assert.Fail(t, "error on getting data", err)
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	assert.NoError(t, err, "should not err when reading body from http")
	defer resp.Body.Close()
	body := buf.String()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "status should be Ok(200)")
	assert.Equal(t, fmt.Sprintf("{\"version\":\"%s\"}", version), body, "version informed should be the current one")
}

//TODO: test the graceful shutdown
//TODO: add tests for metrics serving
