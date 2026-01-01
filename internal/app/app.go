package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heissanjay/oscode/internal/agent"
	"github.com/heissanjay/oscode/internal/commands"
	"github.com/heissanjay/oscode/internal/config"
	"github.com/heissanjay/oscode/internal/hooks"
	"github.com/heissanjay/oscode/internal/llm"
	"github.com/heissanjay/oscode/internal/mcp"
	"github.com/heissanjay/oscode/internal/permissions"
	"github.com/heissanjay/oscode/internal/prompts"
	"github.com/heissanjay/oscode/internal/session"
	"github.com/heissanjay/oscode/internal/tools"
	"github.com/heissanjay/oscode/internal/ui"
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
	config         *config.Config
	options        Options
	provider       llm.Provider
	toolRegistry   *tools.Registry
	sessionManager *session.Manager
	permManager    *permissions.Manager
	hookExecutor   *hooks.Executor
	agentExecutor  *agent.Executor
	mcpClient      *mcp.Client
	uiModel        ui.Model
	program        *tea.Program

	// State
	workDir        string
	systemPrompt   string
	conversation   *llm.Conversation
	currentSession *session.Session
	inPlanMode     bool

	// Permission handling
	permissionChan     chan bool
	permissionResponse chan ui.PermissionResponse
	sessionAllowed     map[string]bool // Tools allowed for this session

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
		config:             cfg,
		options:            opts,
		workDir:            workDir,
		ctx:                ctx,
		cancel:             cancel,
		conversation:       llm.NewConversation(),
		sessionManager:     session.NewManager(),
		permissionResponse: make(chan ui.PermissionResponse, 1),
		sessionAllowed:     make(map[string]bool),
	}

	// Initialize provider
	if err := app.initProvider(); err != nil {
		cancel()
		return nil, err
	}

	// Initialize hooks executor
	app.hookExecutor = hooks.NewExecutor(cfg.Hooks, workDir)

	// Initialize MCP client and connect to configured servers
	app.initMCP()

	// Initialize tools
	app.initTools()

	// Initialize permissions
	app.initPermissions()

	// Initialize agent executor
	app.initAgentExecutor()

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
	a.toolRegistry.Register(tools.NewKillShellTool(bashTool))

	// Register search tools
	a.toolRegistry.Register(tools.NewGlobTool(a.workDir))
	a.toolRegistry.Register(tools.NewGrepTool(a.workDir))
	a.toolRegistry.Register(tools.NewCodeSearchTool(a.workDir))
	a.toolRegistry.Register(tools.NewLSPTool(a.workDir))

	// Register agent tools
	todoTool := tools.NewTodoWriteTool(func(todos []tools.TodoItem) {
		// Update UI with todos
	})
	a.toolRegistry.Register(todoTool)

	// Register notebook tool
	a.toolRegistry.Register(tools.NewNotebookEditTool(a.workDir))

	// Register plan mode tools
	planModeCallback := func(entering bool) {
		// TODO: Implement plan mode state tracking
	}
	a.toolRegistry.Register(tools.NewEnterPlanModeTool(planModeCallback))
	a.toolRegistry.Register(tools.NewExitPlanModeTool(planModeCallback))

	// Register ask user question tool (callback will be wired up via UI)
	a.toolRegistry.Register(tools.NewAskUserQuestionTool(nil))

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
	a.permManager.SetCallback(func(tool string, input map[string]interface{}, description string) (bool, error) {
		// Check if this tool was already allowed for session
		if a.sessionAllowed[tool] {
			return true, nil
		}

		// If no UI program, auto-allow (print mode)
		if a.program == nil {
			return true, nil
		}

		// Build permission request with file info for diff display
		req := &ui.PermissionRequest{
			Tool:        tool,
			Description: description,
		}

		// Extract file info for file operations
		if filePath, ok := input["file_path"].(string); ok {
			req.FilePath = filePath
		}
		if command, ok := input["command"].(string); ok {
			req.Command = command
		}

		// For Edit tool, get old and new content
		if tool == "Edit" {
			if oldStr, ok := input["old_string"].(string); ok {
				req.OldContent = oldStr
				req.IsDiff = true
			}
			if newStr, ok := input["new_string"].(string); ok {
				req.NewContent = newStr
			}
		}

		// For Write tool, show the content being written
		if tool == "Write" {
			if content, ok := input["content"].(string); ok {
				req.NewContent = content
			}
		}

		// Send permission request to UI
		a.program.Send(ui.PermissionRequestMsg{Request: req})

		// Wait for response (blocking)
		resp := <-a.permissionResponse

		// Track "don't ask again" preference
		if resp.DontAskAgain && resp.Allowed {
			a.sessionAllowed[tool] = true
		}

		return resp.Allowed, nil
	})

	a.toolRegistry.SetPermissionChecker(a.permManager)
}

