package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func dummyHandler(
	ctx context.Context,
	args json.RawMessage,
) (ToolResult, error) {
	return ToolResult{
		OK:   true,
		Data: "ok",
	}, nil
}

func TestToolRegistry_RegisterAndGet(t *testing.T) {
	registry := NewToolRegistry()

	tool := &RegisteredTool{
		Name:        "TestTool",
		Description: "A test tool",
		Parameters: map[string]any{
			"param1": "string",
		},
		Handler: func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			return ToolResult{
				OK: true,
				Data: map[string]any{
					"result": "success",
				},
			}, nil
		},
		Timeout: time.Second,
	}
	if err := registry.RegisterTool(*tool); err != nil {
		t.Fatalf("RegisterTool() unexpected error: %v", err)
	}

	retrievedTool, exists := registry.GetTool(tool.Name)

	if !exists {
		t.Fatalf("expected tool to exist, but it doesn't")
	}

	if retrievedTool.Name != tool.Name {
		t.Fatalf("expected tool name %s, got %s", tool.Name, retrievedTool.Name)
	}
	if retrievedTool.Description != tool.Description {
		t.Fatalf("expected tool description %s, got %s", tool.Description, retrievedTool.Description)
	}
	if len(retrievedTool.Parameters) != len(tool.Parameters) {
		t.Fatalf("expected %d parameters, got %d", len(tool.Parameters), len(retrievedTool.Parameters))
	}
	if retrievedTool.Handler == nil {
		t.Fatalf("expected handler to be set, got nil")
	}
}

func TestToolRegistry_GetNonExistent(t *testing.T) {
	registry := NewToolRegistry()

	_, exists := registry.GetTool("NonExistentTool")
	if exists {
		t.Fatalf("expected tool to not exist, but it does")
	}
}

func TestToolRegistry_RegisterEmptyName(t *testing.T) {
	registry := NewToolRegistry()

	err := registry.RegisterTool(RegisteredTool{
		Name:        "",
		Description: "A test tool",
		Handler:     dummyHandler,
		Timeout:     time.Second,
	})

	if err == nil {
		t.Fatal("RegisterTool() expected error, got nil")
	}
}

func TestToolRegistry_RegisterNilHandler(t *testing.T) {
	registry := NewToolRegistry()

	err := registry.RegisterTool(RegisteredTool{
		Name:        "TestTool",
		Description: "A test tool",
		Handler:     nil,
	})

	if err == nil {
		t.Fatal("RegisterTool() expected error, got nil")
	}
}

func TestToolRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewToolRegistry()

	tool := RegisteredTool{
		Name:        "TestTool",
		Description: "A test tool",
		Handler:     dummyHandler,
		Timeout:     time.Second,
	}

	err := registry.RegisterTool(tool)
	if err != nil {
		t.Fatalf("RegisterTool() unexpected error: %v", err)
	}

	err = registry.RegisterTool(tool)
	if err == nil {
		t.Fatal("RegisterTool() expected error for duplicate tool, got nil")
	}
}

func TestToolRegistry_Definitions(t *testing.T) {
	registry := NewToolRegistry()

	tool := RegisteredTool{
		Name:        "TestTool",
		Description: "A test tool",
		Parameters: map[string]any{
			"param1": "string",
		},
		Handler: dummyHandler,
		Timeout: time.Second,
	}

	err := registry.RegisterTool(tool)
	if err != nil {
		t.Fatalf("RegisterTool() unexpected error: %v", err)
	}

	definitions := registry.Definitions()
	if len(definitions) != 1 {
		t.Fatalf("expected 1 tool definition, got %d", len(definitions))
	}

	def := definitions[0]
	if def.Type != "function" {
		t.Fatalf("expected tool type 'function', got %s", def.Type)
	}
	if def.Function.Name != tool.Name {
		t.Fatalf("expected function name %s, got %s", tool.Name, def.Function.Name)
	}
	if def.Function.Description != tool.Description {
		t.Fatalf("expected function description %s, got %s", tool.Description, def.Function.Description)
	}
	if len(def.Function.Parameters) != len(tool.Parameters) {
		t.Fatalf("expected %d parameters, got %d", len(tool.Parameters), len(def.Function.Parameters))
	}
}
