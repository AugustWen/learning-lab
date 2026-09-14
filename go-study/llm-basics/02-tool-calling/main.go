package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"example.com/mod/llm-basics/internal/config"
)

const maxLoopIterations = 5

const (
	ErrInvalidArgument     = "INVALID_ARGUMENT"
	ErrUnknownToolFunction = "UNKNOWN_TOOL_FUNCTION"
	ErrToolExecution       = "TOOL_EXECUTION_ERROR"
	ErrToolTimeout         = "TOOL_TIMEOUT"
)

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

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

type ToolRegistry struct {
	tools map[string]RegisteredTool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]RegisteredTool),
	}
}

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	ToolCallID string `json:"tool_call_id,omitempty"`
}

type WeatherArgs struct {
	City string `json:"city"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role       string     `json:"role"`
			Content    string     `json:"content"`
			Reasoning  string     `json:"reasoning"`
			ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
			ToolCallID string     `json:"tool_call_id,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		TotalTokens      int     `json:"total_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	toolRegistry := NewToolRegistry()
	if err := toolRegistry.RegisterTool(weatherToolRegister()); err != nil {
		log.Fatal(err)
	}
	if err := toolRegistry.RegisterTool(dateToolRegister()); err != nil {
		log.Fatal(err)
	}

	messages := []Message{
		{
			Role:    "user",
			Content: "What's the weather today in Singapore?",
		},
	}

	ctx := context.Background()
	answer, err := agentLoop(ctx, cfg, messages, toolRegistry)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(answer)

}

func weatherToolRegister() RegisteredTool {
	return RegisteredTool{
		Name:        "get_weather",
		Description: "Get the current weather for a city",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"city": map[string]any{
					"type":        "string",
					"description": "The city name",
					"minLength":   1,
				},
			},
			"required":             []string{"city"},
			"additionalProperties": false,
		},
		Handler: func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			var weatherArgs WeatherArgs
			if err := json.Unmarshal(args, &weatherArgs); err != nil {
				return ToolResult{
					OK: false,
					Error: &ToolError{
						Code:    ErrInvalidArgument,
						Message: fmt.Sprintf("invalid arguments: %v", err),
					},
				}, nil
			}
			return getWeather(weatherArgs.City), nil
		},
		Timeout: 5 * time.Second,
	}
}

func dateToolRegister() RegisteredTool {
	return RegisteredTool{
		Name:        "get_date",
		Description: "Get the current date",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			return ToolResult{
				OK: true,
				Data: map[string]any{
					"date": time.Now().Format("2006-01-02"),
				},
			}, nil
		},
		Timeout: 5 * time.Second,
	}
}

func getWeather(city string) ToolResult {
	if city == "" {
		return ToolResult{
			OK: false,
			Error: &ToolError{
				Code:    ErrInvalidArgument,
				Message: "city is required",
			},
		}
	}
	return ToolResult{
		OK: true,
		Data: map[string]any{
			"city":        city,
			"temperature": 30,
			"condition":   "sunny",
		},
	}
}

func agentLoop(ctx context.Context, cfg config.Config, messages []Message, toolRegistry *ToolRegistry) (string, error) {
	for i := 0; i < maxLoopIterations; i++ {
		respBody, err := callModel(ctx, cfg.OpenRouterAPIKey, messages, toolRegistry.Definitions())
		if err != nil {
			return "", fmt.Errorf("call model: %w", err)
		}
		log.Println(respBody)

		var chatResp ChatResponse

		if err := json.Unmarshal([]byte(respBody), &chatResp); err != nil {
			return "", fmt.Errorf("parse model response: %w", err)
		}
		if len(chatResp.Choices) == 0 {
			return "", fmt.Errorf("model response has no choices")
		}

		message := chatResp.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			fmt.Println("model didn't call any tool")
			return message.Content, nil
		}

		messages = append(messages, Message{
			Role:      message.Role,
			Content:   message.Content,
			ToolCalls: message.ToolCalls,
		})

		toolCalls := message.ToolCalls

		toolMessages, err1 := executeToolCalls(ctx, toolRegistry, toolCalls)
		if err1 != nil {
			return "", err1
		}
		messages = append(messages, toolMessages...)
	}
	return "", fmt.Errorf("max loop iterations reached without a final answer")
}

func executeToolCalls(ctx context.Context, registry *ToolRegistry, toolCalls []ToolCall) ([]Message, error) {
	messages := make([]Message, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		toolResult, err := executeToolCall(ctx, registry, toolCall)
		if err != nil {
			return messages, fmt.Errorf("execute tool call %q: %w", toolCall.Function.Name, err)
		}

		toolContent, err := json.Marshal(toolResult)
		if err != nil {
			return messages, fmt.Errorf("marshal tool result for tool call %q: %w", toolCall.Function.Name, err)
		}

		messages = append(messages, Message{
			Role:       "tool",
			Content:    string(toolContent),
			ToolCallID: toolCall.ID,
		})
	}
	return messages, nil
}

func (r *ToolRegistry) RegisterTool(tool RegisteredTool) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	if tool.Handler == nil {
		return fmt.Errorf("tool handler is required")
	}
	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool %q is already registered", tool.Name)
	}
	if tool.Timeout <= 0 {
		return fmt.Errorf("tool timeout must be greater than zero")
	}
	r.tools[tool.Name] = tool
	return nil
}

