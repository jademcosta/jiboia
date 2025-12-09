package app

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppHighConcurrencyScenario(t *testing.T) {

	amountOfPayloads := 10000 //10k

	var mu1 sync.Mutex
	var mu2 sync.Mutex
	objStorageReceivedFlow1 := make([][]byte, 0, amountOfPayloads)
	objStorageReceivedFlow2 := make([][]byte, 0, amountOfPayloads)
	var wg sync.WaitGroup

	storageServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		body, _ := io.ReadAll(r.Body)
		mu1.Lock()
		defer mu1.Unlock()
		objStorageReceivedFlow1 = append(objStorageReceivedFlow1, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer storageServer1.Close()

	storageServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		body, _ := io.ReadAll(r.Body)
		mu2.Lock()
		defer mu2.Unlock()
		objStorageReceivedFlow2 = append(objStorageReceivedFlow2, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer storageServer2.Close()

	confFull := strings.Replace(conf4HighConcurrencyYaml, "http://non-existent-flow5-1.com",
		fmt.Sprintf("%s/%s", storageServer1.URL, "%s"), 1)
	confFull = strings.Replace(confFull, "http://non-existent-flow5-2.com",
		fmt.Sprintf("%s/%s", storageServer2.URL, "%s"), 1)

	conf, err := config.New([]byte(confFull))
	require.NoError(t, err, "should initialize config")
	logg = logger.New(&conf.O11y.Log)

	app := New(conf, logg)
	go app.Start()
	time.Sleep(2 * time.Second)

	payloads := make([]string, 0, amountOfPayloads)
	for i := 0; i < amountOfPayloads; i++ {
		payload := randSeq(21)
		payloads = append(payloads, payload)
	}

	wg.Add(amountOfPayloads * 2) //times 2 because we have 2 obj storages on this flow
	allPayloadsSent := make(chan struct{})
	go sendPayloadsToJiboia(t, payloads, allPayloadsSent)
	<-allPayloadsSent
	//Wait for all payloads to be processed by "mocked external" HTTP receives
	wg.Wait()

	stopDone := app.Stop()
	<-stopDone

	mu1.Lock()
	mu2.Lock()
	defer mu1.Unlock()
	defer mu2.Unlock()

	receivedValuesOnFirstObjStorage := make([]string, 0, len(objStorageReceivedFlow1))
	for _, data := range objStorageReceivedFlow1 {
		receivedValuesOnFirstObjStorage = append(receivedValuesOnFirstObjStorage, string(data))
	}

	receivedValuesOnSecondObjStorage := make([]string, 0, len(objStorageReceivedFlow2))
	for _, data := range objStorageReceivedFlow2 {
		receivedValuesOnSecondObjStorage = append(receivedValuesOnSecondObjStorage, string(data))
	}

	assert.ElementsMatch(t, payloads, receivedValuesOnFirstObjStorage,
		"all the data sent should have been received on the first obj storage")
	assert.ElementsMatch(t, payloads, receivedValuesOnSecondObjStorage,
		"all the data sent should have been received on the second obj storage")
}

func sendPayloadsToJiboia(t *testing.T, payloads []string, allPayloadSent chan struct{}) {
	defer close(allPayloadSent)
	maxConcurrency := 10
	concurrencyLimiter := make(chan struct{}, maxConcurrency)
	defer close(concurrencyLimiter)

	for idx, payload := range payloads {
		concurrencyLimiter <- struct{}{}

		go func() {
			response, err := http.Post("http://localhost:9099/int_flow5/async_ingestion", "application/json", strings.NewReader(payload))
			assert.NoError(t, err, "enqueueing items should not return error on flow, request number %d", idx)
			assert.Equal(t, 200, response.StatusCode, "data enqueueing should have been successful on flow")
			response.Body.Close()
			<-concurrencyLimiter
		}()
	}
}
