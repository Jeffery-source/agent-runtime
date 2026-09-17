package execution

import "fmt"

type Execution struct {
	Steps []Step `json:"steps"`
}

func NewExecution() *Execution {
	return &Execution{
		Steps: make([]Step, 0),
	}
}

func (e *Execution) AddStep(step Step) {
	step.ID = fmt.Sprintf("step_%03d", len(e.Steps)+1)
	e.Steps = append(e.Steps, step)
}

func (e *Execution) GetSteps() []Step {
	return e.Steps
}
