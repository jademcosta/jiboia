package flow

import (
	"context"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/uploaders"
)

type Runnable interface {
	Run(context.Context)
}

type DataFlowRunnable interface {
	domain.DataFlow
	Runnable
}

type Flow struct {
	Name          string
	ObjStorage    uploaders.ObjStorage
	ExternalQueue uploaders.ExternalQueue
	Entrypoint    domain.DataFlow
	Uploader      DataFlowRunnable
	Accumulator   DataFlowRunnable
	UploadWorkers []Runnable
}
