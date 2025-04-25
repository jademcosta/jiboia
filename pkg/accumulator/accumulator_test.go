package accumulator_test

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"context"

	"github.com/jademcosta/jiboia/pkg/accumulator"
	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
)

const queueCapacity int = 30
const dummyDuration = 1 * time.Minute

var l *slog.Logger = logger.NewDummy()
var dummyCB circuitbreaker.CircuitBreaker = circuitbreaker.NewDummyCircuitBreaker()
var implementations = []struct {
	name    string
	newFunc func(
		flowName string,
		logg *slog.Logger,
		limitOfBytes int,
		separator []byte,
		queueCapacity int,
		next domain.DataFlow,
		cb circuitbreaker.CircuitBreaker,
		metricRegistry *prometheus.Registry,
		currentTimeProvider func() time.Time,
		forceFlushAfter time.Duration,
	) flow.DataFlowRunnable
}{
	{name: "size accumulator", newFunc: accumulator.NewAccumulatorBySize},
	{name: "time-n-size accumulator", newFunc: accumulator.NewAccumulatorByTimeAndSize},
}

type dataEnqueuerMock struct {
	callCount   int
	dataWritten [][]byte
	mu          sync.Mutex
}

func (w *dataEnqueuerMock) Enqueue(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.callCount++
	w.dataWritten = append(w.dataWritten, data)
	return nil
}

type failingDataEnqueuerMock struct {
	callCount   int
	dataWritten [][]byte
	mu          sync.Mutex
	fail        bool
}

func (denq *failingDataEnqueuerMock) Enqueue(data []byte) error {
	denq.mu.Lock()
	defer denq.mu.Unlock()

	denq.callCount++
	denq.dataWritten = append(denq.dataWritten, data)
	if denq.fail {
		return errors.New("i always fail")
	}
	return nil

}

func (denq *failingDataEnqueuerMock) SetFail(v bool) {
	denq.mu.Lock()
	defer denq.mu.Unlock()
	denq.fail = v
}

func TestPanicsIfLimitIsOneOrLess(t *testing.T) {
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {

			next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}
			separator := []byte("")

			assert.Panics(t, func() {
				impl.newFunc("someflow", l, 0, separator, 30, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)
			}, "limit of bytes 0 is not allowed")
			assert.Panics(t, func() {
				impl.newFunc("someflow", l, 1, separator, 30, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)
			}, "limit of bytes 1 is not allowed")

			assert.NotPanics(t, func() {
				impl.newFunc("someflow", l, 2, separator, 30, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)
			}, "limit of bytes 2 is allowed")
		})
	}
}

func TestPanicsIfSeparatorLenEqualOrBiggerThanLimit(t *testing.T) {

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}

			limitOfBytes := 10
			separator := []byte("abcdefghij")

			assert.Panics(t, func() {
				impl.newFunc("someflow", l, limitOfBytes, separator, 30, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)
			}, "separator size == limit is not allowed")

			limitOfBytes = 10
			separator = []byte("abcdefghijk")

			assert.Panics(t, func() {
				impl.newFunc("someflow", l, limitOfBytes, separator, 30, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)
			}, "separator size > limit is not allowed")
		})
	}
}

func TestPanicsIfQueueSizeIsTwoOrLess(t *testing.T) {
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}
			separator := []byte("")
			bytesSizeLimit := 11

			assert.Panics(t, func() {
				impl.newFunc("someflow", l, bytesSizeLimit, separator, 0, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)
			}, "queue size of 0 is not allowed")
			assert.Panics(t, func() {
				impl.newFunc("someflow", l, bytesSizeLimit, separator, 1, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)
			}, "queue size of 1 is not allowed")

			assert.NotPanics(t, func() {
				impl.newFunc("someflow", l, bytesSizeLimit, separator, 2, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)
			}, "queue size of 2 should be allowed")
		})
	}

}

func TestItDoesNotPassDataIfLimitNotReached(t *testing.T) {

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
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

				sut := impl.newFunc("someflow", l, tc.limitBytes, tc.separator, queueCapacity, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)

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
		})
	}
}

