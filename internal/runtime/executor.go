package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/Jeffery-source/agent-runtime/internal/task"
)

type Runner interface {
	Run(
		ctx context.Context,
		req RunRequest,
	) (*RunResponse, error)
}

type Executor struct {
	tasks *task.Manager
	run   Runner
}

func NewExecutor(
	tasks *task.Manager,
	runner Runner,
) *Executor {
	return &Executor{
		tasks: tasks,
		run:   runner,
	}
}

func (e *Executor) Execute(
	ctx context.Context,
	taskID string,
) (*task.Task, error) {

	// 1. Task 必须存在。
	currentTask, err := e.tasks.Get(taskID)
	if err != nil {
		return nil, fmt.Errorf(
			"get task: %w",
			err,
		)
	}

	// 2. Context 已经取消时，
	//    Task 尚未开始执行，因此不改变 Task 状态。
	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	default:
	}

	// 3. Task: pending → running。
	err = e.tasks.Start(taskID)
	if err != nil {
		return nil, fmt.Errorf(
			"start task: %w",
			err,
		)
	}

	// 4. 获取 running 状态的 Task。
	currentTask, err = e.tasks.Get(taskID)
	if err != nil {
		return nil, fmt.Errorf(
			"get started task: %w",
			err,
		)
	}

	// 5. 调用 Agent Runtime。
	response, err := e.run.Run(
		ctx,
		RunRequest{
			AgentID:   currentTask.AgentID,
			SessionID: currentTask.SessionID,
			Input:     currentTask.Input,
		},
	)

	// 6. Runtime 执行失败。
	if err != nil {

		// Context cancellation / deadline
		// Task → canceled。
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {

			cancelErr := e.tasks.Cancel(taskID)

			if cancelErr != nil {
				return nil, fmt.Errorf(
					"cancel task: %w",
					cancelErr,
				)
			}

			return nil, err
		}

		// 普通 Runtime 错误。
		// Task → failed。
		failErr := e.tasks.Fail(
			taskID,
			err,
		)

		if failErr != nil {
			return nil, fmt.Errorf(
				"fail task: %w",
				failErr,
			)
		}

		return nil, err
	}

	// 7. Runtime 成功，但是不能返回 nil response。
	if response == nil {

		err = errors.New(
			"runtime returned nil response",
		)

		failErr := e.tasks.Fail(
			taskID,
			err,
		)

		if failErr != nil {
			return nil, fmt.Errorf(
				"fail task: %w",
				failErr,
			)
		}

		return nil, err
	}

	// 8. Runtime 成功。
	// Task → completed。
	err = e.tasks.Complete(
		taskID,
		response.Content,
		response.Execution,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"complete task: %w",
			err,
		)
	}

	// 9. 返回最终 Task。
	return e.tasks.Get(taskID)
}
