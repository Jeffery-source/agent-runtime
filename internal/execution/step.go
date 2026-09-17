package execution

type StepType string

const (
	StepDecision    StepType = "decision"
	StepAction      StepType = "action"
	StepObservation StepType = "observation"
	StepFinish      StepType = "finish"
	StepInput       StepType = "input"
)

type Step struct {
	ID        string   `json:"id"`
	Type      StepType `json:"type"`
	Iteration int      `json:"iteration"`

	// AI 的决策
	Decision  Decision `json:"decision,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Reasoning string   `json:"reasoning,omitempty"`
	// 用户可见内容
	Content string `json:"content,omitempty"`
	// Tool 调用
	ToolCall *ToolCallInfo `json:"tool_call,omitempty"`
	// Tool 返回结果
	Result string `json:"result,omitempty"`
}

type ToolCallInfo struct {
	ID string `json:"id"`

	Name string `json:"name"`

	Arguments string `json:"arguments"`
}
