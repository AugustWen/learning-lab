package tools

import (
	"context"
	"encoding/json"
	"time"
)

const (
	ErrInvalidArgument     = "INVALID_ARGUMENT"
	ErrUnknownToolFunction = "UNKNOWN_TOOL_FUNCTION"
	ErrToolExecution       = "TOOL_EXECUTION_ERROR"
	ErrToolTimeout         = "TOOL_TIMEOUT"
)

type ToolResult struct {
	OK    bool       `json:"ok"`
	Data  any        `json:"data,omitempty"`
	Error *ToolError `json:"error,omitempty"`
}

type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ToolHandler func(ctx context.Context, args json.RawMessage) (ToolResult, error)

type RegisteredTool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Handler     ToolHandler
	Timeout     time.Duration
}