func (a *App) initMCP() {
	a.mcpClient = mcp.NewClient()

	// Connect to configured MCP servers
	for name, serverCfg := range a.config.MCP.Servers {
		if err := a.mcpClient.Connect(name, serverCfg); err != nil {
			// Log warning but continue - MCP servers are optional
			fmt.Fprintf(os.Stderr, "Warning: failed to connect to MCP server %s: %v\n", name, err)
			continue
		}
	}

	// Register MCP tools with the registry
	for _, tool := range a.mcpClient.AllTools() {
		a.toolRegistry.Register(tool)
	}
}

func (a *App) initAgentExecutor() {
	// Create provider map for agent executor
	providers := make(map[string]llm.Provider)
	providers[a.config.DefaultProvider] = a.provider

	a.agentExecutor = agent.NewExecutor(
		providers,
		a.toolRegistry,
		a.workDir,
		a.config.GetModel(),
	)

	// Wire up the Task tool with the agent executor
	taskExecutor := func(ctx context.Context, input tools.TaskInput) (*tools.TaskResult, error) {
		agentInput := agent.TaskInput{
			Description:  input.Description,
			Prompt:       input.Prompt,
			SubagentType: input.SubagentType,
			Model:        input.Model,
			Background:   input.Background,
		}

		result, err := a.agentExecutor.Execute(ctx, agentInput)
		if err != nil {
			return nil, err
		}

		return &tools.TaskResult{
			AgentID: result.AgentID,
			Result:  result.Result,
			Status:  result.Status,
		}, nil
	}

	// Register the Task tool with the executor callback
	a.toolRegistry.Register(tools.NewTaskTool(taskExecutor))
}

func (a *App) buildSystemPrompt() {
	if a.config.SystemPrompt != "" {
		a.systemPrompt = a.config.SystemPrompt
		return
	}

	// Gather dynamic context
	ctx := prompts.GatherContext(a.workDir)

	// Build comprehensive system prompt
	builder := prompts.NewSystemPromptBuilder(a.workDir)

	// Set git info if available
	if ctx.IsGitRepo {
		builder.SetGitInfo(ctx.GitStatus, ctx.GitBranch, ctx.RecentCommits)
	}

	// Load project memory (including OSCODE.md)
	var projectContext []string

	// OSCODE.md takes priority - it's the main project context file
	if ctx.HasOscodeMD {
		projectContext = append(projectContext, ctx.OscodeMD)
	}

	// Also include any memory files
	if memory := a.loadMemory(); memory != "" {
		projectContext = append(projectContext, memory)
	}

	if len(projectContext) > 0 {
		builder.SetProjectMemory(strings.Join(projectContext, "\n\n"))
	}

	// Set available tools
	toolNames := make([]string, 0)
	for _, t := range a.toolRegistry.List() {
		toolNames = append(toolNames, t.Name())
	}
	builder.SetTools(toolNames)

	a.systemPrompt = builder.Build()
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
			InputTokens:  0,                 // Would come from response
			OutputTokens: len(response) / 4, // Rough estimate
		}
	}
}

func (a *App) handlePermission(resp ui.PermissionResponse) {
	// Send response through channel to unblock the permission callback
	// Use blocking send since the permission callback is waiting
	a.permissionResponse <- resp

	// If user provided feedback on rejection, add it to conversation
	if !resp.Allowed && resp.Feedback != "" {
		a.conversation.AddUserMessage("I rejected that change. " + resp.Feedback)
	}
}

func (a *App) handleModelChange(model string) {
	a.config.DefaultModel = model
	providerChanged := false

	// Re-initialize provider if needed (model might be from different provider)
	if strings.HasPrefix(model, "claude") {
		if a.config.DefaultProvider != "anthropic" {
			a.config.DefaultProvider = "anthropic"
			providerChanged = true
		}
	} else if strings.HasPrefix(model, "gpt") || strings.HasPrefix(model, "o1") {
		if a.config.DefaultProvider != "openai" {
			a.config.DefaultProvider = "openai"
			providerChanged = true
		}
	}

	if providerChanged {
		a.initProvider()
	}

	// Update UI
	if a.program != nil {
		a.uiModel.SetProviderInfo(a.config.DefaultProvider, model)
	}
}

func (a *App) handleProviderChange(provider string) {
	a.config.DefaultProvider = provider

	// Set default model for the provider
	switch provider {
	case "anthropic":
		a.config.DefaultModel = "claude-sonnet-4-20250514"
	case "openai":
		a.config.DefaultModel = "gpt-4o"
	}

	// Re-initialize provider
	a.initProvider()

	// Update UI with new model
	if a.program != nil {
		a.uiModel.SetProviderInfo(provider, a.config.DefaultModel)
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
