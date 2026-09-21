package agent

type Agent struct {
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	Description    string   `yaml:"description"`
	Model          string   `yaml:"model"`
	SystemPrompt   string   `yaml:"system_prompt"`
	Skills         []string `yaml:"skills"`
	Tools          []string `yaml:"tools"`
	MaxIterations  int      `yaml:"max_iterations"`
	Temperature    *float64 `yaml:"temperature"`
	ConversationID string   `yaml:"conversation_id"`
}
