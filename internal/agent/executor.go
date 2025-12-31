package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/heissanjay/oscode/internal/llm"
	"github.com/heissanjay/oscode/internal/tools"
)

// TaskInput matches the tools.TaskInput structure
type TaskInput struct {
	Description  string `json:"description"`
	Prompt       string `json:"prompt"`
	SubagentType string `json:"subagent_type"`
	Model        string `json:"model,omitempty"`
	Background   bool   `json:"run_in_background,omitempty"`
	Resume       string `json:"resume,omitempty"` // Agent ID to resume
}

// TaskResult matches the tools.TaskResult structure
type TaskResult struct {
	AgentID string `json:"agent_id"`
	Result  string `json:"result"`
	Status  string `json:"status"`
}

// Executor manages agent execution
type Executor struct {
	providers    map[string]llm.Provider
	toolRegistry *tools.Registry
	workDir      string
	defaultModel string

	// Active agents for resumption
	activeAgents map[string]*Agent
	mu           sync.RWMutex
}

// NewExecutor creates a new agent executor
func NewExecutor(providers map[string]llm.Provider, registry *tools.Registry, workDir, defaultModel string) *Executor {
	return &Executor{
		providers:    providers,
		toolRegistry: registry,
		workDir:      workDir,
		defaultModel: defaultModel,
		activeAgents: make(map[string]*Agent),
	}
}

// Execute runs an agent task
func (e *Executor) Execute(ctx context.Context, input TaskInput) (*TaskResult, error) {
	// Handle resume
	if input.Resume != "" {
		return e.resumeAgent(ctx, input.Resume, input.Prompt)
	}

	// Create new agent
	agentType := Type(input.SubagentType)
	if agentType == "" {
		agentType = TypeGeneral
	}

	agentID := uuid.New().String()[:8]

	// Get provider for the agent
	provider := e.getProvider(agentType, input.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider available for agent")
	}

	agent := NewAgent(agentID, agentType, provider, e.toolRegistry, e.workDir)
	agent.ParentContext = input.Prompt

	// Store for potential resume
	e.mu.Lock()
	e.activeAgents[agentID] = agent
	e.mu.Unlock()

	// Handle background execution
	if input.Background {
		go e.runAgent(context.Background(), agent, input.Prompt)
		return &TaskResult{
			AgentID: agentID,
			Status:  "running",
			Result:  fmt.Sprintf("Agent %s started in background", agentID),
		}, nil
	}

	// Run synchronously
	return e.runAgent(ctx, agent, input.Prompt)
}

func (e *Executor) getProvider(agentType Type, requestedModel string) llm.Provider {
	// Determine which provider to use based on model
	model := requestedModel
	if model == "" {
		config := DefaultConfigs[agentType]
		model = config.Model
	}
	if model == "" {
		model = e.defaultModel
	}

	// Route to appropriate provider based on model name
	if strings.HasPrefix(model, "claude") {
		if p, ok := e.providers["anthropic"]; ok {
			return p
		}
	}
	if strings.HasPrefix(model, "gpt") || strings.HasPrefix(model, "o1") {
		if p, ok := e.providers["openai"]; ok {
			return p
		}
	}

	// Return any available provider
	for _, p := range e.providers {
		return p
	}
	return nil
}

