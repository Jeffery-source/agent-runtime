package execution

type Decision string

const (
	DecisionCallTool  Decision = "call_tool"
	DecisionCallTools Decision = "call_tools"
	DecisionFinish    Decision = "finish"
	DecisionRetry     Decision = "retry"
	DecisionContinue  Decision = "continue"
	DecisionAskUser   Decision = "ask_user"
	DecisionDelegate  Decision = "delegate"
)
