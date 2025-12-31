package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/heissanjay/oscode/internal/llm"
)

// Registry manages tool registration and execution
type Registry struct {
	tools    map[string]Tool
	mu       sync.RWMutex
	executor *Executor
}

// Executor handles tool execution with permissions
type Executor struct {
	registry          *Registry
	permissionChecker PermissionChecker
	onToolStart       func(name, description string)
	onToolEnd         func(name, result string, isError bool)
}

// PermissionChecker checks if a tool execution is allowed
type PermissionChecker interface {
	Check(tool string, input map[string]interface{}) (allowed bool, err error)
	RequestPermission(tool string, input map[string]interface{}) (bool, error)
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]Tool),
	}
	r.executor = &Executor{
		registry: r,
	}
	return r
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ListNames returns all registered tool names
func (r *Registry) ListNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ToLLMTools converts registered tools to LLM tool definitions
func (r *Registry) ToLLMTools() []llm.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]llm.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, llm.Tool{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return tools
}

// FilteredTools returns tools filtered by names
func (r *Registry) FilteredTools(names []string) []llm.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	tools := make([]llm.Tool, 0)
	for _, tool := range r.tools {
		if nameSet[tool.Name()] {
			tools = append(tools, llm.Tool{
				Name:        tool.Name(),
				Description: tool.Description(),
				InputSchema: tool.InputSchema(),
			})
		}
	}
	return tools
}

// ExcludedTools returns tools excluding specified names
func (r *Registry) ExcludedTools(excludeNames []string) []llm.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	excludeSet := make(map[string]bool)
	for _, name := range excludeNames {
		excludeSet[name] = true
	}

	tools := make([]llm.Tool, 0)
	for _, tool := range r.tools {
		if !excludeSet[tool.Name()] {
			tools = append(tools, llm.Tool{
				Name:        tool.Name(),
				Description: tool.Description(),
				InputSchema: tool.InputSchema(),
			})
		}
	}
	return tools
}

// SetPermissionChecker sets the permission checker
func (r *Registry) SetPermissionChecker(checker PermissionChecker) {
	r.executor.permissionChecker = checker
}

// SetCallbacks sets the tool execution callbacks
func (r *Registry) SetCallbacks(onStart func(name, description string), onEnd func(name, result string, isError bool)) {
	r.executor.onToolStart = onStart
	r.executor.onToolEnd = onEnd
}

// Execute executes a tool by name
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (*Result, error) {
	return r.executor.Execute(ctx, name, input)
}

// ExecuteToolUse executes a tool use from the LLM
func (r *Registry) ExecuteToolUse(ctx context.Context, toolUse *llm.ToolUse) (*llm.ToolResult, error) {
	inputJSON, err := json.Marshal(toolUse.Input)
	if err != nil {
		return &llm.ToolResult{
			ToolUseID: toolUse.ID,
			Content:   fmt.Sprintf("Failed to marshal input: %v", err),
			IsError:   true,
		}, nil
	}

	result, err := r.Execute(ctx, toolUse.Name, inputJSON)
	if err != nil {
		return &llm.ToolResult{
			ToolUseID: toolUse.ID,
			Content:   err.Error(),
			IsError:   true,
		}, nil
	}

	return result.ToToolResult(toolUse.ID), nil
}

// Execute executes a tool with permission checking
func (e *Executor) Execute(ctx context.Context, name string, input json.RawMessage) (*Result, error) {
	tool, ok := e.registry.Get(name)
	if !ok {
		return NewErrorResultString(fmt.Sprintf("Unknown tool: %s", name)), nil
	}

	// Parse input for permission checking
	var inputMap map[string]interface{}
	if err := json.Unmarshal(input, &inputMap); err != nil {
		inputMap = make(map[string]interface{})
	}

	// Check permission if required
	if tool.RequiresPermission() && e.permissionChecker != nil {
		allowed, err := e.permissionChecker.Check(name, inputMap)
		if err != nil {
			return NewErrorResult(err), nil
		}

		if !allowed {
			// Request permission from user
			granted, err := e.permissionChecker.RequestPermission(name, inputMap)
			if err != nil {
				return NewErrorResult(err), nil
			}
			if !granted {
				return NewErrorResultString("Permission denied by user"), nil
			}
		}
	}

	// Notify start
	if e.onToolStart != nil {
		e.onToolStart(name, tool.Description())
	}

	// Execute tool
	result, err := tool.Execute(ctx, input)
	if err != nil {
		result = NewErrorResult(err)
	}

	// Notify end
	if e.onToolEnd != nil {
		e.onToolEnd(name, result.Content, result.IsError)
	}

	return result, nil
}

// DefaultRegistry is the global tool registry
var DefaultRegistry = NewRegistry()

// Register registers a tool in the default registry
func Register(tool Tool) {
	DefaultRegistry.Register(tool)
}

// Get returns a tool from the default registry
func Get(name string) (Tool, bool) {
	return DefaultRegistry.Get(name)
}

// Execute executes a tool from the default registry
func Execute(ctx context.Context, name string, input json.RawMessage) (*Result, error) {
	return DefaultRegistry.Execute(ctx, name, input)
}
