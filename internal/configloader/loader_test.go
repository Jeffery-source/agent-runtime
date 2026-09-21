package configloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Jeffery-source/agent-runtime/internal/agent"
	"github.com/Jeffery-source/agent-runtime/internal/skill"
	"github.com/Jeffery-source/agent-runtime/internal/tool"
)

func TestLoadRegistersAgentSkillAndToolDefinition(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "agents/demo.yaml", `
id: demo-agent
name: Demo Agent
description: test agent
model: qwen3
temperature: 0.2
system_prompt: You are a helpful assistant.
skills: [quality-analysis]
tools: [query_attendance]
max_iterations: 10
`)
	writeConfig(t, root, "skills/quality.yaml", `
id: quality-analysis
name: Quality Analysis
description: Analyze manufacturing quality.
instructions: Follow the quality-analysis process.
`)
	writeConfig(t, root, "tools/attendance.yaml", `
name: query_attendance
description: Query attendance data.
schema:
  type: object
  properties:
    employee_id:
      type: string
type: builtin
enabled: true
`)

	cfg, err := NewLoader(root).Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Agents) != 1 || cfg.Agents[0].SystemPrompt != "You are a helpful assistant." || cfg.Agents[0].Temperature == nil || *cfg.Agents[0].Temperature != 0.2 {
		t.Fatalf("agent YAML was not mapped correctly: %#v", cfg.Agents)
	}
	agents := agent.NewRegistry()
	if err := agents.Register(&cfg.Agents[0]); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	if got, err := agents.Get("demo-agent"); err != nil || got.MaxIterations != 10 {
		t.Fatalf("agent registry: got=%#v err=%v", got, err)
	}

	skills := skill.NewRegistry()
	if err := skills.Register(&cfg.Skills[0]); err != nil {
		t.Fatalf("register skill: %v", err)
	}
	if got, ok := skills.Get("quality-analysis"); !ok || got.Instructions == "" {
		t.Fatalf("skill registry: got=%#v ok=%v", got, ok)
	}

	def, err := cfg.Tools[0].Definition()
	if err != nil {
		t.Fatalf("tool definition: %v", err)
	}
	if def.Name != "query_attendance" || def.Type != "builtin" || !def.Enabled || string(def.Schema) == "" {
		t.Fatalf("unexpected tool definition: %#v", def)
	}
	registry := tool.NewRegistry()
	if err := registry.Register(&configTestTool{}); err != nil {
		t.Fatalf("register implementation: %v", err)
	}
	if err := registry.Configure(def); err != nil {
		t.Fatalf("configure implementation: %v", err)
	}
	configured, err := registry.Get(def.Name)
	if err != nil || configured.Description() != def.Description || string(configured.InputSchema()) != string(def.Schema) {
		t.Fatalf("configured tool: %#v err=%v", configured, err)
	}
}

func TestLoadMissingDirectoryReturnsError(t *testing.T) {
	if _, err := NewLoader(t.TempDir()).Load(); err == nil {
		t.Fatal("expected missing configuration directories to fail")
	}
}

func writeConfig(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
