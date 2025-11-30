package worker_test

import (
	"sync"
	"time"

	"github.com/jademcosta/jiboia/pkg/domain"
)

func constantTimeProvider(fixedTime time.Time) func() time.Time {
	return func() time.Time {
		return fixedTime
	}
}

type mockObjStorage struct {
	mu         sync.Mutex
	wg         *sync.WaitGroup
	calledWith []*domain.WorkUnit
	returning  *domain.UploadResult
	err        error
	delay      time.Duration
}

func (objStorage *mockObjStorage) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {
	objStorage.mu.Lock()
	defer objStorage.mu.Unlock()
	defer objStorage.wg.Done()

	if objStorage.delay > 0 {
		time.Sleep(objStorage.delay)
	}

	objStorage.calledWith = append(objStorage.calledWith, workU)

	if objStorage.err != nil {
		return nil, objStorage.err
	}

	if objStorage.returning != nil {
		return objStorage.returning, nil
	}
	return &domain.UploadResult{}, nil
}

type dummyObjStorage struct{}

func (objStorage *dummyObjStorage) Upload(_ *domain.WorkUnit) (*domain.UploadResult, error) {
	return &domain.UploadResult{}, nil
}

type mockExternalQueue struct {
	calledWith []*domain.MessageContext
	mu         sync.Mutex
	wg         *sync.WaitGroup
	err        error
	delay      time.Duration
}

func (queue *mockExternalQueue) Enqueue(data *domain.MessageContext) error {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	defer queue.wg.Done()

	if queue.delay > 0 {
		time.Sleep(queue.delay)
	}

	queue.calledWith = append(queue.calledWith, data)
	return queue.err
}

type dummyExternalQueue struct{}

func (queue *dummyExternalQueue) Enqueue(_ *domain.MessageContext) error {
	return nil
}
