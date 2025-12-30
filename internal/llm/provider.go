package llm

import (
	"context"
	"fmt"
)

// Provider defines the interface for LLM providers
type Provider interface {
	// Name returns the provider name
	Name() string

	// Models returns available models for this provider
	Models() []string

	// Chat sends a chat request and returns the response
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// Stream sends a chat request and returns a stream of events
	Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)

	// SupportsTools returns whether the provider supports tool use
	SupportsTools() bool

	// SupportsVision returns whether the provider supports vision/images
	SupportsVision() bool

	// SupportsStreaming returns whether the provider supports streaming
	SupportsStreaming() bool
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model        string
	Messages     []Message
	Tools        []Tool
	SystemPrompt string
	MaxTokens    int
	Temperature  float64
	TopP         float64
	StopSequences []string
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID           string
	Model        string
	Content      []ContentBlock
	StopReason   string
	Usage        Usage
	ToolUse      []ToolUse
}

// Usage contains token usage information
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// StreamEvent represents a streaming event
type StreamEvent struct {
	Type       StreamEventType
	Delta      string
	ToolUse    *ToolUse
	Response   *ChatResponse
	Error      error
	InputTokens  int
	OutputTokens int
}

// StreamEventType defines the type of stream event
type StreamEventType string

const (
	EventTypeText        StreamEventType = "text"
	EventTypeToolUse     StreamEventType = "tool_use"
	EventTypeToolResult  StreamEventType = "tool_result"
	EventTypeThinking    StreamEventType = "thinking"
	EventTypeDone        StreamEventType = "done"
	EventTypeError       StreamEventType = "error"
	EventTypeUsage       StreamEventType = "usage"
)

// Tool represents a tool definition for the LLM
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolUse represents a tool use request from the LLM
type ToolUse struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
}

// Registry holds all registered providers
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(provider Provider) {
	r.providers[provider.Name()] = provider
}

// Get returns a provider by name
func (r *Registry) Get(name string) (Provider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return provider, nil
}

// List returns all registered provider names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry is the global provider registry
var DefaultRegistry = NewRegistry()

// RegisterProvider registers a provider in the default registry
func RegisterProvider(provider Provider) {
	DefaultRegistry.Register(provider)
}

// GetProvider returns a provider from the default registry
func GetProvider(name string) (Provider, error) {
	return DefaultRegistry.Get(name)
}
