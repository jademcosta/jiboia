package non_blocking_bucket_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"context"

	bucket "github.com/jademcosta/jiboia/pkg/accumulators/non_blocking_bucket"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const queueCapacity int = 30

var l *zap.SugaredLogger

func init() {
	l = logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})
}

type dataEnqueuerMock struct {
	callCount   int
	dataWritten [][]byte
	mu          sync.Mutex
}

func (w *dataEnqueuerMock) Enqueue(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.callCount += 1
	w.dataWritten = append(w.dataWritten, data)
	return nil
}

type dummyDataDropper struct{}

func (dd *dummyDataDropper) Drop([]byte) {
	// Do nothing
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

func TestPanicsIfLimitIsOneOrLess(t *testing.T) {

	next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}
	separator := []byte("")

	assert.Panics(t, func() {
		bucket.New("someflow", l, 0, separator, 30, &dummyDataDropper{}, next, prometheus.NewRegistry())
	}, "limit of bytes 0 is not allowed")
	assert.Panics(t, func() {
		bucket.New("someflow", l, 1, separator, 30, &dummyDataDropper{}, next, prometheus.NewRegistry())
	}, "limit of bytes 1 is not allowed")
}

func TestPanicsIfSeparatorLenEqualOrBiggerThanLimit(t *testing.T) {

	next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}

	limitOfBytes := 10
	separator := []byte("abcdefghij")

	assert.Panics(t, func() {
		bucket.New("someflow", l, limitOfBytes, separator, 30, &dummyDataDropper{}, next, prometheus.NewRegistry())
	}, "separator size == limit is not allowed")

	limitOfBytes = 10
	separator = []byte("abcdefghijk")

	assert.Panics(t, func() {
		bucket.New("someflow", l, limitOfBytes, separator, 30, &dummyDataDropper{}, next, prometheus.NewRegistry())
	}, "separator size > limit is not allowed")
}

func TestItDoesNotPassDataIfLimitNotReached(t *testing.T) {

	type testCase struct {
		limitBytes int
		separator  []byte
		data       []byte
	}

	testCases := []testCase{
		{limitBytes: 10, separator: []byte("b"), data: []byte("1")},
		{limitBytes: 10, separator: []byte("__sep__"), data: []byte("22")},
		{limitBytes: 10, separator: []byte(""), data: []byte("")},
		{limitBytes: 10, separator: []byte(""), data: []byte("999999999")},
		{limitBytes: 2, separator: []byte("a"), data: []byte("")},
		{limitBytes: 1073741824, separator: []byte("aaaaaaasadasda"), data: []byte("12345678910111213")}, //1Gb of limit
	}

	for _, tc := range testCases {
		next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}
		ctx, cancel := context.WithCancel(context.Background())

		sut := bucket.New("someflow", l, tc.limitBytes, tc.separator, queueCapacity, &dummyDataDropper{}, next, prometheus.NewRegistry())
		go sut.Run(ctx)
		err := sut.Enqueue(tc.data)
		assert.NoError(t, err, "should not err on enqueue")
		time.Sleep(5 * time.Millisecond)

		next.mu.Lock()
		assert.Lenf(t, next.dataWritten, 0,
			"should not write to next writter if limit is not reached. Separator: %s, data: %s.",
			tc.separator, tc.data)
		assert.Equalf(t, 0, next.callCount,
			"should not call 'Write' on next writter if limit is not reached. Separator: %s, data: %s.",
			tc.separator, tc.data)
		next.mu.Unlock()

		cancel()
	}
}

