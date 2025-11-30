// nolint: forcetypeassert
package worker_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/compression"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/jademcosta/jiboia/pkg/worker"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func constantTimeProvider(fixedTime time.Time) func() time.Time {
	return func() time.Time {
		return fixedTime
	}
}

var currentTime = time.Now()
var noCompressionConf config.CompressionConfig = config.CompressionConfig{}
var llog = logger.NewDummy()

type mockObjStorage struct {
	mu         sync.Mutex
	wg         *sync.WaitGroup
	calledWith []*domain.WorkUnit
	returning  *domain.UploadResult
	err        error
}

func (objStorage *mockObjStorage) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()

	objStorage.calledWith = append(objStorage.calledWith, workU)
	objStorage.wg.Done()

	if objStorage.err != nil {
		return nil, objStorage.err
	}

	if objStorage.returning != nil {
		return objStorage.returning, nil
	}
	return &domain.UploadResult{}, nil
}

type dummyObjStorage struct{}

func (objStorage *dummyObjStorage) Upload(_ *domain.WorkUnit) (*domain.UploadResult, error) {
	return &domain.UploadResult{}, nil
}

type mockExternalQueue struct {
	calledWith []*domain.MessageContext
	mu         sync.Mutex
	wg         *sync.WaitGroup
	err        error
}

func (queue *mockExternalQueue) Enqueue(data *domain.MessageContext) error {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	defer queue.wg.Done()

	queue.calledWith = append(queue.calledWith, data)
	return queue.err
}

type dummyExternalQueue struct{}

func (queue *dummyExternalQueue) Enqueue(_ *domain.MessageContext) error {
	return nil
}

func TestPanicsIfCreatedWithDifferentCountOfStoragesAndQueues(t *testing.T) {
	workChan := make(chan *domain.WorkUnit, 2)

	objStorages := []worker.ObjStorage{&dummyObjStorage{}, &dummyObjStorage{}, &dummyObjStorage{}}
	queues := []worker.ExternalQueue{&dummyExternalQueue{}, &dummyExternalQueue{}, &dummyExternalQueue{}}
	//Check it doesn't panics when counts are the same
	_ = worker.NewWorker("someflow", llog, objStorages, queues, workChan,
		prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))

	objStorages = []worker.ObjStorage{&dummyObjStorage{}, &dummyObjStorage{}}
	queues = []worker.ExternalQueue{&dummyExternalQueue{}, &dummyExternalQueue{}, &dummyExternalQueue{}}

	assert.Panics(
		t,
		func() {
			worker.NewWorker("someflow", llog, objStorages, queues, workChan,
				prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))
		},
		"panics when counts of storages and queues differ",
	)
}

func TestCallsObjUploaderWithDataPassed(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorages := []worker.ObjStorage{
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			wg:         &wg,
		},
	}
	queues := []worker.ExternalQueue{&dummyExternalQueue{}}

	workChan := make(chan *domain.WorkUnit, 2)

	sut := worker.NewWorker("someflow", llog, objStorages, queues, workChan,
		prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))
	go sut.Run(ctx)
	time.Sleep(time.Millisecond)

	workU := &domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some data"),
	}

	wg.Add(1)
	workChan <- workU
	time.Sleep(1 * time.Millisecond)

	wg.Wait()

	objStorage := objStorages[0].(*mockObjStorage)
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.calledWith, 1, "should have called uploader")
	assert.Same(t, workU, objStorage.calledWith[0], "should have called objUploader with the correct data")
	assert.Equal(t, workU.Data, objStorage.calledWith[0].Data, "should have called objUploader with the correct data")

	close(workChan)
	cancel()
	<-sut.Done()
}

func TestCallsEnqueuerWithUploaderResult(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	uploadResult := &domain.UploadResult{
		Bucket:      "some-bucket",
		Region:      "some-region",
		Path:        "some/path/file.txt",
		URL:         "some-url!",
		SizeInBytes: 1234}
	objStorages := []worker.ObjStorage{
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			wg:         &wg,
			returning:  uploadResult,
		},
	}

	queues := []worker.ExternalQueue{&mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg}}
	workChan := make(chan *domain.WorkUnit, 1)

	sut := worker.NewWorker("someflow", llog, objStorages, queues, workChan,
		prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))
	go sut.Run(ctx)
	time.Sleep(time.Millisecond)

	workU := &domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some data"),
	}

	wg.Add(2)
	workChan <- workU
	time.Sleep(1 * time.Millisecond)

	expected := &domain.MessageContext{
		Bucket:      uploadResult.Bucket,
		Region:      uploadResult.Region,
		Path:        uploadResult.Path,
		URL:         uploadResult.URL,
		SizeInBytes: uploadResult.SizeInBytes,
		SavedAt:     currentTime.Unix(),
	}

	wg.Wait()
	for idx, queue := range queues {
		mockedQueue, ok := queue.(*mockExternalQueue)
		if !ok {
			assert.Fail(t, "cast failed on position %d", idx)
		}

		mockedQueue.mu.Lock()
		defer mockedQueue.mu.Unlock()
		assert.Len(t, mockedQueue.calledWith, 1, "should have called enqueue on position %d", idx)
		assert.Equal(t, expected, mockedQueue.calledWith[0],
			"should have called enqueuer with the correct data on position %d", idx)
	}

	close(workChan)
	cancel()
	<-sut.Done()
}

