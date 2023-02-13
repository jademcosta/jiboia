package flow

import (
	"context"
	"fmt"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/uploaders"
)

type Runnable interface {
	Run(context.Context)
}

type RunnableFlow interface {
	domain.DataFlow
	Runnable
}

type Flow struct {
	Name          string
	objStorage    uploaders.ObjStorage
	externalQueue uploaders.ExternalQueue
	Uploader      RunnableFlow
	Accumulator   RunnableFlow
	Entrypoint    domain.DataFlow
	Workers       []Runnable
}

func New(objStorage uploaders.ObjStorage,
	externalQueue uploaders.ExternalQueue,
	uploader RunnableFlow,
	accumulator RunnableFlow,
	workers []Runnable,
	name string) (*Flow, error) {

	if uploader == nil {
		return nil, fmt.Errorf("uploader cannot be nil")
	}

	var entryPoint domain.DataFlow
	if accumulator == nil {
		entryPoint = uploader
	} else {
		entryPoint = accumulator
	}

	return &Flow{
		objStorage:    objStorage,
		externalQueue: externalQueue,
		Uploader:      uploader,
		Accumulator:   accumulator,
		Entrypoint:    entryPoint,
		Workers:       workers,
		Name:          name,
	}, nil
}