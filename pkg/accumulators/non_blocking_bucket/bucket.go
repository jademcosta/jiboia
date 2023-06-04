package non_blocking_bucket

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	MINIMAL_QUEUE_CAPACITY  = 30
	CB_RETRY_SLEEP_DURATION = 10 * time.Millisecond // TODO: fine tune this value
)

type BucketAccumulator struct {
	l                *zap.SugaredLogger
	limitOfBytes     int
	separator        []byte
	separatorLen     int
	internalDataChan chan []byte //TODO: should we expose this for a distributor to be able to run a select on multiple channels?
	dataDropper      domain.DataDropper
	current          [][]byte
	next             domain.DataFlow
	metrics          *metricCollector
	shutdownMutex    sync.RWMutex
	circBreaker      circuitbreaker.CircuitBreaker
	shuttingDown     bool
}

func New(
	flowName string,
	l *zap.SugaredLogger,
	limitOfBytes int,
	separator []byte,
	queueCapacity int,
	dataDropper domain.DataDropper,
	next domain.DataFlow,
	cb circuitbreaker.CircuitBreaker,
	metricRegistry *prometheus.Registry) *BucketAccumulator {

	if limitOfBytes <= 1 {
		panic("limit of bytes in accumulator should be >= 2")
	}

	if len(separator) >= limitOfBytes {
		panic("separator length in bytes should be smaller than limit")
	}

	if queueCapacity < MINIMAL_QUEUE_CAPACITY {
		queueCapacity = MINIMAL_QUEUE_CAPACITY
	}

	metrics := NewMetricCollector(flowName, metricRegistry)
	metrics.queueCapacity(queueCapacity)

	return &BucketAccumulator{
		l:                l.With(logger.COMPONENT_KEY, "accumulator"),
		limitOfBytes:     limitOfBytes,
		separator:        separator,
		separatorLen:     len(separator),
		internalDataChan: make(chan []byte, queueCapacity),
		dataDropper:      dataDropper,
		current:          make([][]byte, 0, 1024), //TODO: create the initial size based on the capacity
		next:             next,
		metrics:          metrics,
		circBreaker:      cb,
	}
}

func (b *BucketAccumulator) Enqueue(data []byte) error {
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
		b.dataDropped(data)
		return errors.New("enqueueing data on the accumulator failed, queue is full")
	}

	return nil
}

// Run should be called in a new goroutine
func (b *BucketAccumulator) Run(ctx context.Context) {
	// TODO: add flush based on time
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

func (b *BucketAccumulator) append(data []byte) {

	dataLen := len(data)

	noData := dataLen == 0
	if noData {
		return
	}

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

func (b *BucketAccumulator) flush() {
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

func (b *BucketAccumulator) currentBufferLen() int {
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

func (b *BucketAccumulator) dataDropped(data []byte) {
	b.dataDropper.Drop(data)
}

func (b *BucketAccumulator) enqueueOnNext(data []byte) {
	err := b.circBreaker.Call(func() error {
		return b.next.Enqueue(data)
	})

	for err != nil {
		time.Sleep(CB_RETRY_SLEEP_DURATION) // TODO: should this be configurable?
		err = b.circBreaker.Call(
			func() error {
				return b.next.Enqueue(data)
			},
		)
	}

	b.metrics.increaseNextCounter()
}

func (b *BucketAccumulator) updateEnqueuedItemsMetric() {
	itemsCount := len(b.internalDataChan)
	b.metrics.enqueuedItems(itemsCount)
}

func (b *BucketAccumulator) shutdown() {
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

func (b *BucketAccumulator) setShutdown() {
	b.shutdownMutex.Lock()
	defer b.shutdownMutex.Unlock()
	b.shuttingDown = true
}
