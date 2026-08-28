package tool

import "context"

type Tool interface {
	Name() string
	Description() string
	InputSchema() []byte

	Execute(ctx context.Context, arguments []byte) (string, error)
}
