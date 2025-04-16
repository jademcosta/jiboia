package accumulator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	MinQueueCapacity     = 2
	CBRetrySleepDuration = 10 * time.Millisecond // TODO: fine tune this value
	ComponentName        = "accumulator"
)

type AccumulatorBySize struct {
	logg             *slog.Logger
	limitOfBytes     int
	separator        []byte
	separatorLen     int
	internalDataChan chan []byte //TODO: should we expose this for a distributor to be able to run a select on multiple channels?
	current          [][]byte
	next             domain.DataFlow
	metrics          *metricCollector
	shutdownMutex    sync.RWMutex
	circBreaker      circuitbreaker.CircuitBreaker
	shuttingDown     bool
	doneChan         chan struct{}
	doneChanMu       sync.Mutex
}

func NewAccumulatorBySize(
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
) flow.DataFlowRunnable {

	if limitOfBytes <= 1 {
		logg.Error("limit of bytes in accumulator should be >= 2", "flow", flowName)
		panic("limit of bytes in accumulator should be >= 2")
	}

	if len(separator) >= limitOfBytes {
		logg.Error("separator length in bytes should be smaller than limit", "flow", flowName)
		panic("separator length in bytes should be smaller than limit")
	}

	if queueCapacity < MinQueueCapacity {
		logg.Error(fmt.Sprintf("the accumulator capacity cannot be less than %d", //TODO: move this validation to config
			MinQueueCapacity), "flow", flowName)
		panic(fmt.Sprintf("the accumulator capacity cannot be less than %d", MinQueueCapacity))
	}

	metrics := newMetricCollector(flowName, metricRegistry)
	metrics.queueCapacity(queueCapacity)

	return &AccumulatorBySize{
		logg:             logg.With(logger.ComponentKey, ComponentName),
		limitOfBytes:     limitOfBytes,
		separator:        separator,
		separatorLen:     len(separator),
		internalDataChan: make(chan []byte, queueCapacity),
		current:          make([][]byte, 0, 1024), //TODO: create the initial size based on the capacity
		next:             next,
		metrics:          metrics,
		circBreaker:      cb,
	}
}

func (acc *AccumulatorBySize) Enqueue(data []byte) error {
	acc.shutdownMutex.RLock()
	defer acc.shutdownMutex.RUnlock()
	if acc.shuttingDown {
		return errors.New("accumulator shutting down")
	}

	acc.metrics.increaseEnqueueCounter()

	select {
	case acc.internalDataChan <- data:
		acc.updateEnqueuedItemsMetric()
	default:
		acc.metrics.incEnqueueFailed()
		return errors.New("enqueueing data on the accumulator failed, queue is full")
	}

	return nil
}

// Run should be called in a new goroutine
func (acc *AccumulatorBySize) Run(ctx context.Context) {
	acc.doneChanMu.Lock()
	acc.doneChan = make(chan struct{})
	defer close(acc.doneChan)
	acc.doneChanMu.Unlock()

	acc.logg.Info("Starting non-blocking accumulator")
	for {
		select {
		case data := <-acc.internalDataChan:
			acc.append(data)
			acc.updateEnqueuedItemsMetric()
		case <-ctx.Done():
			acc.logg.Debug("accumulator starting shutdown")
			acc.shutdown()
			acc.logg.Info("accumulator shutdown finished")
			return
		}
	}
}

func (acc *AccumulatorBySize) Done() <-chan struct{} {
	acc.doneChanMu.Lock()
	defer acc.doneChanMu.Unlock()
	return acc.doneChan
}

func (acc *AccumulatorBySize) append(data []byte) {

	dataLen := len(data)

	noData := dataLen == 0
	if noData {
		return
	}

	acc.metrics.incDataInBytesBy(dataLen)
	receivedDataTooBigForBuffer := dataLen >= acc.limitOfBytes
	if receivedDataTooBigForBuffer {
		acc.flush()
		acc.enqueueOnNext(data)

		return
	}

	bufferLenAfterAppend := (acc.currentBufferLen() + acc.separatorLen + dataLen)
	appendingDataWillViolateSizeLimit := bufferLenAfterAppend > acc.limitOfBytes

	if appendingDataWillViolateSizeLimit {
		acc.flush()
	}

	acc.current = append(acc.current, data)

	if bufferLenAfterAppend == acc.limitOfBytes {
		acc.flush()
	}
}

func (acc *AccumulatorBySize) flush() {
	chunksCount := len(acc.current)
	if chunksCount == 0 {
		return
	}

	mergedDataLen := acc.currentBufferLen()
	mergedData := make([]byte, mergedDataLen)

	var position int
	var isLastChunk bool
	for i := 0; i < chunksCount; i++ {
		position += copy(mergedData[position:], acc.current[i])

		isLastChunk = i >= (chunksCount - 1)
		if !isLastChunk {
			position += copy(mergedData[position:], acc.separator)
		}
	}

	acc.enqueueOnNext(mergedData)
	acc.current = acc.current[:0]

}

func (acc *AccumulatorBySize) currentBufferLen() int {
	// TODO: save this result in a variable so we don't call it multiple times?
	separatorLen := acc.separatorLen
	var total int

	for _, data := range acc.current {
		total += len(data)
		total += separatorLen
	}
	total -= separatorLen
	return total
}

func (acc *AccumulatorBySize) enqueueOnNext(data []byte) {
	dataSize := len(data)

	_, err := acc.circBreaker.Execute(func() (interface{}, error) {
		return nil, acc.next.Enqueue(data)
	})

	for err != nil {
		time.Sleep(CBRetrySleepDuration)
		_, err = acc.circBreaker.Execute(func() (interface{}, error) {
			return nil, acc.next.Enqueue(data)
		})
	}

	acc.metrics.incDataOutBytesBy(dataSize)
	acc.metrics.increaseNextCounter()
}

func (acc *AccumulatorBySize) updateEnqueuedItemsMetric() {
	itemsCount := len(acc.internalDataChan)
	acc.metrics.enqueuedItems(itemsCount)
}

func (acc *AccumulatorBySize) shutdown() {
	acc.setShutdown()
	close(acc.internalDataChan)

	for {
		data, more := <-acc.internalDataChan
		if !more {
			break
		}

		acc.append(data)
		acc.updateEnqueuedItemsMetric()
	}

	acc.flush()
}

func (acc *AccumulatorBySize) setShutdown() {
	acc.shutdownMutex.Lock()
	defer acc.shutdownMutex.Unlock()
	acc.shuttingDown = true
}
