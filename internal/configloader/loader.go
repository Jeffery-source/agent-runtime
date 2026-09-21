// Package configloader loads Runtime definitions from a directory of YAML files.
package configloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Jeffery-source/agent-runtime/internal/agent"
	"github.com/Jeffery-source/agent-runtime/internal/skill"
	"github.com/Jeffery-source/agent-runtime/internal/tool"
	"gopkg.in/yaml.v3"
)

// Config contains the configuration-owned definitions used to populate the
// existing domain registries.
type Config struct {
	Agents []agent.Agent
	Skills []skill.Skill
	Tools  []ToolConfig
}

// ToolConfig is the YAML definition of a tool; it has no Execute method.
type ToolConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Schema      any    `yaml:"schema"`
	Type        string `yaml:"type"`
	Enabled     bool   `yaml:"enabled"`
}

func (c ToolConfig) Definition() (tool.Definition, error) {
	def := tool.Definition{
		Name: c.Name, Description: c.Description, Type: c.Type, Enabled: c.Enabled,
	}
	if c.Schema == nil {
		return def, nil
	}
	schema, err := json.Marshal(c.Schema)
	if err != nil {
		return tool.Definition{}, fmt.Errorf("marshal schema for tool %q: %w", c.Name, err)
	}
	def.Schema = schema
	return def, nil
}

type Loader struct{ path string }

func NewLoader(path string) *Loader { return &Loader{path: path} }

// Load reads config/{agents,skills,tools}/*.yaml in lexical file order.
func (l *Loader) Load() (*Config, error) {
	cfg := &Config{}
	if err := loadDirectory(filepath.Join(l.path, "agents"), func(data []byte, file string) error {
		var value agent.Agent
		if err := yaml.Unmarshal(data, &value); err != nil {
			return err
		}
		if value.ID == "" {
			return fmt.Errorf("agent id is required")
		}
		cfg.Agents = append(cfg.Agents, value)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := loadDirectory(filepath.Join(l.path, "skills"), func(data []byte, file string) error {
		var value skill.Skill
		if err := yaml.Unmarshal(data, &value); err != nil {
			return err
		}
		if value.ID == "" {
			return fmt.Errorf("skill id is required")
		}
		cfg.Skills = append(cfg.Skills, value)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := loadDirectory(filepath.Join(l.path, "tools"), func(data []byte, file string) error {
		var value ToolConfig
		if err := yaml.Unmarshal(data, &value); err != nil {
			return err
		}
		if value.Name == "" {
			return fmt.Errorf("tool name is required")
		}
		cfg.Tools = append(cfg.Tools, value)
		return nil
	}); err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadDirectory(path string, load func([]byte, string) error) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read config directory %q: %w", path, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && (filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	for _, name := range files {
		filename := filepath.Join(path, name)
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read %q: %w", filename, err)
		}
		if err := load(data, filename); err != nil {
			return fmt.Errorf("load %q: %w", filename, err)
		}
	}
	return nil
}
