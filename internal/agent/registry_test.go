package agent

import (
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	registry := NewRegistry()

	testAgent := &Agent{
		ID:            "attendance",
		Name:          "Attendance Assistant",
		Model:         "qwen-plus",
		SystemPrompt:  "You are an attendance assistant.",
		MaxIterations: 10,
	}

	err := registry.Register(testAgent)
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}

	got, err := registry.Get("attendance")
	if err != nil {
		t.Fatalf("get agent failed: %v", err)
	}

	if got.ID != testAgent.ID {
		t.Fatalf("expected agent ID %q, got %q", testAgent.ID, got.ID)
	}

	if got.Name != testAgent.Name {
		t.Fatalf("expected agent name %q, got %q", testAgent.Name, got.Name)
	}
}

func TestRegistryDuplicateRegister(t *testing.T) {
	registry := NewRegistry()

	testAgent := &Agent{
		ID:   "attendance",
		Name: "Attendance Assistant",
	}

	if err := registry.Register(testAgent); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	err := registry.Register(testAgent)
	if err != ErrAgentExists {
		t.Fatalf("expected ErrAgentExists, got %v", err)
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Get("not-exist")

	if err != ErrAgentNotFound {
		t.Fatalf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestRegistryRemove(t *testing.T) {
	registry := NewRegistry()

	testAgent := &Agent{
		ID:   "attendance",
		Name: "Attendance Assistant",
	}

	if err := registry.Register(testAgent); err != nil {
		t.Fatalf("register agent failed: %v", err)
	}

	if err := registry.Remove("attendance"); err != nil {
		t.Fatalf("remove agent failed: %v", err)
	}

	_, err := registry.Get("attendance")
	if err != ErrAgentNotFound {
		t.Fatalf("expected ErrAgentNotFound after remove, got %v", err)
	}
}

func TestRegistryList(t *testing.T) {
	registry := NewRegistry()

	agents := []*Agent{
		{
			ID:   "attendance",
			Name: "Attendance Assistant",
		},
		{
			ID:   "mes",
			Name: "MES Assistant",
		},
		{
			ID:   "general",
			Name: "General Assistant",
		},
	}

	for _, testAgent := range agents {
		if err := registry.Register(testAgent); err != nil {
			t.Fatalf("register agent %q failed: %v", testAgent.ID, err)
		}
	}

	got := registry.List()

	if len(got) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(got))
	}
}
