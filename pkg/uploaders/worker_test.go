package uploaders_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/jademcosta/jiboia/pkg/uploaders"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var l *zap.SugaredLogger

func init() {
	l = logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})
}

type mockObjStorage struct {
	mu        sync.Mutex
	wg        *sync.WaitGroup
	workU     []*domain.WorkUnit
	returning *domain.UploadResult
}

func (objStorage *mockObjStorage) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()

	objStorage.workU = append(objStorage.workU, workU)
	objStorage.wg.Done()

	if objStorage.returning != nil {
		return objStorage.returning, nil
	}
	return &domain.UploadResult{}, nil
}

type dummyObjStorage struct{}

func (queue *dummyObjStorage) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {
	return nil, nil
}

type mockExternalQueue struct {
	calledWith []*domain.UploadResult
	mu         sync.Mutex
	wg         *sync.WaitGroup
}

func (queue *mockExternalQueue) Enqueue(data *domain.UploadResult) error {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	defer queue.wg.Done()

	queue.calledWith = append(queue.calledWith, data)
	return nil
}

type dummyExternalQueue struct{}

func (queue *dummyExternalQueue) Enqueue(data *domain.UploadResult) error {
	return nil
}

func TestRegistersItsChannelOnStartup(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	objStorage := &dummyObjStorage{}

	queue := &dummyExternalQueue{}

	workerQueueChan := make(chan chan *domain.WorkUnit, 1)

	sut := uploaders.NewWorker(l, objStorage, queue, workerQueueChan, prometheus.NewRegistry())
	go sut.Run(ctx)

	select {
	case <-workerQueueChan:
		// Success
	case <-time.After(1 * time.Millisecond):
		assert.Fail(t, "should register itself on workChannel when Run is called.")
	}
	cancel()
}

func TestCallsObjUploaderWithDataPassed(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorage := &mockObjStorage{
		workU: make([]*domain.WorkUnit, 0),
		wg:    &wg,
	}
	queue := &dummyExternalQueue{}

	workerQueueChan := make(chan chan *domain.WorkUnit, 1)

	sut := uploaders.NewWorker(l, objStorage, queue, workerQueueChan, prometheus.NewRegistry())
	go sut.Run(ctx)

	var workerChan chan *domain.WorkUnit

	select {
	case workerChan = <-workerQueueChan:
		// Success
	case <-time.After(1 * time.Millisecond):
		assert.Fail(t, "should register itself on workChannel when Run is called.")
	}

	workU := &domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some data"),
	}

	wg.Add(1)
	workerChan <- workU
	time.Sleep(1 * time.Millisecond)

	wg.Wait()
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.workU, 1, "should have called uploader")
	assert.Same(t, workU, objStorage.workU[0], "should have called objUploader with the correct data")

	cancel()
}

func TestCallsEnqueuerWithUploaderResult(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	uploadResult := &domain.UploadResult{
		Bucket:      "some-bucket",
		Region:      "some-region",
		Path:        "some/path/file.txt",
		SizeInBytes: 1234}
	objStorage := &mockObjStorage{
		workU:     make([]*domain.WorkUnit, 0),
		wg:        &wg,
		returning: uploadResult,
	}

	queue := &mockExternalQueue{calledWith: make([]*domain.UploadResult, 0), wg: &wg}
	workerQueueChan := make(chan chan *domain.WorkUnit, 1)

	sut := uploaders.NewWorker(l, objStorage, queue, workerQueueChan, prometheus.NewRegistry())
	go sut.Run(ctx)

	var workerChan chan *domain.WorkUnit

	select {
	case workerChan = <-workerQueueChan:
		// Success
	case <-time.After(1 * time.Millisecond):
		assert.Fail(t, "should register itself on workChannel when Run is called.")
	}

	workU := &domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some data"),
	}

	wg.Add(2)
	workerChan <- workU
	time.Sleep(1 * time.Millisecond)

	wg.Wait()
	queue.mu.Lock()
	defer queue.mu.Unlock()
	assert.Len(t, queue.calledWith, 1, "should have called enqueuer")
	assert.Same(t, uploadResult, queue.calledWith[0], "should have called enqueuer with the correct data")

	cancel()
}

func TestRegistersItselfForWorkAgainAfterWorking(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorage := &mockObjStorage{
		workU: make([]*domain.WorkUnit, 0),
		wg:    &wg,
	}
	queue := &dummyExternalQueue{}
	workerQueueChan := make(chan chan *domain.WorkUnit, 1)

	sut := uploaders.NewWorker(l, objStorage, queue, workerQueueChan, prometheus.NewRegistry())
	go sut.Run(ctx)

	wg.Add(11)
	var workerChan chan *domain.WorkUnit

	for i := 0; i < 11; i++ {
		select {
		case workerChan = <-workerQueueChan:
			// Success
		case <-time.After(10 * time.Millisecond):
			assert.Fail(t, "should register itself on workChannel when Run is called.")
		}

		workU := &domain.WorkUnit{
			Filename: "some-filename",
			Prefix:   "some-prefix",
			Data:     []byte(fmt.Sprint(i)),
		}
		workerChan <- workU
	}

	wg.Wait()
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.workU, 11, "should have called uploader")

	cancel()
}

func TestStopsAcceptingWorkAfterContextIsCancelled(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorage := &mockObjStorage{
		workU: make([]*domain.WorkUnit, 0),
		wg:    &wg,
	}
	queue := &dummyExternalQueue{}
	workerQueueChan := make(chan chan *domain.WorkUnit, 1)

	sut := uploaders.NewWorker(l, objStorage, queue, workerQueueChan, prometheus.NewRegistry())
	go sut.Run(ctx)

	var workerChan chan *domain.WorkUnit

	select {
	case workerChan = <-workerQueueChan:
		// Success
	case <-time.After(1 * time.Millisecond):
		assert.Fail(t, "should register itself on workChannel when Run is called.")
	}

	workU := &domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some data"),
	}

	wg.Add(2)
	workerChan <- workU
	time.Sleep(1 * time.Millisecond)

	cancel()

	select {
	case workerChan = <-workerQueueChan:
		// Success
	case <-time.After(1 * time.Millisecond):
		assert.Fail(t, "should register itself on workChannel when Run is called.")
	}

	workerChan <- workU

	time.Sleep(1 * time.Millisecond)

	select {
	case <-workerQueueChan:
		assert.Fail(t, "should not register itself on workChannel after cancel is called.")
	case <-time.After(1 * time.Millisecond):
		//Success
	}

	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.workU, 1, "should have called uploader")
	assert.Same(t, workU, objStorage.workU[0], "should have called objUploader with the correct data")
}
