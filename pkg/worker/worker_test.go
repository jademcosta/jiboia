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

type mockExternalQueue struct {
	calledWith []*domain.MessageContext
	mu         sync.Mutex
	wg         *sync.WaitGroup
}

func (queue *mockExternalQueue) Enqueue(data *domain.MessageContext) error {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	defer queue.wg.Done()

	queue.calledWith = append(queue.calledWith, data)
	return nil
}

type dummyExternalQueue struct{}

func (queue *dummyExternalQueue) Enqueue(_ *domain.MessageContext) error {
	return nil
}

func TestCallsObjUploaderWithDataPassed(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorage := &mockObjStorage{
		calledWith: make([]*domain.WorkUnit, 0),
		wg:         &wg,
	}
	queue := &dummyExternalQueue{}

	workChan := make(chan *domain.WorkUnit, 1)

	sut := worker.NewWorker("someflow", llog, objStorage, queue, workChan,
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
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.calledWith, 1, "should have called uploader")
	assert.Same(t, workU, objStorage.calledWith[0], "should have called objUploader with the correct data")
	assert.Equal(t, workU.Data, objStorage.calledWith[0].Data, "should have called objUploader with the correct data")

	cancel()
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
	objStorage := &mockObjStorage{
		calledWith: make([]*domain.WorkUnit, 0),
		wg:         &wg,
		returning:  uploadResult,
	}

	queue := &mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg}
	workChan := make(chan *domain.WorkUnit, 1)

	sut := worker.NewWorker("someflow", llog, objStorage, queue, workChan,
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
	queue.mu.Lock()
	defer queue.mu.Unlock()
	assert.Len(t, queue.calledWith, 1, "should have called enqueuer")
	assert.Equal(t, expected, queue.calledWith[0], "should have called enqueuer with the correct data")

	close(workChan)
	cancel()
}

func TestKeepsGettingWorkAfterWorking(t *testing.T) {

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorage := &mockObjStorage{
		calledWith: make([]*domain.WorkUnit, 0),
		wg:         &wg,
	}
	queue := &dummyExternalQueue{}
	workChan := make(chan *domain.WorkUnit, 1)

	sut := worker.NewWorker("someflow", llog, objStorage, queue, workChan,
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
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.calledWith, 11, "should have called uploader")

	close(workChan)
	cancel()
}

func TestDrainsWorkChanAfterContextIsCancelled(t *testing.T) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	objStorage := &mockObjStorage{
		calledWith: make([]*domain.WorkUnit, 0),
		wg:         &wg,
	}
	queue := &dummyExternalQueue{}
	workChan := make(chan *domain.WorkUnit, 10)

	sut := worker.NewWorker("someflow", llog, objStorage, queue, workChan,
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

	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.calledWith, 3, "should have called uploader")
	assert.Same(t, workU, objStorage.calledWith[0], "should have called objUploader with the correct data")
	assert.Same(t, workU2, objStorage.calledWith[1], "should have called objUploader with the correct data")
	assert.Same(t, workU3, objStorage.calledWith[2], "should have called objUploader with the correct data")
	close(workChan)
}

func TestDoesNotCallEnqueueWhenObjUploadFails(t *testing.T) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	objStorage := &mockObjStorage{
		calledWith: make([]*domain.WorkUnit, 0),
		wg:         &wg,
		err:        fmt.Errorf("Some error"),
	}

	queue := &mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg}
	workChan := make(chan *domain.WorkUnit, 10)

	sut := worker.NewWorker("someflow", llog, objStorage, queue, workChan,
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
	queue.mu.Lock()
	defer queue.mu.Unlock()
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Len(t, objStorage.calledWith, 1, "should have called upload")
	assert.Len(t, queue.calledWith, 0, "should not have called enqueuer")

	close(workChan)
	cancel()
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

		objStorage := &mockObjStorage{
			calledWith: make([]*domain.WorkUnit, 0),
			wg:         &wg,
			returning:  uploadResult,
		}

		queue := &mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg}
		workChan := make(chan *domain.WorkUnit, 1)

		sut := worker.NewWorker("someflow", llog, objStorage, queue, workChan,
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

		queue.mu.Lock()
		defer queue.mu.Unlock()
		objStorage.mu.Lock()
		defer objStorage.mu.Unlock()
		assert.Len(t, objStorage.calledWith, 1, "worker should have called upload")
		assert.Len(t, queue.calledWith, 1, "worker should have called enqueue")

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
		assert.Equal(t, expectedEnqueue, queue.calledWith[0], "worker should have called enqueue with correct message context")
		close(workChan)
		cancel()
	}
}

func TestDoesNotCallUploaderIfWorkIsNil(t *testing.T) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	objStorage := &mockObjStorage{
		calledWith: make([]*domain.WorkUnit, 0),
		wg:         &wg,
	}

	queue := &mockExternalQueue{calledWith: make([]*domain.MessageContext, 0), wg: &wg}
	workChan := make(chan *domain.WorkUnit, 10)

	sut := worker.NewWorker("someflow", llog, objStorage, queue, workChan,
		prometheus.NewRegistry(), noCompressionConf, constantTimeProvider(currentTime))
	go sut.Run(ctx)
	time.Sleep(time.Millisecond)
	close(workChan)
	time.Sleep(time.Millisecond)

	queue.mu.Lock()
	defer queue.mu.Unlock()
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	assert.Empty(t, objStorage.calledWith, "should not have called upload")
	assert.Empty(t, queue.calledWith, "should not have called enqueuer")

	cancel()
}