func TestKeepsGettingWorkAfterWorking(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorages := []worker.ObjStorage{
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			wg:         &wg,
		},
	}
	queues := []worker.ExternalQueue{&dummyExternalQueue{}}
	workChan := make(chan *domain.WorkUnit, 1)

	sut := worker.NewWorker("someflow", llog, objStorages, queues, workChan,
		prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))
	go sut.Run(ctx)
	time.Sleep(time.Millisecond)

	wg.Add(11)
	for i := 0; i < 11; i++ {
		workU := &domain.WorkUnit{
			Filename: "some-filename",
			Prefix:   "some-prefix",
			Data:     []byte(fmt.Sprint(i)),
		}
		workChan <- workU
	}

	wg.Wait()

	objStorage := objStorages[0].(*mockObjStorage)
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.calledWith, 11, "should have called uploader")

	close(workChan)
	cancel()
	<-sut.Done()
}

func TestDrainsWorkChanAfterContextIsCancelled(t *testing.T) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorages := []worker.ObjStorage{
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			wg:         &wg,
		},
	}
	queues := []worker.ExternalQueue{&dummyExternalQueue{}}
	workChan := make(chan *domain.WorkUnit, 10)

	sut := worker.NewWorker("someflow", llog, objStorages, queues, workChan,
		prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))
	go sut.Run(ctx)
	time.Sleep(time.Millisecond)

	workU := &domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some data"),
	}

	workU2 := &domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some data2"),
	}

	workU3 := &domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some data3"),
	}

	wg.Add(3)
	workChan <- workU
	time.Sleep(time.Millisecond)

	cancel()
	time.Sleep(time.Millisecond)

	workChan <- workU2
	workChan <- workU3
	time.Sleep(time.Millisecond)
	wg.Wait()

	objStorage := objStorages[0].(*mockObjStorage)
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.calledWith, 3, "should have called uploader")
	assert.Same(t, workU, objStorage.calledWith[0], "should have called objUploader with the correct data")
	assert.Same(t, workU2, objStorage.calledWith[1], "should have called objUploader with the correct data")
	assert.Same(t, workU3, objStorage.calledWith[2], "should have called objUploader with the correct data")
	close(workChan)
	<-sut.Done()
}

func TestDoesNotCallEnqueueWhenObjUploadFails(t *testing.T) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	objStorages := []worker.ObjStorage{
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			wg:         &wg,
			err:        fmt.Errorf("Some error"),
		},
	}

	queues := []worker.ExternalQueue{&mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg}}
	workChan := make(chan *domain.WorkUnit, 10)

	sut := worker.NewWorker("someflow", llog, objStorages, queues, workChan,
		prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))
	go sut.Run(ctx)
	time.Sleep(time.Millisecond)

	workU := &domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some data"),
	}

	wg.Add(1)
	workChan <- workU
	time.Sleep(time.Millisecond)

	wg.Wait()

	objStorage := objStorages[0].(*mockObjStorage)
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.calledWith, 1, "should have called upload on position")
	for idx, queue := range queues {
		mockedQueue, ok := queue.(*mockExternalQueue)
		if !ok {
			assert.Fail(t, "cast failed on position %d", idx)
		}

		mockedQueue.mu.Lock()
		defer mockedQueue.mu.Unlock()
		assert.Len(t, mockedQueue.calledWith, 0, "should not have called enqueue on position %d", idx)
	}
	close(workChan)
	cancel()
	<-sut.Done()
}

func TestUsesCompressionConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	testCases := []struct {
		compressConf config.CompressionConfig
	}{
		{compressConf: config.CompressionConfig{Type: "gzip"}},
		{compressConf: config.CompressionConfig{Type: "zlib"}},
		{compressConf: config.CompressionConfig{Type: "deflate"}},
		{compressConf: config.CompressionConfig{Type: "snappy"}},
	}

	for _, tc := range testCases {
		var wg sync.WaitGroup
		ctx, cancel := context.WithCancel(context.Background())

		uploadResult := &domain.UploadResult{
			Bucket:      "some-bucket",
			Region:      "some-region",
			Path:        "some/path/file.txt",
			URL:         "some-url!",
			SizeInBytes: 1234,
		}

		objStorages := []worker.ObjStorage{
			&mockObjStorage{
				calledWith: make([]*domain.WorkUnit, 0),
				wg:         &wg,
				returning:  uploadResult,
			},
		}

		queues := []worker.ExternalQueue{&mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg}}
		workChan := make(chan *domain.WorkUnit, 1)

		sut := worker.NewWorker("someflow", llog, objStorages, queues, workChan,
			prometheus.NewRegistry(), tc.compressConf, constantTimeProvider(currentTime))
		go sut.Run(ctx)
		time.Sleep(time.Millisecond)

		data := strings.Repeat("a", 20480) // 20KB

		workU := &domain.WorkUnit{
			Filename: "some-filename",
			Prefix:   "some-prefix",
			Data:     []byte(data),
		}

		wg.Add(2)
		workChan <- workU
		wg.Wait()

		objStorage := objStorages[0].(*mockObjStorage)
		objStorage.mu.Lock()
		defer objStorage.mu.Unlock()
		assert.Len(t, objStorage.calledWith, 1, "worker should have called upload")

		compressedData := objStorage.calledWith[0].Data

		assert.NotEqualf(t, data, string(compressedData), "the compressed data should be different from original %v", tc)
		assert.NotEqualf(t, len(data), len(compressedData), "the compressed data should have different sizes")

		compressorReader, err := compression.NewReader(&tc.compressConf, bytes.NewReader(compressedData))
		assert.NoError(t, err, "compression reader creation should return no error")

		result, err := io.ReadAll(compressorReader)
		assert.NoError(t, err, "compression reader Read should return no error")

		assert.Equal(t, len(data), len(result), "the decompression result should have the same size as the original")
		assert.Equal(t, data, string(result), "the decompression result be the same as the original")

		expectedEnqueue := &domain.MessageContext{
			Bucket:          uploadResult.Bucket,
			Region:          uploadResult.Region,
			Path:            uploadResult.Path,
			URL:             uploadResult.URL,
			SizeInBytes:     uploadResult.SizeInBytes,
			CompressionType: tc.compressConf.Type,
			SavedAt:         currentTime.Unix(),
		}
		for idx, queue := range queues {
			mockedQueue, ok := queue.(*mockExternalQueue)
			if !ok {
				assert.Fail(t, "cast failed on position %d", idx)
			}

			mockedQueue.mu.Lock()
			defer mockedQueue.mu.Unlock()
			assert.Len(t, mockedQueue.calledWith, 1, "worker should have called enqueue on position %d", idx)
			assert.Equal(t, expectedEnqueue, mockedQueue.calledWith[0],
				"worker should have called enqueue with correct message context on position %d", idx)
		}

		close(workChan)
		cancel()
		<-sut.Done()
	}
}

func TestDoesNotCallUploaderIfWorkIsNil(t *testing.T) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	objStorages := []worker.ObjStorage{
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			wg:         &wg,
		},
	}

	queues := []worker.ExternalQueue{&mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg}}
	workChan := make(chan *domain.WorkUnit, 10)

	sut := worker.NewWorker("someflow", llog, objStorages, queues, workChan,
		prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))
	go sut.Run(ctx)
	time.Sleep(time.Millisecond)
	close(workChan)
	time.Sleep(time.Millisecond)

	objStorage := objStorages[0].(*mockObjStorage)
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Empty(t, objStorage.calledWith, "should not have called upload")
	for idx, queue := range queues {
		mockedQueue, ok := queue.(*mockExternalQueue)
		if !ok {
			assert.Fail(t, "cast failed on position %d", idx)
		}

		mockedQueue.mu.Lock()
		defer mockedQueue.mu.Unlock()
		assert.Empty(t, mockedQueue.calledWith, "should not have called enqueuer on position %d", idx)
	}

	cancel()
	<-sut.Done()
}

