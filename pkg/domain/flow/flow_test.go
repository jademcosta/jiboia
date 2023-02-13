package flow_test

import (
	"context"
	"testing"

	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/stretchr/testify/assert"
)

type mockRunnableFlow struct {
	calledWith [][]byte
}

func (m *mockRunnableFlow) Run(ctx context.Context) {
	panic("should not be called")
}

func (m *mockRunnableFlow) Enqueue(data []byte) error {
	m.calledWith = append(m.calledWith, data)
	return nil
}
func TestAccumulatorIsEntrypointWhenPresent(t *testing.T) {
	uploader := &mockRunnableFlow{calledWith: make([][]byte, 0)}
	accumulator := &mockRunnableFlow{calledWith: make([][]byte, 0)}

	sut, err := flow.New(nil, nil, uploader, accumulator, nil, "whatever")
	assert.NoError(t, err, "should not error when creating flow")
	sut.Entrypoint.Enqueue([]byte("hello123"))

	assert.Lenf(t, uploader.calledWith, 0, "uploader shouldn't be the entrypoint")
	assert.Lenf(t, accumulator.calledWith, 1, "accumulator should be the entrypoint")
	assert.Equalf(t, []byte("hello123"), accumulator.calledWith[0], "data enqueued should be at the accumulator")
}

func TestUploaderIsEntrypointWhenAccumulatorIsNil(t *testing.T) {
	uploader := &mockRunnableFlow{calledWith: make([][]byte, 0)}

	sut, err := flow.New(nil, nil, uploader, nil, nil, "whatever")
	assert.NoError(t, err, "should not error when creating flow")
	sut.Entrypoint.Enqueue([]byte("hello123"))

	assert.Lenf(t, uploader.calledWith, 1, "uploader should be the entrypoint")
	assert.Equalf(t, []byte("hello123"), uploader.calledWith[0], "data enqueued should be at the accumulator")
}

func TestErrorsWHenUploaderIsNil(t *testing.T) {
	accumulator := &mockRunnableFlow{calledWith: make([][]byte, 0)}

	sut, err := flow.New(nil, nil, nil, accumulator, nil, "whatever")
	assert.Error(t, err, "should error when creating flow and uploader is nil")
	assert.Nil(t, sut, "should return a nil flow when uploader is nil")
}
