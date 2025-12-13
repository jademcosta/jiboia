package app_test

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/app"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var charactersForRandomStringGen = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

type flowSurroundingsMock struct {
	mu               *sync.Mutex
	receivedPayloads [][]byte
	flowName         string
	storageServer    *httptest.Server
	ingestionToken   string
}

func TestAppSeveralRandomFlowsScenario(t *testing.T) {

	payloadCount := 10001 //10k + 1
	flowCount := 1000     //1k

	accumulatorSize := 12  // 10 bytes
	singlePayloadSize := 5 //5 characters, or 5 bytes
	accumulatorSeparator := "-"

	flows := make([]*flowSurroundingsMock, 0, flowCount)

	for i := range flowCount {
		mu := &sync.Mutex{}

		newFlowMock := &flowSurroundingsMock{
			mu:               mu,
			receivedPayloads: make([][]byte, 0),
			flowName:         fmt.Sprintf("flow%d", i),
			ingestionToken:   randomStr(20),
		}

		storageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)

			mu.Lock()
			defer mu.Unlock()
			newFlowMock.receivedPayloads = append(newFlowMock.receivedPayloads, body)
			w.WriteHeader(http.StatusOK)
		}))
		defer storageServer.Close()

		newFlowMock.storageServer = storageServer
		flows = append(flows, newFlowMock)
	}

	conf := config.NewEmpty()

	for _, flow := range flows {
		confFlow := config.FlowConfig{
			Name:                 flow.flowName,
			MaxConcurrentUploads: 2,
			QueueMaxSize:         10,

			Ingestion: config.IngestionConfig{
				Token: flow.ingestionToken,
				Decompression: config.DescompressionConfig{
					MaxConcurrency:       2,
					ActiveDecompressions: []string{"gzip"},
				},
				CircuitBreaker: config.CircuitBreakerConfig{
					Disable: true,
				},
			},
			Accumulator: config.AccumulatorConfig{
				Size:          strconv.Itoa(accumulatorSize),
				Separator:     accumulatorSeparator,
				QueueCapacity: 10,
				CircuitBreaker: config.CircuitBreakerConfig{
					Disable: true,
				},
			},
			ExternalQueues: []config.ExternalQueueConfig{{Type: "noop"}},
			ObjectStorages: []config.ObjectStorageConfig{
				{
					Type:   "httpstorage",
					Config: map[string]interface{}{"url": fmt.Sprintf("%s/%s", flow.storageServer.URL, "%s")},
				},
			},
		}

		conf.Flows = append(conf.Flows, confFlow)
	}

	conf.FillDefaultValues()
	err := conf.Validate()
	require.NoError(t, err, "config should be valid")
	logg := logger.NewDummy()

	app := app.New(conf, logg)
	go app.Start()
	time.Sleep(2 * time.Second)

	payloads := make([]string, 0, payloadCount)
	for range payloadCount {
		payload := randomStr(singlePayloadSize)
		payloads = append(payloads, payload)
	}

	sendPayloadsToRandomFlow(t, payloads, flows)
	time.Sleep(500 * time.Millisecond)

	stopDone := app.Stop()
	<-stopDone
	//Giving time in case the obj storages didn't processed all data
	time.Sleep(500 * time.Millisecond)

	receivedContent := make([]string, 0, len(payloads))
	for _, flow := range flows {
		flow.mu.Lock()

		for _, data := range flow.receivedPayloads {
			splittedData := bytes.Split(data, []byte(accumulatorSeparator))
			receivedContent = append(receivedContent, byteSlicesToStrSlice(splittedData)...)
		}
		flow.mu.Unlock()
	}

	assert.Len(t, receivedContent, payloadCount, "all the payloads should have been sent")

	slices.Sort(payloads)
	slices.Sort(receivedContent)
	assert.Equal(t, payloads, receivedContent,
		"the exact (sent) data should have been received on storages")
}

func sendPayloadsToRandomFlow(
	t *testing.T, payloads []string, flows []*flowSurroundingsMock,
) {
	httpCli := http.Client{
		Timeout: 1 * time.Second,
	}

	for idx, payload := range payloads {

		pickedFlow := pickRandomFlow(flows)

		flowURL := fmt.Sprintf("http://localhost:%d/%s/async_ingestion", config.DefaultPort, pickedFlow.flowName)
		req, err := http.NewRequest(http.MethodPost, flowURL, strings.NewReader(payload))
		assert.NoError(t, err, "creating request should be successful, request number %d", idx)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", pickedFlow.ingestionToken))

		response, err := httpCli.Do(req)
		assert.NoError(t, err, "enqueueing items should not return error on flow, request number %d", idx)
		assert.Equal(t, 200, response.StatusCode, "data enqueueing should have been successful on flow")
		response.Body.Close()
	}
}

func pickRandomFlow(flows []*flowSurroundingsMock) *flowSurroundingsMock {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	return flows[r1.Intn(len(flows))]
}

func byteSlicesToStrSlice(byteSlices [][]byte) []string {
	strSlice := make([]string, 0, len(byteSlices))
	for _, bSlice := range byteSlices {
		strSlice = append(strSlice, string(bSlice))
	}
	return strSlice
}

func randomStr(characterCount int) string {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]rune, characterCount)

	for i := range b {
		b[i] = charactersForRandomStringGen[r1.Intn(len(charactersForRandomStringGen))]
	}
	return string(b)
}
