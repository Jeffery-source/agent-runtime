package tool

import (
	"context"
	"testing"
)

type mockTool struct {
	name        string
	description string
	schema      []byte
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) InputSchema() []byte {
	return m.schema
}

func (m *mockTool) Execute(
	ctx context.Context,
	arguments []byte,
) (string, error) {
	return "mock result", nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	registry := NewRegistry()

	testTool := &mockTool{
		name:        "calculator",
		description: "A calculator tool",
		schema:      []byte(`{"type":"object"}`),
	}

	err := registry.Register(testTool)
	if err != nil {
		t.Fatalf("register tool failed: %v", err)
	}

	got, err := registry.Get("calculator")
	if err != nil {
		t.Fatalf("get tool failed: %v", err)
	}

	if got.Name() != "calculator" {
		t.Fatalf(
			"expected tool name calculator, got %s",
			got.Name(),
		)
	}
}

func TestRegistryDuplicateRegister(t *testing.T) {
	registry := NewRegistry()

	testTool := &mockTool{
		name: "calculator",
	}

	if err := registry.Register(testTool); err != nil {
		t.Fatalf(
			"first register failed: %v",
			err,
		)
	}

	err := registry.Register(testTool)

	if err != ErrToolExists {
		t.Fatalf(
			"expected ErrToolExists, got %v",
			err,
		)
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Get("not-exist")

	if err != ErrToolNotFound {
		t.Fatalf(
			"expected ErrToolNotFound, got %v",
			err,
		)
	}
}

func TestRegistryRemove(t *testing.T) {
	registry := NewRegistry()

	testTool := &mockTool{
		name: "calculator",
	}

	if err := registry.Register(testTool); err != nil {
		t.Fatalf(
			"register tool failed: %v",
			err,
		)
	}

	if err := registry.Remove("calculator"); err != nil {
		t.Fatalf(
			"remove tool failed: %v",
			err,
		)
	}

	_, err := registry.Get("calculator")

	if err != ErrToolNotFound {
		t.Fatalf(
			"expected ErrToolNotFound after remove, got %v",
			err,
		)
	}
}

func TestRegistryList(t *testing.T) {
	registry := NewRegistry()

	tools := []*mockTool{
		{
			name: "calculator",
		},
		{
			name: "get_time",
		},
		{
			name: "query_mes",
		},
	}

	for _, testTool := range tools {
		if err := registry.Register(testTool); err != nil {
			t.Fatalf(
				"register tool %q failed: %v",
				testTool.name,
				err,
			)
		}
	}

	got := registry.List()

	if len(got) != 3 {
		t.Fatalf(
			"expected 3 tools, got %d",
			len(got),
		)
	}
}

func TestToolExecute(t *testing.T) {
	registry := NewRegistry()

	testTool := &mockTool{
		name: "calculator",
	}

	if err := registry.Register(testTool); err != nil {
		t.Fatalf(
			"register tool failed: %v",
			err,
		)
	}

	tool, err := registry.Get("calculator")
	if err != nil {
		t.Fatalf(
			"get tool failed: %v",
			err,
		)
	}

	result, err := tool.Execute(
		context.Background(),
		[]byte(`{"expression":"1+1"}`),
	)

	if err != nil {
		t.Fatalf(
			"execute tool failed: %v",
			err,
		)
	}

	if result != "mock result" {
		t.Fatalf(
			"expected mock result, got %s",
			result,
		)
	}
}
