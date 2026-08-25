package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"example.com/mod/llm-basics/internal/config"
)

func getWeather(city string) string {
	return `{"city":"Singapore","temperature":30,"condition":"sunny"}`
}

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

	// tools := []Tool{
	// 	{
	// 		Type: "function",
	// 		Function: ToolFunction{
	// 			Name:        "get_weather",
	// 			Description: "Get the current weather for a city",
	// 			Parameters: map[string]any{
	// 				"type": "object",
	// 				"properties": map[string]any{
	// 					"city": map[string]any{
	// 						"type":        "string",
	// 						"description": "The city name",
	// 					},
	// 				},
	// 				"required": []string{"city"},
	// 			},
	// 		},
	// 	},
	// }
	tools := make([]Tool, 0)
	messages := []Message{
		{
			Role:    "user",
			Content: "What's the weather in Singapore?",
		},
	}

	respBody := callModel(cfg.OpenRouterAPIKey, messages, tools)

	var chatResp ChatResponse

	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		panic(err)
	}

	var args WeatherArgs

	toolCall := chatResp.Choices[0].Message.ToolCalls[0]

	err = json.Unmarshal(
		[]byte(toolCall.Function.Arguments),
		&args,
	)
	if err != nil {
		panic(err)
	}

	var toolResult string

	switch toolCall.Function.Name {
	case "get_weather":
		toolResult = getWeather(args.City)
	default:
		log.Fatalf("Unknown tool function: %s", toolCall.Function.Name)
	}

	messages = append(messages, Message{
		Role:       "tool",
		Content:    toolResult,
		ToolCallID: toolCall.ID,
	})

	respBody = callModel(cfg.OpenRouterAPIKey, messages, tools)

	fmt.Println(string(respBody))
}

func callModel(apiKey string, messages []Message, tools []Tool) []byte {
	reqBody := ChatRequest{
		Model:    "deepseek/deepseek-v4-flash-0731",
		Messages: messages,
		Tools:    tools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatalf("Error marshalling request body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Request failed with status: %s", resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	return respBody
}
