package tool

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrToolNotFound = errors.New("tool not found")
	ErrToolExists   = errors.New("tool already exists")
)

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(t Tool) error {
	if t == nil {
		return errors.New("tool is nil")
	}

	if t.Name() == "" {
		return errors.New("tool name is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[t.Name()]; exists {
		return ErrToolExists
	}

	r.tools[t.Name()] = t

	return nil
}

// Configure applies YAML-owned metadata to an already registered Go tool.
// It deliberately does not create a Tool: Execute remains a Go implementation.
func (r *Registry) Configure(def Definition) error {
	if def.Name == "" {
		return errors.New("tool definition name is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	implementation, exists := r.tools[def.Name]
	if !exists {
		return ErrToolNotFound
	}
	if !def.Enabled {
		delete(r.tools, def.Name)
		return nil
	}

	r.tools[def.Name] = configuredTool{Tool: implementation, definition: def}
	return nil
}

type configuredTool struct {
	Tool
	definition Definition
}

func (t configuredTool) Description() string {
	if t.definition.Description != "" {
		return t.definition.Description
	}
	return t.Tool.Description()
}

func (t configuredTool) InputSchema() []byte {
	if len(t.definition.Schema) > 0 {
		return t.definition.Schema
	}
	return t.Tool.InputSchema()
}

// Compile-time assertion that embedding preserves the execution implementation.
var _ interface {
	Execute(context.Context, []byte) (string, error)
} = configuredTool{}

func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, exists := r.tools[name]
	if !exists {
		return nil, ErrToolNotFound
	}

	return t, nil
}

func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return ErrToolNotFound
	}

	delete(r.tools, name)

	return nil
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))

	for _, t := range r.tools {
		tools = append(tools, t)
	}

	return tools
}
