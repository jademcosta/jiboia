package uploader

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/jademcosta/jiboia/pkg/worker"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

var noCompressionConf config.CompressionConfig = config.CompressionConfig{}
var llog = logger.NewDummy()

type mockFilePather struct {
	prefix   string
	filename string
}

func (mock *mockFilePather) Filename() *string {
	return &mock.filename
}

func (mock *mockFilePather) Prefix() *string {
	return &mock.prefix
}

type dummyExternalQueue struct{}

func (queue *dummyExternalQueue) Enqueue(_ *domain.MessageContext) error {
	return nil
}

type mockCounterObjStorage struct {
	returnError    bool
	counter        *int64
	uploadDuration time.Duration
	wg             *sync.WaitGroup
}

func (objStorage *mockCounterObjStorage) Upload(_ *domain.WorkUnit) (*domain.UploadResult, error) {
	defer objStorage.wg.Done()
	if objStorage.returnError {
		return nil, fmt.Errorf("Error from mockCounterObjStorage")
	}

	time.Sleep(objStorage.uploadDuration) //Simulate some work
	atomic.AddInt64(objStorage.counter, 1)

	return &domain.UploadResult{}, nil
}

type mockObjStorageWithAppend struct {
	wg   *sync.WaitGroup
	work func(workU *domain.WorkUnit)
}

func (objStorage *mockObjStorageWithAppend) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {
	defer objStorage.wg.Done()
	objStorage.work(workU)
	return &domain.UploadResult{}, nil
}

func TestWorkSentDownstreamHasTheCorrectDataInIt(t *testing.T) {
	workersCount := 2
	capacity := 3
	nextQueue := make(chan *domain.WorkUnit, 10)

	ctx, cancel := context.WithCancel(context.Background())

	sut := New(
		"someflow",
		llog,
		workersCount,
		capacity,
		&mockFilePather{prefix: "some-prefix", filename: "some-random-filename"},
		prometheus.NewRegistry(),
		nextQueue,
	)

	go sut.Run(ctx)

	err := sut.Enqueue([]byte("1"))
	assert.NoError(t, err, "should not err on enqueue")

	select {
	case work := <-nextQueue:
		assert.Equal(t, "some-prefix", work.Prefix, "should have used the prefix from filepather")
		assert.Equal(t, "some-random-filename", work.Filename, "should have used the filename from filepather")
	case <-time.After(10 * time.Millisecond):
		assert.Fail(t, "uploader should have distributed work")
	}

	cancel()
	<-sut.Done()
}

func TestItDeniesWorkAfterContextIsCanceled(t *testing.T) {
	workersCount := 10
	capacity := 3
	nextQueue := make(chan *domain.WorkUnit, 10)

	ctx, cancel := context.WithCancel(context.Background())

	sut := New(
		"someflow",
		llog,
		workersCount,
		capacity,
		&mockFilePather{prefix: "some-prefix", filename: "some-random-filename"},
		prometheus.NewRegistry(),
		nextQueue,
	)

	go sut.Run(ctx)

	err := sut.Enqueue([]byte("1"))
	assert.NoError(t, err, "should not err on enqueue")

	select {
	case <-nextQueue:
		// Success
	case <-time.After(10 * time.Millisecond):
		assert.Fail(t, "uploader should have distributed work")
	}

	cancel()
	<-sut.Done()
	err = sut.Enqueue([]byte("2"))
	assert.Error(t, err, "enqueue should return error when context has been canceled")
}

func TestItFlushesAllPendingDataWhenContextIsCancelled(t *testing.T) {
	workersCount := 2
	capacity := 3
	nextQueue := make(chan *domain.WorkUnit, 10)
	ctx, cancel := context.WithCancel(context.Background())

	sut := New(
		"someflow",
		llog,
		workersCount,
		capacity,
		&mockFilePather{prefix: "some-prefix", filename: "some-random-filename"},
		prometheus.NewRegistry(),
		nextQueue,
	)

	go sut.Run(ctx)
	err := sut.Enqueue([]byte("1"))
	assert.NoError(t, err, "should not err on enqueue")
	err = sut.Enqueue([]byte("2"))
	assert.NoError(t, err, "should not err on enqueue")
	time.Sleep(10 * time.Millisecond)

	cancel()
	<-sut.Done()

	select {
	case work := <-nextQueue:
		assert.Equal(t, []byte("1"), work.Data, "should have sent the correct data to be worked")
	case <-time.After(10 * time.Millisecond):
		assert.Fail(t, "uploader should have distributed work")
	}

	select {
	case work := <-nextQueue:
		assert.Equal(t, []byte("2"), work.Data, "should have sent the correct data to be worked")
	case <-time.After(10 * time.Millisecond):
		assert.Fail(t, "uploader should have distributed work")
	}

	err = sut.Enqueue([]byte("3"))
	assert.Error(t, err, "should err on enqueue")

	select {
	case work := <-nextQueue:
		assert.Nil(t, work, "uploader should not have distributed work. work is: %s", work)
	case <-time.After(10 * time.Millisecond):
		// Success
	}
}

func TestUploadersSendAllEnqueuedItems(t *testing.T) {
	// TODO: use t.Run? Parallel?
	// workersCount, objectsCount, capacity
	testUploaderEnsuringEnqueuedItems(t, 10, 60, 60, New)
	testUploaderEnsuringEnqueuedItems(t, 10, 300, 300, New)
	testUploaderEnsuringEnqueuedItems(t, 1, 60, 60, New)
	testUploaderEnsuringEnqueuedItems(t, 10, 2, 2, New)
	testUploaderEnsuringEnqueuedItems(t, 10, 1, 1, New)
	testUploaderEnsuringEnqueuedItems(t, 90, 60, 60, New)
	testUploaderEnsuringEnqueuedItems(t, 90, 2, 2, New)
}

func TestUploadersSendAllEnqueueItemsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	//TODO: (jademcosta) remove the New necessity here. It exists from a time were it had multiple uploader implementations
	// time to proccess an upload, workersCount, objectsCount, capacity, factoryFn, sleepBetweenProduction
	testUploader(t, 50*time.Millisecond, 10, 60, 60, New, 0)
	testUploader(t, 50*time.Millisecond, 1, 30, 30, New, 0)
	testUploader(t, 50*time.Millisecond, 60, 1, 1, New, 0)
	testUploader(t, 50*time.Millisecond, 90, 60, 60, New, 0)
	testUploader(t, 100*time.Millisecond, 10, 60, 60, New, 0)
	testUploader(t, 10*time.Millisecond, 30, 600, 600, New, 0)
	testUploader(t, 10*time.Millisecond, 300, 100, 100, New, 0)

	testUploader(t, 10*time.Millisecond, 10, 60, 60, New, 1*time.Millisecond)
	testUploader(t, 10*time.Millisecond, 300, 60, 60, New, 1*time.Millisecond)
	testUploader(t, 10*time.Millisecond, 1, 30, 30, New, 1*time.Millisecond)
	testUploader(t, 10*time.Millisecond, 30, 2, 2, New, 1*time.Millisecond)
	testUploader(t, 10*time.Millisecond, 10, 45, 40, New, 1*time.Millisecond)
}

func testUploader(
	t *testing.T,
	uploadDuration time.Duration,
	workersCount int,
	objectsToEnqueueCount int,
	capacity int,
	uploaderFactory func(string, *slog.Logger, int, int, domain.FilePathProvider, *prometheus.Registry, chan *domain.WorkUnit) *NonBlockingUploader,
	sleepTimeBeforeProducing time.Duration) {

	resultCounter := int64(0)
	var waitG sync.WaitGroup
	nextQueue := make(chan *domain.WorkUnit, 10)

	ctx, cancel := context.WithCancel(context.Background())

	sut := uploaderFactory(
		"someflow", llog, workersCount, capacity, &mockFilePather{}, prometheus.NewRegistry(), nextQueue,
	)

	waitG.Add(objectsToEnqueueCount)
	for i := 0; i < workersCount; i++ {
		objStorage := &mockCounterObjStorage{
			uploadDuration: uploadDuration,
			counter:        &resultCounter,
			wg:             &waitG,
		}

		w := worker.NewWorker("someflow", llog, objStorage, []worker.ExternalQueue{&dummyExternalQueue{}},
			nextQueue, prometheus.NewRegistry(), noCompressionConf, time.Now)
		go w.Run(ctx)
	}

	go sut.Run(ctx)

	for i := 0; i < objectsToEnqueueCount; i++ {
		err := sut.Enqueue([]byte(fmt.Sprint(i)))
		assert.NoError(t, err, "should not err on enqueue")
		if sleepTimeBeforeProducing != 0 {
			time.Sleep(sleepTimeBeforeProducing)
		}
	}

	uploadCountsConsideringParallellism := math.Ceil(float64(objectsToEnqueueCount) / float64(workersCount))
	time.Sleep(time.Duration(uploadCountsConsideringParallellism) * uploadDuration)
	time.Sleep(2 * uploadDuration) //To make sure
	waitG.Wait()

	testErrorString := fmt.Sprintf(
		"count of uploaded results don't match. Scenario - Workers count: %d, Requests count: %d, Duration: %d",
		workersCount, objectsToEnqueueCount, uploadDuration)
	assert.Equal(t, int64(objectsToEnqueueCount), resultCounter, testErrorString)
	cancel()
	<-sut.Done()
}

func testUploaderEnsuringEnqueuedItems(
	t *testing.T,
	workersCount int,
	objectsToEnqueueCount int,
	capacity int,
	uploaderFactory func(string, *slog.Logger, int, int, domain.FilePathProvider, *prometheus.Registry, chan *domain.WorkUnit) *NonBlockingUploader,
) {

	var waitG sync.WaitGroup
	var mu sync.Mutex
	expected := make([]string, objectsToEnqueueCount)
	result := make([]string, 0, objectsToEnqueueCount)
	nextQueue := make(chan *domain.WorkUnit, 10)

	ctx, cancel := context.WithCancel(context.Background())

	sut := uploaderFactory(
		"someflow", llog, workersCount, capacity, &mockFilePather{}, prometheus.NewRegistry(), nextQueue,
	)

	waitG.Add(objectsToEnqueueCount)
	for i := 0; i < workersCount; i++ {
		objStorage := &mockObjStorageWithAppend{
			wg: &waitG,
			work: func(workU *domain.WorkUnit) {
				mu.Lock()
				defer mu.Unlock()
				result = append(result, string(workU.Data))
			},
		}

		w := worker.NewWorker("someflow", llog, objStorage, []worker.ExternalQueue{&dummyExternalQueue{}},
			nextQueue, prometheus.NewRegistry(), noCompressionConf, time.Now)
		go w.Run(ctx)
	}

	go sut.Run(ctx)

	for i := 0; i < objectsToEnqueueCount; i++ {
		err := sut.Enqueue([]byte(fmt.Sprint(i)))
		assert.NoError(t, err, "should not err on enqueue")
		expected[i] = fmt.Sprint(i)
	}

	waitG.Wait()

	sort.Strings(result)
	sort.Strings(expected)

	testErrorString := fmt.Sprintf(
		"uploaded data don't match. Scenario - Workers count: %d, Requests count: %d",
		workersCount, objectsToEnqueueCount)
	assert.Equal(t, expected, result, testErrorString)
	cancel()
	<-sut.Done()
}

//TODO: ele fecha o next no shutdown
