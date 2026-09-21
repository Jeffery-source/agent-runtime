package skill

// Skill 表示一个可复用的业务能力/方法。
//
// Skill 负责描述“怎么完成一类任务”，
// Tool 负责提供具体的执行能力。
type Skill struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Instructions string   `yaml:"instructions"`
	Tools        []string `yaml:"tools"`
}
