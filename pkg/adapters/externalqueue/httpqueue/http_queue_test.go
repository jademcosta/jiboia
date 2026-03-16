package httpqueue

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jademcosta/jiboia/pkg/adapters/externalqueue/queuemessage"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configYaml = `
url: http://example.com/queue
timeout_milliseconds: 5000
`

const fakeFlowName = "some-flow-name"

func TestNewReturnsErrorOnEmptyURL(t *testing.T) {
	c := &Config{URL: ""}
	l := logger.NewDummy()

	_, err := New(l, c, fakeFlowName)
	assert.Error(t, err, "should return error when url is empty")
}

func TestNewSucceedsWithValidConfig(t *testing.T) {
	c := &Config{URL: "http://example.com/queue"}
	l := logger.NewDummy()

	q, err := New(l, c, fakeFlowName)
	require.NoError(t, err, "should not return error with valid config")
	assert.NotNil(t, q)
}

func TestTypeReturnsHTTP(t *testing.T) {
	c := &Config{URL: "http://example.com/queue"}
	l := logger.NewDummy()

	q, err := New(l, c, fakeFlowName)
	require.NoError(t, err)
	assert.Equal(t, TYPE, q.Type())
}

func TestNameReturnsFlowName(t *testing.T) {
	c := &Config{URL: "http://example.com/queue"}
	l := logger.NewDummy()

	flowName := "my-flow"
	q, err := New(l, c, flowName)
	require.NoError(t, err)
	assert.Equal(t, flowName, q.Name())
}

func TestEnqueueSendsCorrectJSONPayload(t *testing.T) {
	var receivedBody []byte
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &Config{URL: server.URL}
	l := logger.NewDummy()

	q, err := New(l, c, "this-flow-name")
	require.NoError(t, err)

	err = q.Enqueue(&domain.MessageContext{
		Bucket:          "my-bucket1",
		Region:          "region-a",
		Path:            "filepath",
		URL:             "some_url",
		SizeInBytes:     1111,
		CompressionType: "some-compression",
	})
	require.NoError(t, err, "should not err on enqueue")

	assert.Equal(t, "application/json", receivedContentType)

	expectedMsg := queuemessage.Message{
		SchemaVersion: domain.MsgSchemaVersion,
		FlowName:      "this-flow-name",
		Bucket:        queuemessage.Bucket{Name: "my-bucket1", Region: "region-a"},
		Object: queuemessage.Object{
			Path:            "filepath",
			FullURL:         "some_url",
			SizeInBytes:     1111,
			CompressionType: "some-compression",
		},
	}
	expectedJSON, err := json.Marshal(expectedMsg)
	require.NoError(t, err)
	assert.JSONEq(t, string(expectedJSON), string(receivedBody),
		"the correct message format and content should have been sent")
}

func TestEnqueueUsesPostMethod(t *testing.T) {
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &Config{URL: server.URL}
	l := logger.NewDummy()

	q, err := New(l, c, fakeFlowName)
	require.NoError(t, err)

	err = q.Enqueue(&domain.MessageContext{
		Bucket: "my-bucket",
		Region: "us-east-1",
		Path:   "some/path",
		URL:    "some_url",
	})
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, receivedMethod)
}

func TestEnqueueReturnsErrorOnNon2xxResponse(t *testing.T) {
	for _, statusCode := range []int{http.StatusBadRequest, http.StatusInternalServerError, http.StatusNotFound} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(statusCode)
			}))
			defer server.Close()

			c := &Config{URL: server.URL}
			l := logger.NewDummy()

			q, err := New(l, c, fakeFlowName)
			require.NoError(t, err)

			err = q.Enqueue(&domain.MessageContext{
				Bucket: "my-bucket",
				Region: "us-east-1",
				Path:   "some/path",
				URL:    "some_url",
			})
			assert.Errorf(t, err, "should return error on %d status code response", statusCode)
		})
	}
}

func TestEnqueueReturnsErrorOnRequestFailure(t *testing.T) {
	c := &Config{URL: "http://127.0.0.1:1"} // nothing listening there
	l := logger.NewDummy()

	q, err := New(l, c, fakeFlowName)
	require.NoError(t, err)

	err = q.Enqueue(&domain.MessageContext{
		Bucket: "my-bucket",
		Region: "us-east-1",
		Path:   "some/path",
		URL:    "some_url",
	})
	assert.Error(t, err, "should return error when request fails")
}

func TestParseConfig(t *testing.T) {
	confResult, err := ParseConfig([]byte(configYaml))

	require.NoError(t, err, "should not return error from config parsing")
	assert.Equal(t, "http://example.com/queue", confResult.URL, "queue URL doesn't match")
	assert.Equal(t, 5000, confResult.TimeoutMilliseconds, "timeout doesn't match")
}

func TestDefaultTimeoutIsUsedWhenNotSet(t *testing.T) {
	c := &Config{URL: "http://example.com/queue"}
	l := logger.NewDummy()

	q, err := New(l, c, fakeFlowName)
	require.NoError(t, err)

	httpClient, ok := q.client.(*http.Client)
	require.True(t, ok, "client should be *http.Client")
	assert.Equal(t, int64(defaultTimeoutMilliseconds), httpClient.Timeout.Milliseconds())
}
