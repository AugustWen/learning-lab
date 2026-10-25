package main

import (
	"context"
	"log"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

	// Call a tool on the server
	param := &mcp.CallToolParams{
		Name: "greet",
		Arguments: map[string]any{
			"name": "you",
		},
	}
	res, err := session.CallTool(ctx, param)
	if err != nil {
		log.Fatal("CallTool error:", err)
	}
	if res.IsError {
		log.Fatal("tool failed:", res.GetError())
	}
	for _, c := range res.Content {
		log.Print(c.(*mcp.TextContent).Text)
	}
}
