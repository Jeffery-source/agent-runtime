package agent

type Agent struct {
	ID            string
	Name          string
	Description   string
	Model         string
	SystemPrompt  string
	Tools         []string
	MaxIterations int
}