func TestWritesTheDataIfSizeEqualOrBiggerThanCapacity(t *testing.T) {
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
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

				sut := impl.newFunc("someflow", l, tc.limitBytes, tc.separator, queueCapacity, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)

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
		})
	}
}

func TestWritesTheDataWhenLimitIsHitAfterMultipleCalls(t *testing.T) {
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
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

				sut := impl.newFunc("someflow", l, tc.limitBytes, tc.separator, queueCapacity, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)

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
		})
	}
}

func TestRejectsDataIfAtFullCapacity(t *testing.T) {
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
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

				sut := impl.newFunc("someflow", l, limitBytes, tc.separator, tc.queueCapacity, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)

				for i := 0; i < tc.dataEnqueueCount; i++ {
					_ = sut.Enqueue([]byte(fmt.Sprint(i)))
				}

				err := sut.Enqueue([]byte("1"))
				assert.Error(t, err, "should return error")

				next.mu.Lock()
				assert.Equalf(
					t, next.callCount, 0,
					"should not produce any message. Queue cap: %d dataCount: %d.",
					tc.queueCapacity, tc.dataEnqueueCount,
				)
				next.mu.Unlock()

			}
		})
	}
}

func TestTheCapacityIsFixed(t *testing.T) {

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			limitBytes := 6000

			next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}

			separator := []byte("")
			queueCapacity := 2
			dataEnqueueCount := accumulator.MinQueueCapacity

			sut := impl.newFunc("someflow", l, limitBytes, separator, queueCapacity, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)

			for i := 0; i < dataEnqueueCount; i++ {
				err := sut.Enqueue([]byte(fmt.Sprint(i)))
				assert.NoError(t, err, "should not err on enqueue")
			}

			next.mu.Lock()
			assert.Equalf(t, next.callCount, 0,
				"should not produce any message. Queue cap: %d dataCount: %d.",
				queueCapacity, dataEnqueueCount)
			next.mu.Unlock()

			err := sut.Enqueue([]byte("a"))
			assert.Error(t, err, "should err on enqueue")
			err = sut.Enqueue([]byte("b"))
			assert.Error(t, err, "should err on enqueue")
		})
	}
}

func TestSendsPendingDataWhenContextIsCancelled(t *testing.T) {
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
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

				ctx, cancel := context.WithCancel(context.Background())

				sut := impl.newFunc("someflow", l, limitBytes, separator, queueCapacity, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)

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

				cancel()
				time.Sleep(10 * time.Millisecond)

				next.mu.Lock()
				assert.Equalf(t, next.callCount, len(tc.want),
					"(after shutdown) should have sent all data. Data: %v.",
					tc.data)
				next.mu.Unlock()
			}
		})
	}
}

func TestEnqueuesErrorsAfterContextCancelled(t *testing.T) {
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			limitBytes := 10
			separator := []byte("")
			queueCapacity := 30

			next := &dataEnqueuerMock{dataWritten: make([][]byte, 0)}

			ctx, cancel := context.WithCancel(context.Background())

			sut := impl.newFunc("someflow", l, limitBytes, separator, queueCapacity, next, dummyCB, prometheus.NewRegistry(), time.Now, dummyDuration)

			go sut.Run(ctx)
			time.Sleep(1 * time.Millisecond)

			cancel()
			time.Sleep(10 * time.Millisecond)

			next.mu.Lock()
			assert.Equalf(t, next.callCount, 0,
				"should not produce any message.")
			next.mu.Unlock()

			err := sut.Enqueue([]byte("hi"))
			assert.Error(t, err, "should return error if enqueue is called after a shutdown has started")
		})
	}
}

