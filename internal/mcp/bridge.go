package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/heissanjay/oscode/internal/tools"
)

// ToolBridge adapts an MCP tool to the internal tools.Tool interface
type ToolBridge struct {
	server *Server
	tool   Tool
}

// NewToolBridge creates a new tool bridge
func NewToolBridge(server *Server, tool Tool) *ToolBridge {
	return &ToolBridge{
		server: server,
		tool:   tool,
	}
}

// Name returns the qualified tool name (server:tool)
func (b *ToolBridge) Name() string {
	return fmt.Sprintf("%s:%s", b.server.Name, b.tool.Name)
}

// Description returns the tool description
func (b *ToolBridge) Description() string {
	return b.tool.Description
}

// InputSchema returns the tool's input schema
func (b *ToolBridge) InputSchema() map[string]interface{} {
	return b.tool.InputSchema
}

// RequiresPermission returns true as MCP tools should require permission
func (b *ToolBridge) RequiresPermission() bool {
	return true
}

// Category returns the tool category
func (b *ToolBridge) Category() tools.Category {
	return tools.CategoryOther
}

// Execute calls the MCP tool
func (b *ToolBridge) Execute(ctx context.Context, input json.RawMessage) (*tools.Result, error) {
	var arguments map[string]interface{}
	if err := json.Unmarshal(input, &arguments); err != nil {
		return tools.NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	result, err := b.server.CallTool(ctx, b.tool.Name, arguments)
	if err != nil {
		return tools.NewErrorResult(err), nil
	}

	// Format result
	var content string
	switch v := result.(type) {
	case string:
		content = v
	case map[string]interface{}:
		// Check for content array (MCP format)
		if contentArr, ok := v["content"].([]interface{}); ok {
			for _, item := range contentArr {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if text, ok := itemMap["text"].(string); ok {
						content += text
					}
				}
			}
		} else {
			// Just marshal the whole thing
			data, _ := json.MarshalIndent(v, "", "  ")
			content = string(data)
		}
	default:
		data, _ := json.Marshal(v)
		content = string(data)
	}

	return tools.NewResult(content), nil
}

// Ensure ToolBridge implements tools.Tool
var _ tools.Tool = (*ToolBridge)(nil)
