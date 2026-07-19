// Package mcp holds helpers shared by the gaggle MCP services for shaping tool
// results. It is deliberately transport-agnostic so intervals and garmin can
// reuse it without depending on each other.
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// JSONResult wraps raw JSON bytes as an MCP tool result, preserving the exact
// payload an upstream API returned.
func JSONResult(body []byte) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
	}
}

// DataResult marshals a value to JSON and returns it as a tool result. Used for
// write-tool previews/confirmations that a server constructs itself.
func DataResult(value any) (*mcp.CallToolResult, any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding result: %w", err)
	}
	return JSONResult(encoded), nil, nil
}
