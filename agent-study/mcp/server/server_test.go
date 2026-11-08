package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"

	agent "example.com/agent-study/tools"
)

func TestArithmeticMCPHandlers(t *testing.T) {
	for _, tt := range []struct {
		tool agent.RegisteredTool
		want float64
	}{
		{addToolRegister(), 5},
		{multiplyToolRegister(), 6},
	} {
		t.Run(tt.tool.Name, func(t *testing.T) {
			result, output, err := mcpHandler(tt.tool)(context.Background(), nil, AddArgs{A: 2, B: 3})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError || !output.OK || output.Data != tt.want {
				t.Fatalf("result = %+v, output = %+v; want %v", result, output, tt.want)
			}
		})
	}
}

// Use the production server registration and exchange actual MCP messages.
func connectMCP(t *testing.T, server *mcp.Server) (context.Context, *mcp.ClientSession) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	ct, st := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "protocol-test", Version: "1.0.0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return ctx, cs
}

func TestMCPListTools(t *testing.T) {
	ctx, client := connectMCP(t, newServer())
	result, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(result.Tools))
	}
	seen := map[string]bool{}
	for _, tool := range result.Tools {
		if tool.Name != "add" && tool.Name != "multiply" {
			t.Fatalf("unexpected tool %q", tool.Name)
		}
		if seen[tool.Name] {
			t.Fatalf("duplicate tool %q", tool.Name)
		}
		seen[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("%s has no description", tool.Name)
		}
		schema, ok := tool.InputSchema.(map[string]any)
		if !ok {
			t.Fatalf("unexpected schema %T", tool.InputSchema)
		}
		if schema["type"] != "object" || schema["additionalProperties"] != false {
			t.Errorf("unexpected schema: %v", schema)
		}
		if !reflect.DeepEqual(schema["required"], []any{"a", "b"}) {
			t.Errorf("required = %v", schema["required"])
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("missing properties: %v", schema)
		}
		for _, name := range []string{"a", "b"} {
			property, ok := properties[name].(map[string]any)
			if !ok || property["type"] != "number" {
				t.Errorf("%s schema = %v", name, properties[name])
			}
		}
	}
}

func decodeToolResult(t *testing.T, result *mcp.CallToolResult) agent.ToolResult {
	t.Helper()
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output agent.ToolResult
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content = %v", result.Content)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T", result.Content[0])
	}
	var textOutput agent.ToolResult
	if err := json.Unmarshal([]byte(text.Text), &textOutput); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(output, textOutput) {
		t.Fatalf("text and structured output differ: %+v / %+v", textOutput, output)
	}
	return output
}

func TestMCPCallArithmetic(t *testing.T) {
	ctx, client := connectMCP(t, newServer())
	for _, tt := range []struct {
		name       string
		a, b, want float64
	}{
		{"add", 2, 3, 5}, {"multiply", 2, 3, 6},
		{"add", -1.5, 0.25, -1.25}, {"multiply", 0, 3, 0},
	} {
		t.Run(tt.name+"/"+fmt.Sprint(tt.a), func(t *testing.T) {
			result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: tt.name, Arguments: map[string]any{"a": tt.a, "b": tt.b}})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError {
				t.Fatalf("tool error: %v", result.Content)
			}
			output := decodeToolResult(t, result)
			if !output.OK || output.Data != tt.want || output.Error != nil {
				t.Fatalf("output = %+v, want %v", output, tt.want)
			}
		})
	}
}

func TestMCPRejectsInvalidArguments(t *testing.T) {
	server := newServer()
	// A spy proves schema rejection happens before the handler runs.
	calls := 0
	tool := addToolRegister()
	handler := tool.Handler
	tool.Handler = func(ctx context.Context, args json.RawMessage) (agent.ToolResult, error) {
		calls++
		return handler(ctx, args)
	}
	mcp.AddTool(server, &mcp.Tool{Name: tool.Name, InputSchema: tool.Parameters}, mcpHandler(tool))
	ctx, client := connectMCP(t, server)
	for _, tt := range []struct {
		name string
		args any
	}{
		{"missing a", map[string]any{"b": 3}},
		{"missing b", map[string]any{"a": 2}},
		{"wrong type", map[string]any{"a": "2", "b": 3}},
		{"null number", map[string]any{"a": nil, "b": 3}},
		{"extra property", map[string]any{"a": 2, "b": 3, "c": 4}},
		{"array", []int{2, 3}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "add", Arguments: tt.args})
			if err != nil {
				t.Fatalf("expected tool error result, got protocol error: %v", err)
			}
			if !result.IsError || errorText(t, result) == "" {
				t.Fatalf("expected error content, got %+v", result)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("invalid input reached handler %d times", calls)
	}
}

func TestMCPUnknownTool(t *testing.T) {
	ctx, client := connectMCP(t, newServer())
	_, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "does-not-exist", Arguments: map[string]any{}})
	if err == nil {
		t.Fatal("expected protocol error for unknown tool")
	}
	// A failed request must not break the session.
	if _, err := client.ListTools(ctx, nil); err != nil {
		t.Fatalf("session unusable after error: %v", err)
	}
}

func TestMCPHandlerErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		output agent.ToolResult
		err    error
	}{
		{"business error", agent.ToolResult{OK: false, Error: &agent.ToolError{Code: agent.ErrToolExecution, Message: "calculation failed"}}, nil},
		{"handler error", agent.ToolResult{}, errors.New("handler failed")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := newServer()
			tool := addToolRegister()
			tool.Handler = func(context.Context, json.RawMessage) (agent.ToolResult, error) { return tt.output, tt.err }
			mcp.AddTool(server, &mcp.Tool{Name: tool.Name, InputSchema: tool.Parameters}, mcpHandler(tool))
			ctx, client := connectMCP(t, server)
			result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "add", Arguments: map[string]any{"a": 2, "b": 3}})
			if err != nil {
				t.Fatalf("expected tool error, got protocol error: %v", err)
			}
			if !result.IsError || errorText(t, result) == "" {
				t.Fatalf("expected error content, got %+v", result)
			}
			if tt.err == nil {
				output := decodeToolResult(t, result)
				if !reflect.DeepEqual(output, tt.output) {
					t.Fatalf("output = %+v, want %+v", output, tt.output)
				}
			} else if !strings.Contains(errorText(t, result), tt.err.Error()) {
				t.Fatalf("missing handler error: %v", result.Content)
			}
		})
	}
}

// GetError is local SDK state; error messages on the wire live in Content.
func errorText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("error content = %v", result.Content)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("error content type = %T", result.Content[0])
	}
	return text.Text
}
