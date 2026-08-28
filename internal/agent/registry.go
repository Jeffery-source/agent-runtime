package agent

import (
	"errors"
	"sync"
)

var (
	ErrAgentNotFound = errors.New("agent not found")
	ErrAgentExists   = errors.New("agent already exists")
)

type Registry struct {
	mu     sync.RWMutex
	agents map[string]*Agent
}

func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*Agent),
	}
}

func (r *Registry) Register(agent *Agent) error {
	if agent == nil {
		return errors.New("agent is nil")
	}

	if agent.ID == "" {
		return errors.New("agent ID is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agent.ID]; exists {
		return ErrAgentExists
	}

	r.agents[agent.ID] = agent

	return nil
}

func (r *Registry) Get(id string) (*Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[id]
	if !exists {
		return nil, ErrAgentNotFound
	}

	return agent, nil
}

func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[id]; !exists {
		return ErrAgentNotFound
	}

	delete(r.agents, id)

	return nil
}

func (r *Registry) List() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*Agent, 0, len(r.agents))

	for _, agent := range r.agents {
		agents = append(agents, agent)
	}

	return agents
}