func TestWritesTheDataIfSizeEqualOrBiggerThanCapacity(t *testing.T) {
	type testCase struct {
		limitBytes int
		separator  []byte
		data       []byte
		want       [][]byte
	}

	testCases := []testCase{
		{limitBytes: 2,
			separator: []byte(""),
			data:      []byte("aa"),
			want:      [][]byte{[]byte("aa")}},

		{limitBytes: 2,
			separator: []byte(""),
			data:      []byte("aaa"),
			want:      [][]byte{[]byte("aaa")}},

		{limitBytes: 10,
			separator: []byte(""),
			data:      []byte("aaaaaaaaaa"),
			want:      [][]byte{[]byte("aaaaaaaaaa")}},

		{limitBytes: 10,
			separator: []byte(""),
			data:      []byte("asdfasdfasdfasdfasdf"),
			want:      [][]byte{[]byte("asdfasdfasdfasdfasdf")}},

		{limitBytes: 3,
			separator: []byte("ii"),
			data:      []byte("asdfasdfasdfasdfasdf"),
			want:      [][]byte{[]byte("asdfasdfasdfasdfasdf")}},

		{limitBytes: 2,
			separator: []byte("i"),
			data:      []byte("as"),
			want:      [][]byte{[]byte("as")}},
	}

	for _, tc := range testCases {
		next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}
		ctx, cancel := context.WithCancel(context.Background())

		sut := bucket.New("someflow", l, tc.limitBytes, tc.separator, queueCapacity, &dummyDataDropper{}, next, prometheus.NewRegistry())
		go sut.Run(ctx)
		err := sut.Enqueue(tc.data)
		assert.NoError(t, err, "should not err on enqueue")
		time.Sleep(5 * time.Millisecond)

		next.mu.Lock()
		assert.Equal(t, 1, next.callCount, "should produce only 1 data message")
		assert.Equal(t, tc.want, next.dataWritten, "should call 'Write' with exactly the same data passed into it")
		next.mu.Unlock()

		cancel()
	}
}

func TestWritesTheDataWhenLimitIsHitAfterMultipleCalls(t *testing.T) {

	type testCase struct {
		limitBytes int
		separator  []byte
		data       [][]byte
		want       [][]byte
	}

	testCases := []testCase{
		{limitBytes: 6, separator: []byte("_"),
			data: [][]byte{[]byte("55555"), []byte("1")},
			want: [][]byte{[]byte("55555")}},

		{limitBytes: 6, separator: []byte(""),
			data: [][]byte{[]byte("4444"), []byte("22")},
			want: [][]byte{[]byte("444422")}},

		{limitBytes: 6, separator: []byte("__v__"),
			data: [][]byte{[]byte("4444"), []byte("55555")},
			want: [][]byte{[]byte("4444")}},

		{limitBytes: 6, separator: []byte("__v__"),
			data: [][]byte{[]byte("4444"), []byte("55555"), []byte("1")},
			want: [][]byte{[]byte("4444"), []byte("55555")}},

		{limitBytes: 6, separator: []byte("__v__"),
			data: [][]byte{[]byte("4444"), []byte("666666"), []byte("1")},
			want: [][]byte{[]byte("4444"), []byte("666666")}},

		{limitBytes: 6, separator: []byte("__v__"),
			data: [][]byte{[]byte("4444"), []byte("7777777"), []byte("1")},
			want: [][]byte{[]byte("4444"), []byte("7777777")}},

		{limitBytes: 6, separator: []byte("_"),
			data: [][]byte{[]byte("1"), []byte("4444")},
			want: [][]byte{[]byte("1_4444")}},

		{limitBytes: 6, separator: []byte("_"),
			data: [][]byte{[]byte("1"), []byte("22"), []byte("333")},
			want: [][]byte{[]byte("1_22")}},

		{limitBytes: 16, separator: []byte("__"),
			data: [][]byte{[]byte("1"), []byte("22"), []byte("a"), []byte("b"), []byte("c"), []byte("88888888")},
			want: [][]byte{[]byte("1__22__a__b__c")}},

		{limitBytes: 16, separator: []byte("__"),
			data: [][]byte{
				[]byte("1"), []byte("22"), []byte("a"), []byte("b"), []byte("c"), []byte("88888888"),
				[]byte("22"), []byte("asd"),
			},
			want: [][]byte{[]byte("1__22__a__b__c"), []byte("88888888__22")}},

		{limitBytes: 16, separator: []byte("__v__v__"),
			data: [][]byte{[]byte("88888888"), []byte("22")},
			want: [][]byte{[]byte("88888888")}}, // TODO: add more like these

		{limitBytes: 16, separator: []byte("__v__v__"),
			data: [][]byte{[]byte("666666"), []byte("22")},
			want: [][]byte{[]byte("666666__v__v__22")}},

		{limitBytes: 16, separator: []byte("__v__v__"),
			data: [][]byte{[]byte("666666"), []byte("1"), []byte("22")},
			want: [][]byte{[]byte("666666__v__v__1")}},

		{limitBytes: 6, separator: []byte("_"),
			data: [][]byte{[]byte("1"), []byte("55555")},
			want: [][]byte{[]byte("1")}},

		{limitBytes: 6, separator: []byte("_"),
			data: [][]byte{[]byte("1"), []byte("666666")},
			want: [][]byte{[]byte("1"), []byte("666666")}},

		{limitBytes: 6, separator: []byte("_"),
			data: [][]byte{[]byte("1"), []byte("7777777")},
			want: [][]byte{[]byte("1"), []byte("7777777")}},
	}

	for _, tc := range testCases {
		next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}
		ctx, cancel := context.WithCancel(context.Background())

		sut := bucket.New("someflow", l, tc.limitBytes, tc.separator, queueCapacity, &dummyDataDropper{}, next, prometheus.NewRegistry())
		go sut.Run(ctx)
		for _, data := range tc.data {
			err := sut.Enqueue(data)
			assert.NoError(t, err, "should not err on enqueue")
			time.Sleep(1 * time.Millisecond)
		}
		time.Sleep(5 * time.Millisecond)

		next.mu.Lock()
		assert.Equalf(t, next.callCount, len(tc.want),
			"should produce the correct amount of data messages. Separator: %s Data: %v.",
			tc.separator, tc.data)
		assert.Equalf(t, tc.want, next.dataWritten,
			"should call 'Write' following the algorithm of not appending data to the buffer if it will result in a bigger buffer. Separator: %s Data: %v.",
			tc.separator, tc.data)
		next.mu.Unlock()

		cancel()
	}
}

