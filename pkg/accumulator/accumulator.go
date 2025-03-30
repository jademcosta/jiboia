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
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	MinQueueCapacity     = 2
	CBRetrySleepDuration = 10 * time.Millisecond // TODO: fine tune this value
	ComponentName        = "accumulator"
)

type Accumulator struct {
	l                *slog.Logger
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

func New(
	flowName string,
	l *slog.Logger,
	limitOfBytes int,
	separator []byte,
	queueCapacity int,
	next domain.DataFlow,
	cb circuitbreaker.CircuitBreaker,
	metricRegistry *prometheus.Registry) *Accumulator {

	if limitOfBytes <= 1 {
		l.Error("limit of bytes in accumulator should be >= 2", "flow", flowName)
		panic("limit of bytes in accumulator should be >= 2")
	}

	if len(separator) >= limitOfBytes {
		l.Error("separator length in bytes should be smaller than limit", "flow", flowName)
		panic("separator length in bytes should be smaller than limit")
	}

	if queueCapacity < MinQueueCapacity {
		l.Error(fmt.Sprintf("the accumulator capacity cannot be less than %d", //TODO: move this validation to config
			MinQueueCapacity), "flow", flowName)
		panic(fmt.Sprintf("the accumulator capacity cannot be less than %d", MinQueueCapacity))
	}

	metrics := newMetricCollector(flowName, metricRegistry)
	metrics.queueCapacity(queueCapacity)

	return &Accumulator{
		l:                l.With(logger.ComponentKey, ComponentName),
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

func (b *Accumulator) Enqueue(data []byte) error {
	b.shutdownMutex.RLock()
	defer b.shutdownMutex.RUnlock()
	if b.shuttingDown {
		return errors.New("accumulator shutting down")
	}

	b.metrics.increaseEnqueueCounter()

	select {
	case b.internalDataChan <- data:
		b.updateEnqueuedItemsMetric()
	default:
		b.metrics.incEnqueueFailed()
		return errors.New("enqueueing data on the accumulator failed, queue is full")
	}

	return nil
}

// Run should be called in a new goroutine
func (b *Accumulator) Run(ctx context.Context) {
	b.doneChanMu.Lock()
	b.doneChan = make(chan struct{})
	defer close(b.doneChan)
	b.doneChanMu.Unlock()

	b.l.Info("Starting non-blocking accumulator")
	for {
		select {
		case data := <-b.internalDataChan:
			b.append(data)
			b.updateEnqueuedItemsMetric()
		case <-ctx.Done():
			b.l.Debug("accumulator starting shutdown")
			b.shutdown()
			b.l.Info("accumulator shutdown finished")
			return
		}
	}
}

func (b *Accumulator) Done() <-chan struct{} {
	b.doneChanMu.Lock()
	defer b.doneChanMu.Unlock()
	return b.doneChan
}

func (b *Accumulator) append(data []byte) {

	dataLen := len(data)

	noData := dataLen == 0
	if noData {
		return
	}

	b.metrics.incDataInBytesBy(dataLen)
	receivedDataTooBigForBuffer := dataLen >= b.limitOfBytes
	if receivedDataTooBigForBuffer {
		b.flush()
		b.enqueueOnNext(data)

		return
	}

	bufferLenAfterAppend := (b.currentBufferLen() + b.separatorLen + dataLen)
	appendingDataWillViolateSizeLimit := bufferLenAfterAppend > b.limitOfBytes

	if appendingDataWillViolateSizeLimit {
		b.flush()
	}

	b.current = append(b.current, data)

	if bufferLenAfterAppend == b.limitOfBytes {
		b.flush()
	}
}

func (b *Accumulator) flush() {
	chunksCount := len(b.current)
	if chunksCount == 0 {
		return
	}

	mergedDataLen := b.currentBufferLen()
	mergedData := make([]byte, mergedDataLen)

	var position int
	var isLastChunk bool
	for i := 0; i < chunksCount; i++ {
		position += copy(mergedData[position:], b.current[i])

		isLastChunk = i >= (chunksCount - 1)
		if !isLastChunk {
			position += copy(mergedData[position:], b.separator)
		}
	}

	b.enqueueOnNext(mergedData)
	b.current = b.current[:0]

}

func (b *Accumulator) currentBufferLen() int {
	// TODO: save this result in a variable so we don't call it multiple times?
	separatorLen := b.separatorLen
	var total int

	for _, data := range b.current {
		total += len(data)
		total += separatorLen
	}
	total -= separatorLen
	return total
}

func (b *Accumulator) enqueueOnNext(data []byte) {
	dataSize := len(data)

	_, err := b.circBreaker.Execute(func() (interface{}, error) {
		return nil, b.next.Enqueue(data)
	})

	for err != nil {
		time.Sleep(CBRetrySleepDuration)
		_, err = b.circBreaker.Execute(func() (interface{}, error) {
			return nil, b.next.Enqueue(data)
		})
	}

	b.metrics.incDataOutBytesBy(dataSize)
	b.metrics.increaseNextCounter()
}

func (b *Accumulator) updateEnqueuedItemsMetric() {
	itemsCount := len(b.internalDataChan)
	b.metrics.enqueuedItems(itemsCount)
}

func (b *Accumulator) shutdown() {
	b.setShutdown()
	close(b.internalDataChan)

	for {
		data, more := <-b.internalDataChan
		if !more {
			break
		}

		b.append(data)
		b.updateEnqueuedItemsMetric()
	}

	b.flush()
}

func (b *Accumulator) setShutdown() {
	b.shutdownMutex.Lock()
	defer b.shutdownMutex.Unlock()
	b.shuttingDown = true
}