func TestCanSendMultipleMessagesMultipleTimes(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorages := []worker.ObjStorage{
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			returning: &domain.UploadResult{
				Bucket: "some-new-bucket",
			},
			wg: &wg,
		},
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			returning: &domain.UploadResult{
				Bucket: "some-new-bucket2",
			},
			wg: &wg,
		},
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			returning: &domain.UploadResult{
				Bucket: "some-new-bucket2",
			},
			wg: &wg,
		},
	}

	queues := []worker.ExternalQueue{
		&mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg},
		&mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg},
		&mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg},
	}
	workChan := make(chan *domain.WorkUnit, 1000)

	sut := worker.NewWorker("someflow", llog, objStorages, queues, workChan,
		prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))
	go sut.Run(ctx)
	time.Sleep(time.Millisecond)

	workUnitiesSent := 11
	wg.Add(workUnitiesSent*len(objStorages) + workUnitiesSent*len(queues))
	for i := 0; i < workUnitiesSent; i++ {
		workU := &domain.WorkUnit{
			Filename: "some-filename",
			Prefix:   "some-prefix",
			Data:     []byte(fmt.Sprint(i)),
		}
		workChan <- workU
	}

	wg.Wait()

	for idx, objStorageIface := range objStorages {
		objStorage := objStorageIface.(*mockObjStorage)
		objStorage.mu.Lock()
		defer objStorage.mu.Unlock()
		assert.Len(t, objStorage.calledWith, workUnitiesSent,
			"should have called obj storage on position %d", idx)

		for i := range workUnitiesSent {
			expectedData := []byte(fmt.Sprint(i))
			assert.Equal(t, expectedData, objStorage.calledWith[i].Data,
				"should have called obj storage %d with correct data at call %d", idx, i)
		}

		expected := &domain.MessageContext{
			Bucket:  objStorage.returning.Bucket,
			Region:  "",
			SavedAt: currentTime.Unix(),
		}

		mockedQueue := queues[idx].(*mockExternalQueue)
		mockedQueue.mu.Lock()
		defer mockedQueue.mu.Unlock()
		assert.Len(t, mockedQueue.calledWith, workUnitiesSent,
			"should have called enqueue on position %d with all data", idx)
		assert.Equal(t, expected, mockedQueue.calledWith[0],
			"should have called enqueuer with the correct data on position %d", idx)
	}

	close(workChan)
	cancel()
	<-sut.Done()
}

// FIXME: breaking given new array on obj storages
func TestKeepsSendingMessagesEvenIfAQueueOrObjStorageIsFailing(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorages := []worker.ObjStorage{
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			returning: &domain.UploadResult{
				Bucket: "some-new-bucket",
			},
			wg: &wg,
		},
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			err:        fmt.Errorf("some obj storage error"),
			wg:         &wg,
		},
		&mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			returning: &domain.UploadResult{
				Bucket: "some-new-bucket",
			},
			wg: &wg,
		},
	}
	queues := []worker.ExternalQueue{
		&mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg},
		&mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg},
		&mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg, err: fmt.Errorf("some error")},
	}
	workChan := make(chan *domain.WorkUnit, 100)

	sut := worker.NewWorker("someflow", llog, objStorages, queues, workChan,
		prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))
	go sut.Run(ctx)
	time.Sleep(time.Millisecond)

	workUnitiesSent := 1
	//minus 1 because one of the storages always fails
	wg.Add(workUnitiesSent*len(objStorages) + workUnitiesSent*(len(queues)-1))
	for i := 0; i < workUnitiesSent; i++ {
		workU := &domain.WorkUnit{
			Filename: "some-filename",
			Prefix:   "some-prefix",
			Data:     []byte(fmt.Sprint(i)),
		}
		workChan <- workU
	}

	wg.Wait()

	for idx, objStorageIface := range objStorages {
		objStorage := objStorageIface.(*mockObjStorage)
		objStorage.mu.Lock()
		defer objStorage.mu.Unlock()
		assert.Len(t, objStorage.calledWith, workUnitiesSent,
			"should have called obj storage on position %d", idx)

		mockedQueue := queues[idx].(*mockExternalQueue)
		mockedQueue.mu.Lock()
		defer mockedQueue.mu.Unlock()
		if idx == 1 {
			//The second obj storage always returns error
			assert.Empty(t, mockedQueue.calledWith,
				"should have NOT called enqueue on position %d", idx)
		} else {
			assert.Len(t, mockedQueue.calledWith, workUnitiesSent,
				"should have called enqueue on position %d with all data", idx)
		}
	}

	close(workChan)
	cancel()
	<-sut.Done()
}

//TODO:
// * create test for case where some storages are slow, or queues are slow.
