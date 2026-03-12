package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "echo-server",
		Version: "1.0.0",
	}, nil)

	server.AddTool(&mcp.Tool{
		Name:        "echo",
		Description: "Echo the input text back",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args json.RawMessage
		args = req.Params.Arguments
		var m map[string]any
		if err := json.Unmarshal(args, &m); err != nil {
			return nil, err
		}
		text, _ := m["text"].(string)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("echo: %s", text)}},
		}, nil
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
