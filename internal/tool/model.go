package tool

import (
	"context"
	"encoding/json"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() []byte

	Execute(ctx context.Context, arguments []byte) (string, error)
}

// Definition is the configuration-owned part of a Tool. Its implementation is
// still provided by a Go Tool registered in the composition root.
type Definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
	Type        string          `json:"type"`
	Enabled     bool            `json:"enabled"`
}
