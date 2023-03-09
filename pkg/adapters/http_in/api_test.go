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

func TestPassesDataFlows(t *testing.T) {
	l := logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})
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
		assert.Fail(t, "error on posting data to flow 1", err)
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
