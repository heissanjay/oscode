package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oscode-cli/oscode/internal/commands"
	"github.com/oscode-cli/oscode/internal/config"
	"github.com/oscode-cli/oscode/internal/llm"
	"github.com/oscode-cli/oscode/internal/permissions"
	"github.com/oscode-cli/oscode/internal/session"
	"github.com/oscode-cli/oscode/internal/tools"
	"github.com/oscode-cli/oscode/internal/ui"
)

// Options contains application startup options
type Options struct {
	InitialPrompt   string
	PrintMode       bool
	OutputFormat    string
	MaxTurns        int
	ContinueSession bool
	ResumeSession   string
	SkipPermissions bool
}

// App is the main application
type App struct {
	config          *config.Config
	options         Options
	provider        llm.Provider
	toolRegistry    *tools.Registry
	sessionManager  *session.Manager
	permManager     *permissions.Manager
	uiModel         ui.Model
	program         *tea.Program

	// State
	workDir         string
	systemPrompt    string
	conversation    *llm.Conversation
	currentSession  *session.Session

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new application instance
func New(cfg *config.Config, opts Options) (*App, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config:         cfg,
		options:        opts,
		workDir:        workDir,
		ctx:            ctx,
		cancel:         cancel,
		conversation:   llm.NewConversation(),
		sessionManager: session.NewManager(),
	}

	// Initialize provider
	if err := app.initProvider(); err != nil {
		cancel()
		return nil, err
	}

	// Initialize tools
	app.initTools()

	// Initialize permissions
	app.initPermissions()

	// Build system prompt
	app.buildSystemPrompt()

	// Handle session resumption
	if opts.ContinueSession {
		if err := app.resumeLatestSession(); err != nil {
			// Not fatal, just start fresh
			fmt.Fprintf(os.Stderr, "Warning: could not resume session: %v\n", err)
		}
	} else if opts.ResumeSession != "" {
		if err := app.resumeSession(opts.ResumeSession); err != nil {
			cancel()
			return nil, fmt.Errorf("failed to resume session: %w", err)
		}
	}

	return app, nil
}

func (a *App) initProvider() error {
	providerName := a.config.DefaultProvider

	apiKey := a.config.GetAPIKey(providerName)
	if apiKey == "" {
		return fmt.Errorf("no API key configured for %s. Set %s_API_KEY environment variable",
			providerName, strings.ToUpper(providerName))
	}

	baseURL := a.config.GetBaseURL(providerName)

	switch providerName {
	case "anthropic":
		a.provider = llm.NewAnthropicProvider(apiKey, baseURL)
	case "openai":
		a.provider = llm.NewOpenAIProvider(apiKey, baseURL)
	default:
		return fmt.Errorf("unknown provider: %s", providerName)
	}

	return nil
}

func (a *App) initTools() {
	a.toolRegistry = tools.NewRegistry()

	// Register file tools
	readTool := tools.NewReadTool(a.workDir)
	writeTool := tools.NewWriteTool(a.workDir)
	editTool := tools.NewEditTool(a.workDir)

	a.toolRegistry.Register(readTool)
	a.toolRegistry.Register(writeTool)
	a.toolRegistry.Register(editTool)

	// Register bash tool
	bashTool := tools.NewBashTool(a.workDir)
	a.toolRegistry.Register(bashTool)
	a.toolRegistry.Register(tools.NewBashOutputTool(bashTool))

	// Register search tools
	a.toolRegistry.Register(tools.NewGlobTool(a.workDir))
	a.toolRegistry.Register(tools.NewGrepTool(a.workDir))

	// Register agent tools
	todoTool := tools.NewTodoWriteTool(func(todos []tools.TodoItem) {
		// Update UI with todos
	})
	a.toolRegistry.Register(todoTool)

	// Set tool callbacks for UI updates
	a.toolRegistry.SetCallbacks(
		func(name, desc string) {
			if a.program != nil {
				a.program.Send(ui.ToolStartMsg{ToolName: name, Description: desc})
			}
		},
		func(name, result string, isError bool) {
			if a.program != nil {
				a.program.Send(ui.ToolDoneMsg{ToolName: name, Result: result, IsError: isError})
			}
		},
	)
}

func (a *App) initPermissions() {
	a.permManager = permissions.NewManager(a.config)

	if a.options.SkipPermissions {
		a.permManager.SetSkipPermissions(true)
	}

	// Set permission callback for UI prompts
	// For now, auto-allow all permissions to avoid blocking
	// TODO: Implement proper async permission flow with UI
	a.permManager.SetCallback(func(tool string, input map[string]interface{}, description string) (bool, error) {
		// Auto-allow for now - proper permission UI requires async handling
		return true, nil
	})

	a.toolRegistry.SetPermissionChecker(a.permManager)
}