func (e *Executor) runAgent(ctx context.Context, agent *Agent, prompt string) (*TaskResult, error) {
	// Add timeout for agent execution
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Add initial user message
	agent.Conversation.AddUserMessage(prompt)

	// Get model
	model := agent.GetModel(e.defaultModel)

	// Get system prompt
	systemPrompt := agent.GetSystemPrompt("")

	// Process message loop (max iterations to prevent infinite loops)
	const maxIterations = 50
	var response strings.Builder

	for i := 0; i < maxIterations; i++ {
		// Build request
		req := &llm.ChatRequest{
			Model:        model,
			Messages:     agent.Conversation.Messages,
			Tools:        agent.GetFilteredTools(),
			SystemPrompt: systemPrompt,
			MaxTokens:    agent.Config.MaxTokens,
		}

		// Stream response
		events, err := agent.Provider.Stream(ctx, req)
		if err != nil {
			return &TaskResult{
				AgentID: agent.ID,
				Status:  "error",
				Result:  err.Error(),
			}, nil
		}

		var textResponse strings.Builder
		var pendingToolUses []*llm.ToolUse

		for event := range events {
			switch event.Type {
			case llm.EventTypeText:
				textResponse.WriteString(event.Delta)
				response.WriteString(event.Delta)

			case llm.EventTypeToolUse:
				pendingToolUses = append(pendingToolUses, event.ToolUse)

			case llm.EventTypeDone:
				// Add assistant message
				if textResponse.Len() > 0 {
					agent.Conversation.AddAssistantMessage(textResponse.String())
				}

				// Handle tool uses
				if len(pendingToolUses) > 0 {
					err := e.executeToolUses(ctx, agent, pendingToolUses)
					if err != nil {
						return &TaskResult{
							AgentID: agent.ID,
							Status:  "error",
							Result:  err.Error(),
						}, nil
					}
					// Continue loop to process tool results
					continue
				}

				// No more tool uses, agent is done
				return &TaskResult{
					AgentID: agent.ID,
					Status:  "completed",
					Result:  response.String(),
				}, nil

			case llm.EventTypeError:
				return &TaskResult{
					AgentID: agent.ID,
					Status:  "error",
					Result:  event.Error.Error(),
				}, nil
			}
		}
	}

	return &TaskResult{
		AgentID: agent.ID,
		Status:  "completed",
		Result:  response.String() + "\n\n(Agent reached maximum iterations)",
	}, nil
}

func (e *Executor) executeToolUses(ctx context.Context, agent *Agent, toolUses []*llm.ToolUse) error {
	// Add assistant message with tool uses
	msg := llm.Message{Role: llm.RoleAssistant}
	for _, tu := range toolUses {
		msg.AddToolUse(tu)
	}
	agent.Conversation.AddMessage(msg)

	// Execute each tool
	resultMsg := llm.Message{Role: llm.RoleUser}

	for _, tu := range toolUses {
		// Check if agent can execute this tool
		if !agent.CanExecuteTool(tu.Name) {
			result := &llm.ToolResult{
				ToolUseID: tu.ID,
				Content:   fmt.Sprintf("Tool '%s' is not available for this agent type", tu.Name),
				IsError:   true,
			}
			resultMsg.AddToolResult(result)
			continue
		}

		// Execute tool
		result, err := e.toolRegistry.ExecuteToolUse(ctx, tu)
		if err != nil {
			result = &llm.ToolResult{
				ToolUseID: tu.ID,
				Content:   err.Error(),
				IsError:   true,
			}
		}
		resultMsg.AddToolResult(result)
	}

	agent.Conversation.AddMessage(resultMsg)
	return nil
}

func (e *Executor) resumeAgent(ctx context.Context, agentID, prompt string) (*TaskResult, error) {
	e.mu.RLock()
	agent, ok := e.activeAgents[agentID]
	e.mu.RUnlock()

	if !ok {
		return &TaskResult{
			AgentID: agentID,
			Status:  "error",
			Result:  fmt.Sprintf("Agent %s not found", agentID),
		}, nil
	}

	// Add follow-up prompt
	if prompt != "" {
		agent.Conversation.AddUserMessage(prompt)
	}

	return e.runAgent(ctx, agent, "")
}

// GetAgent returns an active agent by ID
func (e *Executor) GetAgent(agentID string) (*Agent, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	agent, ok := e.activeAgents[agentID]
	return agent, ok
}

// CleanupOldAgents removes agents older than the given duration
func (e *Executor) CleanupOldAgents(maxAge time.Duration) {
	// In a full implementation, we'd track creation time
	// For now, just limit total number of agents
	e.mu.Lock()
	defer e.mu.Unlock()

	const maxAgents = 100
	if len(e.activeAgents) > maxAgents {
		// Remove oldest (first added, roughly)
		count := 0
		for id := range e.activeAgents {
			if count >= maxAgents/2 {
				delete(e.activeAgents, id)
			}
			count++
		}
	}
}
