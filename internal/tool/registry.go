package tool

import (
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