func (a *App) buildSystemPrompt() {
	if a.config.SystemPrompt != "" {
		a.systemPrompt = a.config.SystemPrompt
		return
	}

	// Build default system prompt
	var sb strings.Builder

	sb.WriteString("You are OSCode, an AI-powered coding assistant running in a CLI environment.\n\n")
	sb.WriteString("You help users with software engineering tasks including:\n")
	sb.WriteString("- Writing, editing, and reviewing code\n")
	sb.WriteString("- Debugging and fixing issues\n")
	sb.WriteString("- Explaining code and concepts\n")
	sb.WriteString("- Running commands and tests\n")
	sb.WriteString("- Managing files and projects\n\n")

	sb.WriteString("Guidelines:\n")
	sb.WriteString("- Be concise and direct in your responses\n")
	sb.WriteString("- Read files before modifying them\n")
	sb.WriteString("- Use the available tools to complete tasks\n")
	sb.WriteString("- Explain what you're doing when performing complex operations\n")
	sb.WriteString("- Ask clarifying questions when requirements are unclear\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n", a.workDir))

	// Load CLAUDE.md if exists
	if memory := a.loadMemory(); memory != "" {
		sb.WriteString("\n--- Project Memory (CLAUDE.md) ---\n")
		sb.WriteString(memory)
		sb.WriteString("\n--- End Project Memory ---\n")
	}

	a.systemPrompt = sb.String()
}

func (a *App) loadMemory() string {
	// Try project memory first
	paths := []string{
		config.GetProjectMemoryPath(a.workDir),
		config.GetProjectLocalMemoryPath(a.workDir),
		config.GetUserMemoryPath(),
	}

	var memories []string
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err == nil && len(content) > 0 {
			memories = append(memories, string(content))
		}
	}

	return strings.Join(memories, "\n\n")
}

func (a *App) resumeLatestSession() error {
	sess, err := a.sessionManager.LoadLatest()
	if err != nil {
		return err
	}

	a.currentSession = sess
	a.conversation.Messages = sess.Messages
	a.sessionManager.SetCurrent(sess)
	return nil
}

func (a *App) resumeSession(id string) error {
	sess, err := a.sessionManager.Load(id)
	if err != nil {
		// Try by name
		sess, err = a.sessionManager.LoadByName(id)
		if err != nil {
			return err
		}
	}

	a.currentSession = sess
	a.conversation.Messages = sess.Messages
	a.sessionManager.SetCurrent(sess)
	return nil
}

// Run starts the application
func (a *App) Run() error {
	// Print mode (non-interactive)
	if a.options.PrintMode {
		return a.runPrintMode()
	}

	// Interactive mode
	return a.runInteractive()
}

