package tools

import (
	"context"
	"encoding/json"

	"github.com/oscode-cli/oscode/internal/llm"
)

// Tool defines the interface for all tools
type Tool interface {
	// Name returns the tool's name
	Name() string

	// Description returns a description of what the tool does
	Description() string

	// InputSchema returns the JSON schema for the tool's input
	InputSchema() map[string]interface{}

	// Execute runs the tool with the given input
	Execute(ctx context.Context, input json.RawMessage) (*Result, error)

	// RequiresPermission returns whether the tool requires user permission
	RequiresPermission() bool

	// Category returns the tool category
	Category() Category
}

// Category represents a tool category
type Category string

const (
	CategoryFile      Category = "file"
	CategoryExecution Category = "execution"
	CategorySearch    Category = "search"
	CategoryWeb       Category = "web"
	CategoryAgent     Category = "agent"
	CategoryOther     Category = "other"
)

// Result represents the result of a tool execution
type Result struct {
	// Content is the main output content
	Content string `json:"content"`

	// IsError indicates if the result is an error
	IsError bool `json:"is_error"`

	// Metadata contains additional metadata about the execution
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewResult creates a new successful result
func NewResult(content string) *Result {
	return &Result{
		Content: content,
		IsError: false,
	}
}

// NewErrorResult creates a new error result
func NewErrorResult(err error) *Result {
	return &Result{
		Content: err.Error(),
		IsError: true,
	}
}

// NewErrorResultString creates a new error result from a string
func NewErrorResultString(msg string) *Result {
	return &Result{
		Content: msg,
		IsError: true,
	}
}

// WithMetadata adds metadata to the result
func (r *Result) WithMetadata(key string, value interface{}) *Result {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
	return r
}

// ToToolResult converts to an LLM ToolResult
func (r *Result) ToToolResult(toolUseID string) *llm.ToolResult {
	return &llm.ToolResult{
		ToolUseID: toolUseID,
		Content:   r.Content,
		IsError:   r.IsError,
	}
}

// BaseTool provides common functionality for tools
type BaseTool struct {
	name        string
	description string
	schema      map[string]interface{}
	permission  bool
	category    Category
}

// NewBaseTool creates a new base tool
func NewBaseTool(name, description string, schema map[string]interface{}, requiresPermission bool, category Category) BaseTool {
	return BaseTool{
		name:        name,
		description: description,
		schema:      schema,
		permission:  requiresPermission,
		category:    category,
	}
}

func (b BaseTool) Name() string {
	return b.name
}

func (b BaseTool) Description() string {
	return b.description
}

func (b BaseTool) InputSchema() map[string]interface{} {
	return b.schema
}

func (b BaseTool) RequiresPermission() bool {
	return b.permission
}

func (b BaseTool) Category() Category {
	return b.category
}

// ParseInput parses the JSON input into the given struct
func ParseInput[T any](input json.RawMessage) (T, error) {
	var result T
	err := json.Unmarshal(input, &result)
	return result, err
}

// Schema builders for common patterns

// StringProperty creates a string property schema
func StringProperty(description string, required bool) map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": description,
	}
}

// IntProperty creates an integer property schema
func IntProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "integer",
		"description": description,
	}
}

// BoolProperty creates a boolean property schema
func BoolProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "boolean",
		"description": description,
	}
}

// ArrayProperty creates an array property schema
func ArrayProperty(description string, itemType string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": description,
		"items": map[string]interface{}{
			"type": itemType,
		},
	}
}

// BuildSchema creates a tool input schema
func BuildSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}
