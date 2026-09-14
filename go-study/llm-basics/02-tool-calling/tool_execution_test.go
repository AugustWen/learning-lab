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

	result, err := executeToolCall(context.Background(), registry, newToolCall("TestTool", string(argsJSON)))
	if err != nil {
		t.Fatalf("failed to execute tool call: %v", err)
	}

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

	result, err := executeToolCall(context.Background(), registry, call)
	if err != nil {
		t.Fatalf("failed to execute tool call: %v", err)
	}

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

	result, err := executeToolCall(context.Background(), registry, call)
	if err != nil {
		t.Fatalf("failed to execute tool call: %v", err)
	}

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

	result, err := executeToolCall(context.Background(), registry, call)
	if err != nil {
		t.Fatalf("failed to execute tool call: %v", err)
	}

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
			result, err := executeToolCall(
				context.Background(),
				registry,
				newToolCall("get_weather", tt.args),
			)
			if err != nil {
				t.Fatalf("failed to execute tool call: %v", err)
			}

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

func TestExecuteToolCall_Timeout(t *testing.T) {
	registry := NewToolRegistry()

	tool := RegisteredTool{
		Name:        "slow_tool",
		Description: "A tool that takes a long time to complete",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Handler: func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			select {
			case <-time.After(1 * time.Second):
				return ToolResult{
					OK:   true,
					Data: "finished",
				}, nil
			case <-ctx.Done():
				return ToolResult{}, ctx.Err()
			}
		},
		Timeout: 50 * time.Millisecond,
	}
	if err := registry.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool() unexpected error: %v", err)
	}

	result, err := executeToolCall(context.Background(), registry, newToolCall("slow_tool", "{}"))
	if err != nil {
		t.Fatalf("failed to execute tool call: %v", err)
	}

	if result.OK {
		t.Fatalf("expected result OK to be false, got true")
	}

	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Error.Code != ErrToolTimeout {
		t.Errorf(
			"error code = %q, want %q",
			result.Error.Code,
			ErrToolTimeout,
		)
	}

}

func TestExecuteToolCall_ContextCanceled(t *testing.T) {
	registry := NewToolRegistry()

	tool := RegisteredTool{
		Name:        "cancellable_tool",
		Description: "A tool that can be cancelled",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Handler: func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			select {
			case <-time.After(1 * time.Second):
				return ToolResult{
					OK:   true,
					Data: "finished",
				}, nil
			case <-ctx.Done():
				return ToolResult{}, ctx.Err()
			}
		},
		Timeout: 2 * time.Second,
	}
	if err := registry.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool() unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context immediately

	_, err := executeToolCall(ctx, registry, newToolCall("cancellable_tool", "{}"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

}

// timeout不能强制终止
func TestHandler_SleepIgnoresCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()

	time.Sleep(200 * time.Millisecond)

	elapsed := time.Since(start)

	if elapsed < 200*time.Millisecond {
		t.Fatalf("sleep unexpectedly interrupted")
	}

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("expected context canceled")
	}
}

// timeout能强制终止
func TestHandler_ContextAwareWorkStopsEarly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()

	select {
	case <-time.After(time.Second):
		t.Fatal("work should have been canceled")

	case <-ctx.Done():
	}

	elapsed := time.Since(start)

	if elapsed >= time.Second {
		t.Fatalf(
			"cancellation did not stop work early: %v",
			elapsed,
		)
	}

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			ctx.Err(),
		)
	}
}

func TestContext_ParentCancelCancelsChild(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())
	childCtx, childCancel := context.WithTimeout(parentCtx, time.Second)

	defer childCancel() // Ensure the child context is canceled to avoid resource leaks
	parentCancel()      // Cancel the parent context

	select {
	case <-childCtx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("child context did not cancel in time")
	}

	if !errors.Is(childCtx.Err(), context.Canceled) {
		t.Fatalf("expected parent context to be canceled, got: %v", childCtx.Err())
	}
}

func TestContext_ChildCancelCancelsParent(t *testing.T) {
	parentCtx := context.Background()
	childCtx, childCancel := context.WithTimeout(parentCtx, 20*time.Millisecond)

	defer childCancel() // Ensure the child context is canceled to avoid resource leaks

	<-childCtx.Done() // Wait for the child context to be canceled

	if !errors.Is(childCtx.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected parent context to be canceled, got: %v", childCtx.Err())
	}
	if parentCtx.Err() != nil {
		t.Fatalf("expected parent context to be unaffected, got: %v", parentCtx.Err())
	}
}
