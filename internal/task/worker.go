package task

import (
	"context"
	"errors"
	"fmt"
)

type Worker struct {
	service TaskService
}

func NewWorker(
	service TaskService,
) *Worker {
	return &Worker{
		service: service,
	}
}

func (w *Worker) Execute(
	ctx context.Context,
	taskID string,
) (*Task, error) {

	if w.service == nil {
		return nil, errors.New("task service is nil")
	}

	if taskID == "" {
		return nil, errors.New("task ID is empty")
	}

	result, err := w.service.Execute(
		ctx,
		taskID,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"execute task: %w",
			err,
		)
	}

	return result, nil
}
