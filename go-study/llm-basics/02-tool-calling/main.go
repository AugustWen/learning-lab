package main

import (
	"bytes"
	"encoding/json"
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

	tools := []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_weather",
				Description: "Get the current weather for a city",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{
							"type":        "string",
							"description": "The city name",
						},
					},
					"required": []string{"city"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_date",
				Description: "Get the current date",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
	}
	messages := []Message{
		{
			Role:    "user",
			Content: "What's the weather today in Singapore?",
		},
	}

	answer, err := agentLoop(cfg, messages, tools)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(answer)

}

func getWeather(city string) ToolResult {
	return ToolResult{
		OK: true,
		Data: map[string]any{
			"city":        city,
			"temperature": 30,
			"condition":   "sunny",
		},
	}
}

func agentLoop(cfg config.Config, messages []Message, tools []Tool) (string, error) {
	for i := 0; i < maxLoopIterations; i++ {
		respBody, err := callModel(cfg.OpenRouterAPIKey, messages, tools)
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

		toolMessages, err1 := executeToolCalls(toolCalls)
		if err1 != nil {
			return "", err1
		}
		messages = append(messages, toolMessages...)
	}
	return "", fmt.Errorf("max loop iterations reached without a final answer")
}

func executeToolCalls(toolCalls []ToolCall) ([]Message, error) {
	messages := make([]Message, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		toolResult := executeToolCall(toolCall)

		toolContent, err := json.Marshal(toolResult)
		if err != nil {
			return nil, fmt.Errorf("marshal tool result: %w", err)
		}

		messages = append(messages, Message{
			Role:       "tool",
			Content:    string(toolContent),
			ToolCallID: toolCall.ID,
		})
	}
	return messages, nil
}

func executeToolCall(toolCall ToolCall) ToolResult {
	var toolResult ToolResult

	switch toolCall.Function.Name {

	case "get_weather":
		var args WeatherArgs
		if err := json.Unmarshal(
			[]byte(toolCall.Function.Arguments),
			&args,
		); err != nil {
			log.Printf(
				"parse get_weather arguments failed: %v, args=%q",
				err,
				toolCall.Function.Arguments,
			)
			toolResult = ToolResult{
				OK: false,
				Error: &ToolError{
					Code:    ErrInvalidArgument,
					Message: "arguments must be valid JSON matching the tool schema",
				},
			}
			return toolResult
		}
		if args.City == "" {
			toolResult = ToolResult{
				OK: false,
				Error: &ToolError{
					Code:    ErrInvalidArgument,
					Message: "city is required",
				},
			}
			return toolResult
		}
		toolResult = getWeather(args.City)

	case "get_date":
		toolResult = ToolResult{
			OK: true,
			Data: map[string]string{
				"date": time.Now().Format("2006-01-02"),
			},
		}

	default:
		toolResult = ToolResult{
			OK: false,
			Error: &ToolError{
				Code:    ErrUnknownToolFunction,
				Message: fmt.Sprintf("unknown tool function: %s", toolCall.Function.Name),
			},
		}
	}
	return toolResult
}

func callModel(apiKey string, messages []Message, tools []Tool) (string, error) {
	reqBody := ChatRequest{
		Model:    "deepseek/deepseek-v4-flash-0731",
		Messages: messages,
		Tools:    tools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(body))
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
