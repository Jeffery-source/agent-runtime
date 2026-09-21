package configloader

import "context"

type configTestTool struct{}

func (*configTestTool) Name() string                                    { return "query_attendance" }
func (*configTestTool) Description() string                             { return "Go description" }
func (*configTestTool) InputSchema() []byte                             { return []byte(`{"type":"object"}`) }
func (*configTestTool) Execute(context.Context, []byte) (string, error) { return "ok", nil }
