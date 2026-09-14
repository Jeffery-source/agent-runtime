package skill

import (
	"fmt"
	"sync"
)

type Registry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
}

func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*Skill),
	}
}

func (r *Registry) Register(s *Skill) error {
	if s == nil {
		return fmt.Errorf("skill is nil")
	}

	if s.ID == "" {
		return fmt.Errorf("skill id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[s.ID]; exists {
		return fmt.Errorf("skill already registered: %s", s.ID)
	}

	r.skills[s.ID] = s

	return nil
}

func (r *Registry) Get(id string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.skills[id]
	return s, ok
}

func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Skill, 0, len(r.skills))

	for _, s := range r.skills {
		result = append(result, s)
	}

	return result
}
