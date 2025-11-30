package flow

import (
	"context"

	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/worker"
)

type Runnable interface {
	Run(context.Context)
	Done() <-chan struct{}
}

type DataFlowRunnable interface {
	domain.DataFlow
	Runnable
}

type Flow struct {
	Name                                string
	ObjStorages                         []worker.ObjStorage
	ExternalQueues                      []worker.ExternalQueue
	Entrypoint                          domain.DataFlow
	Uploader                            DataFlowRunnable
	Accumulator                         DataFlowRunnable
	UploadWorkers                       []Runnable
	Token                               string
	DecompressionAlgorithms             []string
	DecompressionMaxConcurrency         int
	DecompressionInitialBufferSizeBytes int
	CircuitBreaker                      circuitbreaker.TwoStepCircuitBreaker
}