func TestDropsDataIfAtFullCapacity(t *testing.T) {

	type testCase struct {
		separator        []byte
		dataEnqueueCount int
		queueCapacity    int
	}

	limitBytes := 6000

	testCases := []testCase{
		{separator: []byte("__v__"), queueCapacity: 30, dataEnqueueCount: 31},
		{separator: []byte("__v__"), queueCapacity: 30, dataEnqueueCount: 92},
		{separator: []byte(""), queueCapacity: 30, dataEnqueueCount: 92},
		{separator: []byte(""), queueCapacity: 90, dataEnqueueCount: 91},
	}

	for _, tc := range testCases {
		next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}
		ddropper := &mockDataDropper{dataDropped: make([][]byte, 0)}

		sut := bucket.New("someflow", l, limitBytes, tc.separator, tc.queueCapacity, ddropper, next, prometheus.NewRegistry())

		for i := 0; i < tc.dataEnqueueCount; i++ {
			_ = sut.Enqueue([]byte(fmt.Sprint(i)))

		}

		next.mu.Lock()
		assert.Equalf(t, next.callCount, 0,
			"should not produce any message. Queue cap: %d dataCount: %d.",
			tc.queueCapacity, tc.dataEnqueueCount)
		next.mu.Unlock()

		ddropper.mu.Lock()
		assert.Lenf(t, ddropper.dataDropped, (tc.dataEnqueueCount - tc.queueCapacity),
			"should drop messages if queue capacity is reached. Queue cap: %d dataCount: %d.",
			tc.queueCapacity, tc.dataEnqueueCount)
		ddropper.mu.Unlock()
	}
}

