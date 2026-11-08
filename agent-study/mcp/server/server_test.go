package main

import (
	"context"
	"testing"

	agent "example.com/agent-study/tools"
)

func TestArithmeticMCPHandlers(t *testing.T) {
	for _, tt := range []struct {
		tool agent.RegisteredTool
		want float64
	}{
		{addToolRegister(), 5},
		{multiplyToolRegister(), 6},
	} {
		t.Run(tt.tool.Name, func(t *testing.T) {
			result, output, err := mcpHandler(tt.tool)(context.Background(), nil, AddArgs{A: 2, B: 3})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError || !output.OK || output.Data != tt.want {
				t.Fatalf("result = %+v, output = %+v; want %v", result, output, tt.want)
			}
		})
	}
}
