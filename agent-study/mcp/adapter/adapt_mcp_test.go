package adaptmcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAdaptMCPToolOverSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	type args struct {
		A float64 `json:"a"`
		B float64 `json:"b"`
	}
	type output struct {
		Sum float64 `json:"sum"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "add"}, func(_ context.Context, _ *mcp.CallToolRequest, in args) (*mcp.CallToolResult, output, error) {
		if in.A < 0 {
			return nil, output{}, errors.New("negative a")
		}
		return nil, output{Sum: in.A + in.B}, nil
	})
	ct, st := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	tool, err := AdaptMCPTool(session, listed.Tools[0])
	if err != nil {
		t.Fatal(err)
	}
	result, err := tool.Handler(ctx, json.RawMessage(`{"a":2,"b":3}`))
	if err != nil {
		t.Fatal(err)
	}
	data, ok := result.Data.(map[string]any)
	if !result.OK || !ok || data["sum"] != float64(5) {
		t.Fatalf("result = %+v", result)
	}
	result, err = tool.Handler(ctx, json.RawMessage(`{"a":-1,"b":3}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Error == nil || !strings.Contains(result.Error.Message, "negative a") {
		t.Fatalf("error result = %+v", result)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if _, err := tool.Handler(cancelled, json.RawMessage(`{"a":2,"b":3}`)); err == nil {
		t.Fatal("expected cancellation error")
	}
	if _, err := AdaptMCPTool(session, &mcp.Tool{Name: "bad", InputSchema: "invalid"}); err == nil {
		t.Fatal("expected schema error")
	}
}