func TestCallindEnqueueUsesACircuitBreakerAndRetriesOnFailure(t *testing.T) {
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			openInterval := 100 * time.Millisecond
			registry := prometheus.NewRegistry()

			next := &failingDataEnqueuerMock{
				dataWritten: make([][]byte, 0),
				fail:        true,
			}
			cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
				Name:        "accumulator-test",
				MaxRequests: 1, //FIXME: magic number. This should be extracted into a const
				Timeout:     openInterval,
				ReadyToTrip: func(_ gobreaker.Counts) bool {
					return true
				},
			})

			limitOfBytes := 3
			separator := []byte("")

			sut := impl.newFunc("someflow", l, limitOfBytes, separator, queueCapacity, next, cb, registry, time.Now, dummyDuration)

			ctx, cancel := context.WithCancel(context.Background())
			go sut.Run(ctx)

			payload := []byte("333")
			err := sut.Enqueue(payload)
			assert.NoError(t, err, "should not err on enqueue")
			time.Sleep(5 * time.Millisecond)

			next.mu.Lock()
			assert.Equal(t, 1, next.callCount, "should produce only 1 data message, as the CB was open")
			next.mu.Unlock()

			time.Sleep(openInterval)
			time.Sleep(accumulator.CBRetrySleepDuration)
			time.Sleep(5 * time.Microsecond)

			wanted := [][]byte{payload, payload}
			next.mu.Lock()
			assert.Equal(t, 2, next.callCount, "should produce only 2 data message, as the CB has closed, open, and closed again")
			assert.Equal(t, wanted, next.dataWritten, "should call 'Write' with exactly the same data passed into it")
			next.mu.Unlock()

			time.Sleep(openInterval)
			time.Sleep(accumulator.CBRetrySleepDuration)
			time.Sleep(5 * time.Microsecond)

			wanted = [][]byte{payload, payload, payload}
			next.mu.Lock()
			assert.Equal(t, 3, next.callCount, "should produce only 3 data message, as the CB has closed, open, and closed again")
			assert.Equal(t, wanted, next.dataWritten, "should call 'Write' with exactly the same data passed into it")
			next.mu.Unlock()

			cancel()
		})
	}
}

func TestItStopsRetryingOnceItSendsTheData(t *testing.T) {
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			openInterval := 100 * time.Millisecond
			registry := prometheus.NewRegistry()

			next := &failingDataEnqueuerMock{
				dataWritten: make([][]byte, 0),
				fail:        true,
			}
			cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
				Name:        "accumulator-test",
				MaxRequests: 1, //FIXME: magic number. This should be extracted into a const
				Timeout:     openInterval,
				ReadyToTrip: func(_ gobreaker.Counts) bool {
					return true
				},
			})

			limitOfBytes := 3
			separator := []byte("")

			sut := impl.newFunc("someflow", l, limitOfBytes, separator, queueCapacity, next, cb, registry, time.Now, dummyDuration)

			ctx, cancel := context.WithCancel(context.Background())
			go sut.Run(ctx)

			payload := []byte("333")
			err := sut.Enqueue(payload)
			assert.NoError(t, err, "should not err on enqueue")
			time.Sleep(1 * time.Millisecond)

			next.mu.Lock()
			assert.Equal(t, 1, next.callCount, "should produce only 1 data message, as the CB was open")
			next.mu.Unlock()

			next.SetFail(false)
			time.Sleep(openInterval)
			time.Sleep(accumulator.CBRetrySleepDuration)
			time.Sleep(1 * time.Millisecond)

			wanted := [][]byte{payload, payload}
			next.mu.Lock()
			assert.Equal(t, 2, next.callCount, "should produce only 2 data message, as the CB has closed, open, and closed again")
			assert.Equal(t, wanted, next.dataWritten, "should call 'Write' with exactly the same data passed into it")
			next.mu.Unlock()

			next.SetFail(true)
			payload2 := []byte("4444")
			err = sut.Enqueue(payload2)
			assert.NoError(t, err, "should not err on enqueue")
			time.Sleep(1 * time.Millisecond)

			next.SetFail(false)

			time.Sleep(openInterval * 5) //Waiting longer to show we don't have any more message
			time.Sleep(accumulator.CBRetrySleepDuration)
			time.Sleep(1 * time.Millisecond)

			wanted = [][]byte{payload, payload, payload2, payload2}
			next.mu.Lock()
			assert.Equal(t, 4, next.callCount, "should produce only 2 data message, as the CB has closed, open, and closed again")
			assert.Equal(t, wanted, next.dataWritten, "should call 'Write' with exactly the same data passed into it")
			next.mu.Unlock()

			cancel()
		})
	}
}