func (r *ToolRegistry) GetTool(name string) (RegisteredTool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

func (r *ToolRegistry) Definitions() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	return tools
}

func executeToolCall(ctx context.Context, registry *ToolRegistry, toolCall ToolCall) (ToolResult, error) {
	var toolResult ToolResult
	// 查tool
	tool, exists := registry.GetTool(toolCall.Function.Name)
	if !exists {
		toolResult = ToolResult{
			OK: false,
			Error: &ToolError{
				Code:    ErrUnknownToolFunction,
				Message: fmt.Sprintf("unknown tool function: %s", toolCall.Function.Name),
			},
		}
		return toolResult, nil
	}

	rawArgs := json.RawMessage(toolCall.Function.Arguments)

	// 2. Validate the arguments
	if err := validateJSONArguments(rawArgs); err != nil {
		return ToolResult{
			OK: false,
			Error: &ToolError{
				Code:    ErrInvalidArgument,
				Message: fmt.Sprintf("invalid arguments: %v", err),
			},
		}, nil
	}

	// 3.schema
	if err := validateArguments(tool.Parameters, rawArgs); err != nil {
		return ToolResult{
			OK: false,
			Error: &ToolError{
				Code:    ErrInvalidArgument,
				Message: fmt.Sprintf("invalid arguments: %v", err),
			},
		}, nil
	}

	// 4. tool timeout
	toolCtx, cancel := context.WithTimeout(ctx, tool.Timeout)
	defer cancel()

	// 5. Execute the tool handler
	result, err := tool.Handler(toolCtx, rawArgs)
	if err != nil {
		switch {
		case errors.Is(toolCtx.Err(), context.DeadlineExceeded):
			return ToolResult{
				OK: false,
				Error: &ToolError{
					Code:    ErrToolTimeout,
					Message: "tool execution timed out",
				},
			}, nil
		case errors.Is(ctx.Err(), context.Canceled):
			return ToolResult{}, ctx.Err()
		default:
			return ToolResult{
				OK: false,
				Error: &ToolError{
					Code:    ErrToolExecution,
					Message: fmt.Sprintf("tool execution failed: %v", err),
				},
			}, nil
		}
	}

	return result, nil
}

func callModel(ctx context.Context, apiKey string, messages []Message, tools []Tool) (string, error) {
	reqBody := ChatRequest{
		Model:    "deepseek/deepseek-v4-flash-0731",
		Messages: messages,
		Tools:    tools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("model API returned %s: %s",
			resp.Status,
			string(respBody),
		)
	}

	return string(respBody), nil
}

func validateJSONArguments(args json.RawMessage) error {
	if len(args) == 0 {
		return fmt.Errorf("arguments is empty")
	}
	if !json.Valid(args) {
		return fmt.Errorf("arguments is not valid JSON")
	}
	return nil
}

func validateArguments(schema map[string]any, args json.RawMessage) error {
	var argsMap map[string]any
	if err := json.Unmarshal(args, &argsMap); err != nil {
		return fmt.Errorf("unmarshal arguments: %w", err)
	}

	// Check for required properties
	if requiredField, exists := schema["required"]; exists {
		if requiredList, ok := requiredField.([]string); ok {
			for _, reqStr := range requiredList {
				if _, exists := argsMap[reqStr]; !exists {
					return fmt.Errorf("missing required property: %s", reqStr)
				}

			}
		}
	}

	schemaProperties, ok := schema["properties"].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid schema: properties field is missing or not an object")
	}
	// Check for additional properties
	if additionalProperties, exists := schema["additionalProperties"]; exists {
		if !additionalProperties.(bool) {
			for key := range argsMap {
				if _, exists := schemaProperties[key]; !exists {
					return fmt.Errorf("additional property %s is not allowed", key)
				}
			}
		}
	}

	for name, propertySchemaValue := range schemaProperties {
		value, exists := argsMap[name]
		if !exists {
			continue // Skip validation for properties that are not present
		}
		propertySchema, ok := propertySchemaValue.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid schema for property %s: not an object", name)
		}

		if expectedType, ok := propertySchema["type"].(string); ok {
			switch expectedType {
			case "string":
				strValue, ok := value.(string)
				if !ok {
					return fmt.Errorf("property %s is not of type string", name)
				}
				// Check length
				if minLength, exists := propertySchema["minLength"]; exists {
					if minLength, ok := minLength.(int); ok {
						if len(strValue) < minLength {
							return fmt.Errorf("property %s is too short: got %d, want at least %d", name, len(strValue), minLength)
						}
					}
				}
			case "number":
				if _, ok := value.(float64); !ok {
					return fmt.Errorf("property %s is not of type number", name)
				}
			case "boolean":
				if _, ok := value.(bool); !ok {
					return fmt.Errorf("property %s is not of type boolean", name)
				}
			case "object":
				if _, ok := value.(map[string]any); !ok {
					return fmt.Errorf("property %s is not of type object", name)
				}
			case "array":
				if _, ok := value.([]any); !ok {
					return fmt.Errorf("property %s is not of type array", name)
				}
			default:
				return fmt.Errorf("unsupported type %s for property %s", expectedType, name)
			}
		}
	}
	return nil
}