func TestTheMinimunCapacityIsFixed(t *testing.T) {

	limitBytes := 6000

	next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}
	ddropper := &mockDataDropper{dataDropped: make([][]byte, 0)}
	separator := []byte("")
	queueCapacity := 3
	dataEnqueueCount := bucket.MINIMAL_QUEUE_CAPACITY

	sut := bucket.New("someflow", l, limitBytes, separator, queueCapacity, ddropper, next, prometheus.NewRegistry())

	for i := 0; i < dataEnqueueCount; i++ {
		err := sut.Enqueue([]byte(fmt.Sprint(i)))
		assert.NoError(t, err, "should not err on enqueue")
	}

	next.mu.Lock()
	assert.Equalf(t, next.callCount, 0,
		"should not produce any message. Queue cap: %d dataCount: %d.",
		queueCapacity, dataEnqueueCount)
	next.mu.Unlock()

	ddropper.mu.Lock()
	assert.Lenf(t, ddropper.dataDropped, 0,
		"should not drop messages until queue capacity is exceeded.")
	ddropper.mu.Unlock()

	err := sut.Enqueue([]byte("a"))
	assert.Error(t, err, "should err on enqueue")
	err = sut.Enqueue([]byte("b"))
	assert.Error(t, err, "should err on enqueue")

	ddropper.mu.Lock()
	assert.Lenf(t, ddropper.dataDropped, 2,
		"should drop messages once the queue capacity is exceeded.")
	ddropper.mu.Unlock()
}

func TestSendsPendingDataWhenContextIsCancelled(t *testing.T) {
	type testCase struct {
		data [][]byte
		want [][]byte
	}

	limitBytes := 10
	separator := []byte("")
	queueCapacity := 30

	testCases := []testCase{
		{
			data: [][]byte{[]byte("1"), []byte("22"), []byte("333")},
			want: [][]byte{[]byte("122333")},
		},
		{
			data: [][]byte{[]byte("55555"), []byte("1")},
			want: [][]byte{[]byte("555551")},
		},
		{
			data: [][]byte{[]byte("55555")},
			want: [][]byte{[]byte("55555")},
		},
		{
			data: [][]byte{[]byte("999999999")},
			want: [][]byte{[]byte("999999999")},
		},
	}

	for _, tc := range testCases {
		next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}
		ddropper := &mockDataDropper{dataDropped: make([][]byte, 0)}
		ctx, cancel := context.WithCancel(context.Background())

		sut := bucket.New("someflow", l, limitBytes, separator, queueCapacity, ddropper, next, prometheus.NewRegistry())
		go sut.Run(ctx)

		for _, data := range tc.data {
			err := sut.Enqueue(data)
			assert.NoError(t, err, "should not err on enqueue")
		}

		next.mu.Lock()
		assert.Equalf(t, next.callCount, 0,
			"(before shutdown) should not produce any message. Data: %v.",
			tc.data)
		next.mu.Unlock()

		ddropper.mu.Lock()
		assert.Lenf(t, ddropper.dataDropped, 0,
			"(before shutdown) should have not dropped any data. Data: %v.",
			tc.data)
		ddropper.mu.Unlock()

		cancel()
		time.Sleep(10 * time.Millisecond)

		ddropper.mu.Lock()
		assert.Lenf(t, ddropper.dataDropped, 0,
			"(after shutdown) should have not dropped any data. Data: %v.",
			tc.data)
		ddropper.mu.Unlock()

		next.mu.Lock()
		assert.Equalf(t, next.callCount, len(tc.want),
			"(after shutdown) should have sent all data. Data: %v.",
			tc.data)
		next.mu.Unlock()
	}
}

func TestEnqueuesErrorsAfterContextCancelled(t *testing.T) {

	limitBytes := 10
	separator := []byte("")
	queueCapacity := 30

	next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}
	ddropper := &mockDataDropper{dataDropped: make([][]byte, 0)}
	ctx, cancel := context.WithCancel(context.Background())

	sut := bucket.New("someflow", l, limitBytes, separator, queueCapacity, ddropper, next, prometheus.NewRegistry())
	go sut.Run(ctx)
	time.Sleep(1 * time.Millisecond)

	cancel()
	time.Sleep(10 * time.Millisecond)

	next.mu.Lock()
	assert.Equalf(t, next.callCount, 0,
		"should not produce any message.")
	next.mu.Unlock()

	ddropper.mu.Lock()
	assert.Lenf(t, ddropper.dataDropped, 0,
		"should have not dropped any data.")
	ddropper.mu.Unlock()

	err := sut.Enqueue([]byte("hi"))
	assert.Error(t, err, "should return error if enqueue is called after a shutdown has started")
}
