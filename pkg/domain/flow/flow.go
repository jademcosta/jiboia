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
	domain.DataEnqueuer
	Runnable
}

type Flow struct {
	Name                                string
	ObjStorage                          worker.ObjStorage
	ExternalQueue                       worker.ExternalQueue
	Entrypoint                          domain.DataEnqueuer
	Uploader                            DataFlowRunnable
	Accumulator                         DataFlowRunnable
	UploadWorkers                       []Runnable
	Token                               string
	DecompressionAlgorithms             []string
	DecompressionMaxConcurrency         int
	DecompressionInitialBufferSizeBytes int
	CircuitBreaker                      circuitbreaker.TwoStepCircuitBreaker
}
