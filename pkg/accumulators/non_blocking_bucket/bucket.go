package non_blocking_bucket

import (
	"context"
	"fmt"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	MINIMAL_QUEUE_CAPACITY = 30
)

type BucketAccumulator struct {
	l                  *zap.SugaredLogger
	limitOfBytes       int
	separator          []byte
	separatorLen       int
	internalDataChan   chan []byte //TODO: should we expose this for a distributor to be able to run a select on multiple channels?
	dataDropper        domain.DataDropper
	current            [][]byte // TODO: replace this with a list. The trashing of having to reallocate a new array is not worth the simplicity
	next               domain.DataFlow
	enqueueCounter     *prometheus.CounterVec
	nextCounter        *prometheus.CounterVec
	capacityGauge      *prometheus.GaugeVec
	enqueuedItemsGauge *prometheus.GaugeVec
}

func New(
	l *zap.SugaredLogger,
	limitOfBytes int,
	separator []byte,
	queueCapacity int,
	dataDropper domain.DataDropper,
	next domain.DataFlow,
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

	enqueueCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "jiboia",
			Subsystem: "accumulator",
			Name:      "enqueue_calls_total",
			Help:      "The total number of times that data was enqueued."},
		[]string{})

	nextCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "jiboia",
			Subsystem: "accumulator",
			Name:      "next_calls_total",
			Help:      "The total number of times that data was sent to next step."},
		[]string{})

	capacityGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "accumulator",
			Name:      "queue_capacity",
			Help:      "The total capacity of the internal queue.",
		},
		[]string{},
	)

	enqueuedItemsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "accumulator",
			Name:      "items_in_queue",
			Help:      "The count of current items in the internal queue.",
		},
		[]string{},
	)

	metricRegistry.MustRegister(enqueueCounter, nextCounter, capacityGauge, enqueuedItemsGauge)
	capacityGauge.WithLabelValues().Set(float64(queueCapacity))

	return &BucketAccumulator{
		l:                  l.With(logger.COMPONENT_KEY, "accumulator"),
		limitOfBytes:       limitOfBytes,
		separator:          separator,
		separatorLen:       len(separator),
		internalDataChan:   make(chan []byte, queueCapacity),
		dataDropper:        dataDropper,
		current:            make([][]byte, 0), // TODO: make([][]byte, 0, 2)
		next:               next,
		enqueueCounter:     enqueueCounter,
		nextCounter:        nextCounter,
		capacityGauge:      capacityGauge,
		enqueuedItemsGauge: enqueuedItemsGauge,
	}
}

func (b *BucketAccumulator) Enqueue(data []byte) error {
	b.enqueueCounter.WithLabelValues().Inc()

	select {
	case b.internalDataChan <- data:
		b.updateEnqueuedItemsMetric()
	default:
		b.dataDropped(data)
		return fmt.Errorf("enqueueing data to accumulate on bucket failed, queue is full")
	}

	return nil
}

//Run should be called in a new goroutine
func (b *BucketAccumulator) Run(ctx context.Context) {
	// TODO: add flush based on time
	b.l.Info("Starting non-blocking accumulator")
	for {
		select {
		case data := <-b.internalDataChan:
			b.append(data)
			b.updateEnqueuedItemsMetric()
		case <-ctx.Done():
			//TODO: implenment graceful shutdown
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
		b.next.Enqueue(data)
		b.nextCounter.WithLabelValues().Inc()

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

	b.next.Enqueue(mergedData)
	b.current = make([][]byte, 0) // TODO: make([][]byte, 0, 2)
	b.nextCounter.WithLabelValues().Inc()
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

func (b *BucketAccumulator) updateEnqueuedItemsMetric() {
	itemsCount := len(b.internalDataChan)
	b.enqueuedItemsGauge.WithLabelValues().Set(float64(itemsCount))
}
