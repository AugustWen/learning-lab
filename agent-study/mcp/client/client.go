package main

import (
	"context"
	"log"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	agent "example.com/agent-study/tools"
)

func main() {
	ctx := context.Background()

	// Create a new MCP client with no features
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)

	// Connect ti a server over stdin/stdout.
	transport := &mcp.CommandTransport{Command: exec.Command("./bin/server")}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	listToolResults, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatal("ListTools error:", err)
	}
	log.Print("Available tools:")
	RegisteredTools := make(map[string]agent.RegisteredTool)
	for _, tool := range listToolResults.Tools {
		parameters, ok := tool.InputSchema.(map[string]any)
		if !ok {
			log.Fatalf("工具 %s 的 InputSchema 类型不支持：%T",
				tool.Name, tool.InputSchema)
		}
		RegisteredTools[tool.Name] = agent.RegisteredTool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  parameters,
		}
		log.Printf(" - %s: %s", tool.Name, tool.Description)
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "add",
		Arguments: map[string]any{
			"a": 2,
			"b": 3,
		},
	})
	if err != nil {
		log.Fatal("CallTool error:", err)
	}
	if res.IsError {
		log.Fatal("tool failed: ", res.Content)
	}
	for _, c := range res.Content {
		log.Print(c.(*mcp.TextContent).Text)
	}
}
