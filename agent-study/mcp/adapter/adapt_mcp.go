package adaptmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agent "example.com/agent-study/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AdaptMCPTool binds a discovered tool to its live MCP session.
// The caller must keep the session open while the returned handler is in use.
func AdaptMCPTool(session *mcp.ClientSession, tool *mcp.Tool) (agent.RegisteredTool, error) {
	if session == nil || tool == nil {
		return agent.RegisteredTool{}, fmt.Errorf("MCP session and tool are required")
	}
	parameters, ok := tool.InputSchema.(map[string]any)
	if !ok {
		return agent.RegisteredTool{}, fmt.Errorf("tool %s: unsupported InputSchema type %T", tool.Name, tool.InputSchema)
	}
	name := tool.Name
	return agent.RegisteredTool{
		Name:        name,
		Description: tool.Description,
		Parameters:  parameters,
		Timeout:     5 * time.Second,
		Handler: func(ctx context.Context, args json.RawMessage) (agent.ToolResult, error) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
			if err != nil {
				return agent.ToolResult{}, fmt.Errorf("call MCP tool %s: %w", name, err)
			}
			if result.IsError {
				var messages []string
				for _, content := range result.Content {
					if text, ok := content.(*mcp.TextContent); ok {
						messages = append(messages, text.Text)
					}
				}
				message := strings.Join(messages, "\n")
				if message == "" {
					message = "MCP tool " + name + " failed"
				}
				return agent.ToolResult{OK: false, Error: &agent.ToolError{Code: agent.ErrToolExecution, Message: message}}, nil
			}
			if result.StructuredContent != nil {
				return agent.ToolResult{OK: true, Data: result.StructuredContent}, nil
			}
			return agent.ToolResult{OK: true, Data: result.Content}, nil
		},
	}, nil
}
