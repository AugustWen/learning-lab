package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func newToolCall(name, arguments string) ToolCall {
	var call ToolCall
	call.ID = "call-1"
	call.Type = "function"
	call.Function.Name = name
	call.Function.Arguments = arguments
	return call
}

func TestToolCall_Execute(t *testing.T) {
	registry := NewToolRegistry()

	tool := RegisteredTool{
		Name:        "TestTool",
		Description: "A test tool",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param1": map[string]any{"type": "string"},
			},
		},
		Handler: dummyHandler,
		Timeout: time.Second,
	}

	err := registry.RegisterTool(tool)
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	args := map[string]any{
		"param1": "value1",
	}
	argsJSON, _ := json.Marshal(args)

	result := executeToolCall(context.Background(), registry, newToolCall("TestTool", string(argsJSON)))

	if !result.OK {
		t.Fatalf("expected result OK to be true, got false")
	}

	if result.Data != "ok" {
		t.Fatalf("expected result data to be 'ok', got %v", result.Data)
	}
}
func TestToolCall_ExecuteWithUnknownTool(t *testing.T) {
	registry := NewToolRegistry()

	call := newToolCall("not_exists", "{}")

	result := executeToolCall(context.Background(), registry, call)

	if result.OK {
		t.Fatalf("expected result OK to be false for unknown tool, got true")
	}

	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Error.Code != ErrUnknownToolFunction {
		t.Errorf(
			"error code = %q, want %q",
			result.Error.Code,
			ErrUnknownToolFunction,
		)
	}
}

func TestToolCall_ExecuteWithHandlerError(t *testing.T) {
	registry := NewToolRegistry()

	tool := RegisteredTool{
		Name:        "ErrorTool",
		Description: "A tool that returns an error",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Handler: func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			return ToolResult{}, errors.New("dependency failed")
		},
		Timeout: time.Second,
	}

	if err := registry.RegisterTool(tool); err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	call := newToolCall("ErrorTool", "{}")

	result := executeToolCall(context.Background(), registry, call)

	if result.OK {
		t.Fatalf("expected result OK to be false for handler error, got true")
	}

	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Error.Code != ErrToolExecution {
		t.Errorf(
			"error code = %q, want %q",
			result.Error.Code,
			ErrToolExecution,
		)
	}
}

func TestToolCall_BusinessFailed(t *testing.T) {
	registry := NewToolRegistry()

	tool := RegisteredTool{
		Name:        "BusinessFailTool",
		Description: "A tool that simulates a business failure",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Handler: func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			return ToolResult{
				OK: false,
				Error: &ToolError{
					Code:    "business_failure",
					Message: "simulated business failure",
				},
			}, nil
		},
		Timeout: time.Second,
	}

	if err := registry.RegisterTool(tool); err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	call := newToolCall("BusinessFailTool", "{}")

	result := executeToolCall(context.Background(), registry, call)

	if result.OK {
		t.Fatalf("expected result OK to be false for business failure, got true")
	}

	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Error.Code != "business_failure" {
		t.Errorf(
			"error code = %q, want %q",
			result.Error.Code,
			"business_failure",
		)
	}
}

func TestToolCall_ExecuteWithInvalidArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      json.RawMessage
		wantError bool
	}{
		{
			name:      "valid object",
			args:      json.RawMessage(`{"city": "singapore"}`),
			wantError: false,
		},
		{
			name:      "empty object",
			args:      json.RawMessage("{}"),
			wantError: true,
		},
		{
			name:      "invalid type",
			args:      json.RawMessage(`{"city":1}`),
			wantError: true,
		},
		{
			name:      "city empty string",
			args:      json.RawMessage(`{"city": ""}`),
			wantError: true,
		},
		{
			name:      "extra args",
			args:      json.RawMessage(`{"city": "singapore", "country": "sg"}`),
			wantError: true,
		},
	}

	weatherTool := weatherToolRegister()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArguments(weatherTool.Parameters, tt.args)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateArguments() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}

}

func TestExecuteToolCall_ArgumentValidation(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantOK   bool
		wantCode string
	}{
		{
			name:   "valid arguments",
			args:   `{"city":"Singapore"}`,
			wantOK: true,
		},
		{
			name:     "missing city",
			args:     `{}`,
			wantOK:   false,
			wantCode: ErrInvalidArgument,
		},
		{
			name:     "wrong city type",
			args:     `{"city":1}`,
			wantOK:   false,
			wantCode: ErrInvalidArgument,
		},
		{
			name:     "empty city",
			args:     `{"city":""}`,
			wantOK:   false,
			wantCode: ErrInvalidArgument,
		},
		{
			name:     "extra property",
			args:     `{"city":"Singapore","country":"SG"}`,
			wantOK:   false,
			wantCode: ErrInvalidArgument,
		},
		{
			name:     "invalid json",
			args:     `{"city":`,
			wantOK:   false,
			wantCode: ErrInvalidArgument,
		},
	}

	registry := NewToolRegistry()

	if err := registry.RegisterTool(weatherToolRegister()); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeToolCall(
				context.Background(),
				registry,
				newToolCall("get_weather", tt.args),
			)

			if result.OK != tt.wantOK {
				t.Fatalf(
					"OK = %v, want %v, result = %+v",
					result.OK,
					tt.wantOK,
					result,
				)
			}

			if tt.wantCode != "" {
				if result.Error == nil {
					t.Fatal("expected error, got nil")
				}

				if result.Error.Code != tt.wantCode {
					t.Errorf(
						"error code = %q, want %q",
						result.Error.Code,
						tt.wantCode,
					)
				}
			}
		})
	}
}
