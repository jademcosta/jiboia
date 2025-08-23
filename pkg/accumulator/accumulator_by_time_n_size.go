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

// The time between each check of the oldest data in the accumulator
const (
	tickerInterval     = 1 * time.Second
	byTimeoutFlushType = "time"
)

type ByTimeAndSize struct {
	logg                *slog.Logger
	limitOfBytes        int
	separator           []byte
	separatorLen        int
	internalDataChan    chan *domain.WorkUnit
	currentAccumulation []*domain.WorkUnit
	next                domain.DataEnqueuer
	metrics             *metricCollector
	shutdownMutex       sync.RWMutex
	circBreaker         circuitbreaker.CircuitBreaker
	shuttingDown        bool
	doneChan            chan struct{}
	doneChanMu          sync.Mutex
	timeCheckTicker     *time.Ticker
	oldestDataTime      time.Time
	forceFlushAfter     time.Duration
	currentTimeProvider func() time.Time
}

func NewAccumulatorByTimeAndSize(
	flowName string,
	logg *slog.Logger,
	limitOfBytes int,
	separator []byte,
	queueCapacity int,
	next domain.DataEnqueuer,
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

	return &ByTimeAndSize{
		logg:                logg.With(logger.ComponentKey, ComponentName),
		limitOfBytes:        limitOfBytes,
		separator:           separator,
		separatorLen:        len(separator),
		internalDataChan:    make(chan *domain.WorkUnit, queueCapacity),
		currentAccumulation: make([]*domain.WorkUnit, 0, 1024), //TODO: create the initial size based on the capacity
		next:                next,
		metrics:             metrics,
		circBreaker:         cb,
		timeCheckTicker:     time.NewTicker(tickerInterval),
		currentTimeProvider: currentTimeProvider,
		forceFlushAfter:     forceFlushAfter,
	}
}

func (acc *ByTimeAndSize) Enqueue(payload *domain.WorkUnit) error {
	acc.shutdownMutex.RLock()
	defer acc.shutdownMutex.RUnlock()
	if acc.shuttingDown {
		return errors.New("accumulator shutting down")
	}

	acc.metrics.increaseEnqueueCounter()

	select {
	case acc.internalDataChan <- payload:
		acc.updateEnqueuedItemsMetric()
	default:
		acc.metrics.incEnqueueFailed()
		return errors.New("enqueueing data on the accumulator failed, queue is full")
	}

	return nil
}

// Run should be called in a new goroutine
func (acc *ByTimeAndSize) Run(ctx context.Context) {
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
		case currentTime := <-acc.timeCheckTicker.C:
			if !acc.oldestDataTime.IsZero() && currentTime.Sub(acc.oldestDataTime) >= acc.forceFlushAfter {
				acc.logg.Debug("flushing data due to timeout")
				acc.flush(byTimeoutFlushType)
			}
		}
	}
}

func (acc *ByTimeAndSize) Done() <-chan struct{} {
	acc.doneChanMu.Lock()
	defer acc.doneChanMu.Unlock()
	return acc.doneChan
}

func (acc *ByTimeAndSize) append(payload *domain.WorkUnit) {

	dataLen := payload.DataLen()
	noData := dataLen == 0
	if noData {
		return
	}

	acc.metrics.incDataInBytesBy(dataLen)
	receivedDataTooBigForBuffer := dataLen >= acc.limitOfBytes
	if receivedDataTooBigForBuffer {
		acc.flush(bySizeFlushType)
		acc.enqueueOnNext(payload)

		return
	}

	bufferLenAfterAppend := (acc.currentBufferLen() + acc.separatorLen + dataLen)
	appendingDataWillViolateSizeLimit := bufferLenAfterAppend > acc.limitOfBytes
	if appendingDataWillViolateSizeLimit {
		acc.flush(bySizeFlushType)
	}

	acc.currentAccumulation = append(acc.currentAccumulation, payload)
	acc.oldestDataTime = acc.currentTimeProvider()

	if bufferLenAfterAppend == acc.limitOfBytes {
		acc.flush(bySizeFlushType)
	}
}

func (acc *ByTimeAndSize) flush(typeOfFlush string) {
	chunksCount := len(acc.currentAccumulation)
	if chunksCount == 0 {
		return
	}

	mergedDataLen := acc.currentBufferLen()
	mergedData := make([]byte, mergedDataLen)

	var position int
	var isLastChunk bool
	for i := 0; i < chunksCount; i++ {
		position += copy(mergedData[position:], acc.currentAccumulation[i].Data)

		isLastChunk = i >= (chunksCount - 1)
		if !isLastChunk {
			position += copy(mergedData[position:], acc.separator)
		}
	}

	sendToNextPayload := &domain.WorkUnit{Data: mergedData}

	acc.enqueueOnNext(sendToNextPayload)
	acc.currentAccumulation = acc.currentAccumulation[:0]
	acc.oldestDataTime = time.Time{}
	acc.metrics.increaseFlushCounter(typeOfFlush)
}

func (acc *ByTimeAndSize) currentBufferLen() int {
	// TODO: save this result in a variable so we don't call it multiple times?
	separatorLen := acc.separatorLen
	var total int

	for _, payload := range acc.currentAccumulation {
		total += payload.DataLen()
		total += separatorLen
	}
	total -= separatorLen
	return total
}

func (acc *ByTimeAndSize) enqueueOnNext(payload *domain.WorkUnit) {
	dataSize := payload.DataLen()

	_, err := acc.circBreaker.Execute(func() (interface{}, error) {
		return nil, acc.next.Enqueue(payload)
	})

	for err != nil {
		time.Sleep(CBRetrySleepDuration)
		_, err = acc.circBreaker.Execute(func() (interface{}, error) {
			return nil, acc.next.Enqueue(payload)
		})
	}

	acc.metrics.incDataOutBytesBy(dataSize)
	acc.metrics.increaseNextCounter()
}

func (acc *ByTimeAndSize) updateEnqueuedItemsMetric() {
	itemsCount := len(acc.internalDataChan)
	acc.metrics.enqueuedItems(itemsCount)
}

func (acc *ByTimeAndSize) shutdown() {
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

	acc.flush(byShutdownFlushType)
}

func (acc *ByTimeAndSize) setShutdown() {
	acc.shutdownMutex.Lock()
	defer acc.shutdownMutex.Unlock()
	acc.shuttingDown = true
}
