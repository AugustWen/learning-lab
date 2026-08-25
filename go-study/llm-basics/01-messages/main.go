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

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model          string    `json:"model"`
	Messages       []Message `json:"messages"`
	ResponseFormat any       `json:"response_format,omitempty"`
	Stream         bool      `json:"stream"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			Reasoning string `json:"reasoning"`
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

	reqBody := ChatRequest{
		Model: "deepseek/deepseek-v4-flash-0731",
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello! How are you?"},
			{Role: "assistant", Content: "Hello! I'm doing great, thank you for asking! 😊  I'm here and ready to help you with whatever you need. How's your day going so far?"},
			{Role: "user", Content: "I'm doing well, thank you! I was wondering if you could help me with a question I have about programming."},
			{Role: "assistant", Content: "Of course! I'd be happy to help you with your programming question. What do you need assistance with?"},
			{Role: "user", Content: "I'm trying to understand how to use pointers in Go. Can you explain it to me?"},
		},
		Stream: false,
		ResponseFormat: map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "go_explanation",
				"strict": true,
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"concept": map[string]any{
							"type": "string",
						},
						"example": map[string]any{
							"type": "string",
						},
						"warnings": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
					"required": []string{
						"concept",
						"example",
						"warnings",
					},
					"additionalProperties": false,
				},
			},
		},
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.OpenRouterAPIKey))

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

	var chatResp ChatResponse

	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		panic(err)
	}

	fmt.Println("model:", chatResp.Model)
	fmt.Println("answer:", chatResp.Choices[0].Message.Content)
	fmt.Println("reasoning:", chatResp.Choices[0].Message.Reasoning)

	fmt.Println("prompt tokens:", chatResp.Usage.PromptTokens)
	fmt.Println("completion tokens:", chatResp.Usage.CompletionTokens)
	fmt.Println("total tokens:", chatResp.Usage.TotalTokens)
	fmt.Println("cost:", chatResp.Usage.Cost)

}