func (a *App) runPrintMode() error {
	if a.options.InitialPrompt == "" {
		return fmt.Errorf("no prompt provided for print mode")
	}

	// Process the prompt
	response, err := a.processMessage(a.options.InitialPrompt)
	if err != nil {
		return err
	}

	// Output based on format
	switch a.options.OutputFormat {
	case "json":
		output := map[string]interface{}{
			"result": response,
		}
		if a.currentSession != nil {
			output["session_id"] = a.currentSession.ID
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
	default:
		fmt.Println(response)
	}

	return nil
}

func (a *App) runInteractive() error {
	// Create or use existing session
	if a.currentSession == nil {
		a.currentSession = a.sessionManager.Create(
			a.workDir,
			a.config.DefaultProvider,
			a.config.GetModel(),
		)
	}

	// Create UI model
	a.uiModel = ui.NewModel()
	a.uiModel.SetProviderInfo(a.config.DefaultProvider, a.config.GetModel())
	a.uiModel.SetSessionID(a.currentSession.ID)
	a.uiModel.SetVerbose(a.config.Verbose)

	// Set up handlers
	a.uiModel.SetHandlers(
		a.handleSubmit,
		a.handlePermission,
		a.handleQuit,
	)

	// Set up model/provider change handlers
	a.uiModel.SetChangeHandlers(
		a.handleModelChange,
		a.handleProviderChange,
	)

	// Create program with proper options for fluid UI
	a.program = tea.NewProgram(
		a.uiModel,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithoutSignalHandler(), // Handle signals ourselves
	)

	// Handle initial prompt
	if a.options.InitialPrompt != "" {
		go func() {
			a.program.Send(ui.StreamTextMsg{Content: ""}) // Trigger processing
		}()
	}

	// Run the program
	_, err := a.program.Run()
	return err
}

func (a *App) handleSubmit(input string) tea.Cmd {
	return func() tea.Msg {
		// Check for slash commands
		if commands.IsCommand(input) {
			ctx := a.createCommandContext()
			err := commands.Execute(ctx, input)
			if err != nil {
				return ui.ErrorMsg{Error: err}
			}
			return ui.StreamDoneMsg{}
		}

		// Process as chat message
		response, err := a.processMessage(input)
		if err != nil {
			return ui.StreamErrorMsg{Error: err}
		}

		return ui.StreamDoneMsg{
			InputTokens:  0, // Would come from response
			OutputTokens: len(response) / 4, // Rough estimate
		}
	}
}

func (a *App) handlePermission(allowed bool) {
	// This would be connected to the permission callback
}

func (a *App) handleModelChange(model string) {
	a.config.DefaultModel = model

	// Re-initialize provider if needed (model might be from different provider)
	if strings.HasPrefix(model, "claude") || strings.HasPrefix(model, "claude-") {
		if a.config.DefaultProvider != "anthropic" {
			a.config.DefaultProvider = "anthropic"
			a.initProvider()
		}
	} else if strings.HasPrefix(model, "gpt") || strings.HasPrefix(model, "o1") {
		if a.config.DefaultProvider != "openai" {
			a.config.DefaultProvider = "openai"
			a.initProvider()
		}
	}
}

func (a *App) handleProviderChange(provider string) {
	a.config.DefaultProvider = provider
	a.initProvider()

	// Set default model for the provider
	switch provider {
	case "anthropic":
		a.config.DefaultModel = "claude-sonnet-4-20250514"
	case "openai":
		a.config.DefaultModel = "gpt-4o"
	}
}

func (a *App) handleQuit() {
	// Save session before quitting
	if a.currentSession != nil {
		a.sessionManager.Save()
	}
	a.cancel()
}

func (a *App) createCommandContext() *commands.Context {
	return &commands.Context{
		Session:      a.currentSession,
		Config:       a.config,
		Provider:     a.provider,
		ToolRegistry: a.toolRegistry,
		Print: func(s string) {
			if a.program != nil {
				a.program.Send(ui.StreamTextMsg{Content: s})
			} else {
				fmt.Print(s)
			}
		},
		PrintError: func(s string) {
			if a.program != nil {
				a.program.Send(ui.ErrorMsg{Error: fmt.Errorf(s)})
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s\n", s)
			}
		},
		Clear: func() {
			if a.program != nil {
				a.program.Send(ui.ClearMsg{})
			}
			a.conversation.Clear()
		},
		SetModel: func(model string) {
			a.config.DefaultModel = model
		},
		SetProvider: func(provider string) {
			a.config.DefaultProvider = provider
			a.initProvider()
		},
		Exit: func() {
			a.handleQuit()
			if a.program != nil {
				a.program.Send(ui.QuitMsg{})
			}
		},
		Reload: func() {
			// Reload configuration
			newCfg, _ := config.Load()
			if newCfg != nil {
				a.config = newCfg
			}
		},
	}
}

func (a *App) processMessage(input string) (string, error) {
	// Add user message
	a.conversation.AddUserMessage(input)

	// Build chat request
	req := &llm.ChatRequest{
		Model:        a.config.GetModel(),
		Messages:     a.conversation.Messages,
		Tools:        a.toolRegistry.ToLLMTools(),
		SystemPrompt: a.systemPrompt,
		MaxTokens:    8192,
	}

	// Stream the response
	events, err := a.provider.Stream(a.ctx, req)
	if err != nil {
		return "", err
	}

	var response strings.Builder
	var pendingToolUses []*llm.ToolUse

	for event := range events {
		switch event.Type {
		case llm.EventTypeText:
			response.WriteString(event.Delta)
			if a.program != nil {
				a.program.Send(ui.StreamTextMsg{Content: event.Delta})
			}

		case llm.EventTypeToolUse:
			pendingToolUses = append(pendingToolUses, event.ToolUse)

		case llm.EventTypeDone:
			// Handle tool uses
			if len(pendingToolUses) > 0 {
				err := a.executeToolUses(pendingToolUses)
				if err != nil {
					return "", err
				}

				// Continue conversation after tool execution
				return a.processMessage("")
			}

			// Update session
			if event.Response != nil && a.currentSession != nil {
				a.currentSession.UpdateTokens(
					event.Response.Usage.InputTokens,
					event.Response.Usage.OutputTokens,
				)
			}

		case llm.EventTypeError:
			return "", event.Error
		}
	}

	// Add assistant response to conversation
	responseText := response.String()
	if responseText != "" {
		a.conversation.AddAssistantMessage(responseText)
	}

	// Save session
	if a.currentSession != nil {
		a.currentSession.Messages = a.conversation.Messages
		a.sessionManager.Save()
	}

	return responseText, nil
}

func (a *App) executeToolUses(toolUses []*llm.ToolUse) error {
	// Add assistant message with tool uses
	msg := llm.Message{Role: llm.RoleAssistant}
	for _, tu := range toolUses {
		msg.AddToolUse(tu)
	}
	a.conversation.AddMessage(msg)

	// Execute each tool and collect results
	resultMsg := llm.Message{Role: llm.RoleUser}

	for _, tu := range toolUses {
		result, err := a.toolRegistry.ExecuteToolUse(a.ctx, tu)
		if err != nil {
			result = &llm.ToolResult{
				ToolUseID: tu.ID,
				Content:   err.Error(),
				IsError:   true,
			}
		}
		resultMsg.AddToolResult(result)
	}

	a.conversation.AddMessage(resultMsg)
	return nil
}

// Close cleans up the application
func (a *App) Close() {
	a.cancel()
	if a.currentSession != nil {
		a.sessionManager.Save()
	}
}
