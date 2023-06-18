package uploader_test

// TODO: merge this whole file with the other test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/jademcosta/jiboia/pkg/uploader"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

type dummyFilePather struct{}

func (dummy *dummyFilePather) Filename() *string {
	filename := "a"
	return &filename
}

func (dummy *dummyFilePather) Prefix() *string {
	prefix := "b"
	return &prefix
}

type mockDataDropper struct {
	dataDropped [][]byte
	mu          sync.Mutex
}

func (d *mockDataDropper) Drop(data []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.dataDropped = append(d.dataDropped, data)
}

func TestDropsDataIfAtFullCapacity(t *testing.T) {

	type testConfig struct {
		capacity              int
		objectsToEnqueueCount int
		want                  int // (objectsToEnqueueCount - capacity)
	}

	testCases := []testConfig{
		{capacity: 10, objectsToEnqueueCount: 20, want: 10},
		{capacity: 10, objectsToEnqueueCount: 200, want: 190},
		{capacity: 1, objectsToEnqueueCount: 2, want: 1},
		{capacity: 1, objectsToEnqueueCount: 1, want: 0},
		{capacity: 1, objectsToEnqueueCount: 100, want: 99},
		{capacity: 100, objectsToEnqueueCount: 2000, want: 1900},
		{capacity: 1, objectsToEnqueueCount: 4, want: 3},
	}

	l := logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})
	workersCount := 0

	for _, tc := range testCases {
		d := &mockDataDropper{dataDropped: make([][]byte, 0)}
		ctx := context.Background()

		uploader := uploader.New("someflow", l, workersCount, tc.capacity, d, &dummyFilePather{}, prometheus.NewRegistry())

		go uploader.Run(ctx)

		for i := 0; i < tc.objectsToEnqueueCount; i++ {
			_ = uploader.Enqueue([]byte(fmt.Sprint(i)))
		}

		assert.Lenf(t, d.dataDropped, tc.want, "should have dropped %d items", tc.want)
	}
}

func TestDoesNotDropDataIfNotAtFullCapacity(t *testing.T) {

	type testConfig struct {
		capacity              int
		objectsToEnqueueCount int
	}

	testCases := []testConfig{
		{capacity: 100, objectsToEnqueueCount: 99},
		{capacity: 76, objectsToEnqueueCount: 76},
		{capacity: 1, objectsToEnqueueCount: 1},
		{capacity: 1, objectsToEnqueueCount: 0},
		{capacity: 0, objectsToEnqueueCount: 0},
		{capacity: 2000, objectsToEnqueueCount: 1},
		{capacity: 4, objectsToEnqueueCount: 1},
	}

	l := logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})
	workersCount := 0

	for _, tc := range testCases {
		d := &mockDataDropper{dataDropped: make([][]byte, 0)}
		ctx := context.Background()

		uploader := uploader.New("someflow", l, workersCount, tc.capacity, d, &dummyFilePather{}, prometheus.NewRegistry())

		go uploader.Run(ctx)

		for i := 0; i < tc.objectsToEnqueueCount; i++ {
			err := uploader.Enqueue([]byte(fmt.Sprint(i)))
			assert.NoError(t, err, "should not err on enqueue")
		}

		assert.Lenf(t, d.dataDropped, 0, "should have no dropped items")
	}
}

// func FuzzDropsDataIfAtFullCapacity(f *testing.F) {
// 	f.Add(uint(10), uint(20)) // Use f.Add to provide a seed corpus
// 	f.Fuzz(func(t *testing.T, capacity uint, objectsToEnqueueCount uint) {
// 		l := logger.New(&config.Config{LogLevel: "warn", LogFormat: "json"})
// 		d := &mockDataDropper{dataDropped: make([][]byte, 0)}
// 		ctx := context.Background()

// 		workersCount := 0

// 		uploader := uploader.NewNonBlockingUploader(l, workersCount, int(capacity), d)

// 		go uploader.Run(ctx)

// 		for i := 0; i < int(objectsToEnqueueCount); i++ {
// 			uploader.Enqueue([]byte(fmt.Sprint(i)))
// 		}

// 		if len(d.dataDropped) != int(objectsToEnqueueCount-capacity) {
// 			t.Errorf("should have dropped %d items", (objectsToEnqueueCount - capacity))
// 		}
// 		// assert.Lenf(t, d.dataDropped, (objectsToEnqueueCount - capacity), "should have dropped %d items", (objectsToEnqueueCount - capacity))
// 	})
// }
