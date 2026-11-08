package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	agent "agent-study/agent-loop"
)

type Input struct {
	Name string `json:"name"`
}

type Output struct {
	Greeting string `json:"greeting"`
}

type AddArgs struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

func addToolRegister() agent.RegisteredTool {
	return agent.RegisteredTool{
		Name:        "add",
		Description: "Add two numbers",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{
					"type":        "number",
					"description": "The first number",
				},
				"b": map[string]any{
					"type":        "number",
					"description": "The second number",
				},
			},
			"required":             []string{"a", "b"},
			"additionalProperties": false,
		},
		Handler: func(ctx context.Context, args json.RawMessage) (agent.ToolResult, error) {
			var addArgs AddArgs
			if err := json.Unmarshal(args, &addArgs); err != nil {
				return agent.ToolResult{
					OK: false,
					Error: &agent.ToolError{
						Code:    agent.ErrInvalidArgument,
						Message: fmt.Sprintf("invalid arguments: %v", err),
					},
				}, nil
			}
			return agent.ToolResult{OK: true, Value: addArgs.A + addArgs.B}, nil
		},
		Timeout: 5 * time.Second,
	}
}

func multiplyToolRegister() agent.RegisteredTool {
	return agent.RegisteredTool{
		Name:        "multiply",
		Description: "Multiply two numbers",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{
					"type":        "number",
					"description": "The first number",
				},
				"b": map[string]any{
					"type":        "number",
					"description": "The second number",
				},
			},
			"required":             []string{"a", "b"},
			"additionalProperties": false,
		},
		Handler: func(ctx context.Context, args json.RawMessage) (agent.ToolResult, error) {
			var multiplyArgs AddArgs
			if err := json.Unmarshal(args, &multiplyArgs); err != nil {
				return agent.ToolResult{
					OK: false,
					Error: &agent.ToolError{
						Code:    agent.ErrInvalidArgument,
						Message: fmt.Sprintf("invalid arguments: %v", err),
					},
				}, nil
			}
			return agent.ToolResult{OK: true, Value: multiplyArgs.A * multiplyArgs.B}, nil
		},
		Timeout: 5 * time.Second,
	}
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "greeter", Version: "v1.0.0"}, nil)
	addTool := addToolRegister()
	multiplyTool := multiplyToolRegister()

	// Register the tools with the server
	mcp.AddTool(server, &mcp.Tool{Name: addTool.Name, Description: addTool.Description}, addTool.Handler)
	mcp.AddTool(server, &mcp.Tool{Name: multiplyTool.Name, Description: multiplyTool.Description}, multiplyTool.Handler)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
