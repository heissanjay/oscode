package agent

import (
	"github.com/heissanjay/oscode/internal/llm"
	"github.com/heissanjay/oscode/internal/tools"
)

// Type represents the type of agent
type Type string

const (
	// TypeExplore is a fast agent for codebase exploration
	TypeExplore Type = "Explore"
	// TypePlan is an agent for designing implementation plans
	TypePlan Type = "Plan"
	// TypeGeneral is a general-purpose agent with all capabilities
	TypeGeneral Type = "general-purpose"
)

// Config defines configuration for an agent type
type Config struct {
	Type         Type
	Model        string // Model to use (empty = inherit from parent)
	MaxTokens    int
	AllowedTools []string // Tool names to include (nil = all)
	ReadOnly     bool     // Whether agent can only read, not write
	SystemPrompt string   // Custom system prompt for this agent type
}

// DefaultConfigs returns the default configurations for each agent type
var DefaultConfigs = map[Type]Config{
	TypeExplore: {
		Type:         TypeExplore,
		Model:        "claude-haiku-3-5-20241022", // Fast model for exploration
		MaxTokens:    4096,
		AllowedTools: []string{"Read", "Glob", "Grep", "CodeSearch", "LSP"},
		ReadOnly:     true,
		SystemPrompt: `You are a fast exploration agent. Your job is to quickly find and analyze code.

Use these tools efficiently:
- Glob: Find files by pattern
- Grep: Search file contents
- Read: Read file contents
- CodeSearch: Find symbols and definitions
- LSP: Get code intelligence

Be thorough but concise. Return specific file paths and relevant code snippets.
Focus on answering the question directly.`,
	},
	TypePlan: {
		Type:         TypePlan,
		Model:        "claude-sonnet-4-20250514",
		MaxTokens:    8192,
		AllowedTools: []string{"Read", "Glob", "Grep", "CodeSearch", "LSP", "TodoWrite"},
		ReadOnly:     true,
		SystemPrompt: `You are a software architect agent. Your job is to design implementation plans.

Analyze the codebase and create detailed implementation plans including:
- Step-by-step implementation approach
- Files that need to be modified or created
- Key architectural decisions
- Potential risks or trade-offs

Use available tools to explore the codebase before planning.
Be specific about file paths and code locations.`,
	},
	TypeGeneral: {
		Type:         TypeGeneral,
		Model:        "", // Inherit from parent
		MaxTokens:    16384,
		AllowedTools: nil, // All tools
		ReadOnly:     false,
		SystemPrompt: "", // Use parent's system prompt
	},
}

// Agent represents a running agent instance
type Agent struct {
	ID            string
	Type          Type
	Config        Config
	Provider      llm.Provider
	ToolRegistry  *tools.Registry
	Conversation  *llm.Conversation
	SystemPrompt  string
	WorkDir       string
	ParentContext string // Context passed from parent agent
}

// NewAgent creates a new agent instance
func NewAgent(id string, agentType Type, provider llm.Provider, registry *tools.Registry, workDir string) *Agent {
	config := DefaultConfigs[agentType]
	if config.Type == "" {
		config = DefaultConfigs[TypeGeneral]
	}

	return &Agent{
		ID:           id,
		Type:         agentType,
		Config:       config,
		Provider:     provider,
		ToolRegistry: registry,
		Conversation: llm.NewConversation(),
		WorkDir:      workDir,
	}
}

// GetFilteredTools returns tools filtered by agent config
func (a *Agent) GetFilteredTools() []llm.Tool {
	if a.Config.AllowedTools == nil {
		return a.ToolRegistry.ToLLMTools()
	}

	allowedSet := make(map[string]bool)
	for _, name := range a.Config.AllowedTools {
		allowedSet[name] = true
	}

	var filtered []llm.Tool
	for _, tool := range a.ToolRegistry.ToLLMTools() {
		if allowedSet[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// GetModel returns the model to use for this agent
func (a *Agent) GetModel(defaultModel string) string {
	if a.Config.Model != "" {
		return a.Config.Model
	}
	return defaultModel
}

// GetSystemPrompt returns the system prompt for this agent
func (a *Agent) GetSystemPrompt(defaultPrompt string) string {
	if a.Config.SystemPrompt != "" {
		prompt := a.Config.SystemPrompt
		if a.ParentContext != "" {
			prompt += "\n\n## Context from parent:\n" + a.ParentContext
		}
		prompt += "\n\nWorking directory: " + a.WorkDir
		return prompt
	}
	return defaultPrompt
}

// CanExecuteTool checks if this agent can execute the given tool
func (a *Agent) CanExecuteTool(toolName string) bool {
	if a.Config.ReadOnly {
		// Read-only agents can't execute write tools
		writeTools := map[string]bool{
			"Write":        true,
			"Edit":         true,
			"Bash":         true,
			"NotebookEdit": true,
			"KillShell":    true,
		}
		if writeTools[toolName] {
			return false
		}
	}

	if a.Config.AllowedTools == nil {
		return true
	}

	for _, allowed := range a.Config.AllowedTools {
		if allowed == toolName {
			return true
		}
	}
	return false
}
