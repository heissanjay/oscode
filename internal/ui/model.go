package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// State represents the current UI state
type State int

const (
	StateInput State = iota
	StateProcessing
	StatePermissionPrompt
	StateError
	StateQuitting
)

// MessageType represents types of messages in the conversation
type MessageType int

const (
	MessageTypeUser MessageType = iota
	MessageTypeAssistant
	MessageTypeSystem
	MessageTypeTool
	MessageTypeError
)

// DisplayMessage represents a message to display
type DisplayMessage struct {
	Type        MessageType
	Content     string
	Description string // For tools: file path, command, etc.
	Timestamp   time.Time
	ToolName    string
	IsError     bool
}

// PermissionRequest represents a pending permission request
type PermissionRequest struct {
	Tool        string
	Description string
	Command     string
	FilePath    string          // For file operations
	OldContent  string          // For Edit: content being replaced
	NewContent  string          // For Edit/Write: new content
	IsDiff      bool            // Whether this is a diff view
	Callback    func(response PermissionResponse)
}

// PermissionResponse represents the user's response to a permission request
type PermissionResponse struct {
	Allowed      bool
	DontAskAgain bool   // "a" - allow all for this tool
	Feedback     string // If rejected, user can explain why
}

// Model represents the main UI model
type Model struct {
	// UI state
	state  State
	width  int
	height int
	ready  bool

	// Components
	textarea  textarea.Model
	viewport  viewport.Model
	spinner   spinner.Model
	selection *SelectionModel

	// Conversation
	messages     []DisplayMessage
	history      []string
	historyIndex int

	// Current state
	provider  string
	model     string
	tokens    int
	cost      float64
	sessionID string

	// Available options
	availableModels    []SelectionItem
	availableProviders []SelectionItem

	// Permission handling
	permissionRequest  *PermissionRequest
	permissionChoice   int  // 0=yes, 1=yes always, 2=no with feedback
	rejectingWithInput bool // User is typing rejection feedback

	// Streaming state
	streamingContent string
	isStreaming      bool
	currentVerb      string // Dynamic verb for spinner (Thinking, Reading, etc.)

	// Verbose mode
	verbose bool

	// Vim mode
	vimMode   bool
	vimNormal bool

	// Error message
	errorMsg string

	// Command suggestions
	showingSuggestions bool
	suggestions        []SelectionItem
	suggestionCursor   int

	// Event handlers (set by app)
	onSubmit         func(string) tea.Cmd
	onPermission     func(PermissionResponse)
	onQuit           func()
	onModelChange    func(string)
	onProviderChange func(string)
}

// allCommands is the list of all available commands for suggestions
var allCommands = []SelectionItem{
	{ID: "help", Label: "/help", Description: "Show commands"},
	{ID: "model", Label: "/model", Description: "Switch model"},
	{ID: "provider", Label: "/provider", Description: "Switch provider"},
	{ID: "clear", Label: "/clear", Description: "Clear conversation"},
	{ID: "compact", Label: "/compact", Description: "Compact conversation"},
	{ID: "cost", Label: "/cost", Description: "Show token usage"},
	{ID: "vim", Label: "/vim", Description: "Toggle vim mode"},
	{ID: "verbose", Label: "/verbose", Description: "Toggle verbose"},
	{ID: "exit", Label: "/exit", Description: "Exit application"},
}

// updateSuggestions updates the command suggestions based on filter
func (m *Model) updateSuggestions(filter string) {
	filter = strings.ToLower(filter)
	m.suggestions = nil
	m.suggestionCursor = 0

	for _, cmd := range allCommands {
		if strings.HasPrefix(strings.ToLower(cmd.ID), filter) ||
			strings.Contains(strings.ToLower(cmd.Label), filter) {
			m.suggestions = append(m.suggestions, cmd)
		}
	}
}

// NewModel creates a new UI model
func NewModel() Model {
	// Create textarea for input - single line, minimal style
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.SetWidth(80)
	ta.SetHeight(1) // Always single line
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	// Minimal styling - no background, just text
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	ta.Focus()

	// Create OSCode sparkle spinner - the magical thinking animation
	s := spinner.New()
	s.Spinner = OsCodeSpinner()
	s.Style = ToolSpinnerStyle

	// Create viewport for messages
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle()

	return Model{
		state:        StateInput,
		textarea:     ta,
		viewport:     vp,
		spinner:      s,
		selection:    NewSelectionModel(),
		messages:     make([]DisplayMessage, 0),
		history:      make([]string, 0),
		historyIndex: -1,
		width:        80,
		height:       24,
		ready:        true, // Start ready immediately
		availableModels: []SelectionItem{
			{ID: "gpt-4o", Label: "gpt-4o", Description: "OpenAI flagship", Selected: true},
			{ID: "gpt-4o-mini", Label: "gpt-4o-mini", Description: "Fast and affordable"},
			{ID: "o1-preview", Label: "o1-preview", Description: "Reasoning model"},
			{ID: "o1-mini", Label: "o1-mini", Description: "Fast reasoning"},
			{ID: "claude-sonnet-4-20250514", Label: "claude-sonnet-4", Description: "Best for coding"},
			{ID: "claude-opus-4-20250514", Label: "claude-opus-4", Description: "Most capable"},
			{ID: "claude-haiku-3-5-20241022", Label: "claude-haiku-3.5", Description: "Fast Claude"},
		},
		availableProviders: []SelectionItem{
			{ID: "openai", Label: "OpenAI", Description: "GPT models", Selected: true},
			{ID: "anthropic", Label: "Anthropic", Description: "Claude models"},
		},
	}
}

// SetHandlers sets the event handlers
func (m *Model) SetHandlers(onSubmit func(string) tea.Cmd, onPermission func(PermissionResponse), onQuit func()) {
	m.onSubmit = onSubmit
	m.onPermission = onPermission
	m.onQuit = onQuit
}

// SetChangeHandlers sets model/provider change handlers
func (m *Model) SetChangeHandlers(onModelChange func(string), onProviderChange func(string)) {
	m.onModelChange = onModelChange
	m.onProviderChange = onProviderChange
}

// ShowModelSelection shows the model selection menu
func (m *Model) ShowModelSelection() {
	// Update selected state based on current model
	items := make([]SelectionItem, len(m.availableModels))
	for i, item := range m.availableModels {
		items[i] = item
		items[i].Selected = item.ID == m.model
	}
	m.selection.Show(SelectionModelMenu, "Select Model", items)
}

// ShowProviderSelection shows the provider selection menu
func (m *Model) ShowProviderSelection() {
	// Update selected state based on current provider
	items := make([]SelectionItem, len(m.availableProviders))
	for i, item := range m.availableProviders {
		items[i] = item
		items[i].Selected = item.ID == m.provider
	}
	m.selection.Show(SelectionProviderMenu, "Select Provider", items)
}

// ShowHelpMenu shows the help menu
func (m *Model) ShowHelpMenu() {
	items := []SelectionItem{
		{ID: "clear", Label: "/clear", Description: "Clear conversation"},
		{ID: "model", Label: "/model", Description: "Switch model"},
		{ID: "provider", Label: "/provider", Description: "Switch provider"},
		{ID: "compact", Label: "/compact", Description: "Compact conversation"},
		{ID: "cost", Label: "/cost", Description: "Show token usage"},
		{ID: "vim", Label: "/vim", Description: "Toggle vim mode"},
		{ID: "exit", Label: "/exit", Description: "Exit application"},
	}
	m.selection.Show(SelectionHelpMenu, "Commands", items)
}

// IsSelectionActive returns whether a selection menu is active
func (m *Model) IsSelectionActive() bool {
	return m.selection != nil && m.selection.IsActive()
}

// ShowCommandPalette shows the command palette for autocomplete
func (m *Model) ShowCommandPalette(filter string) {
	items := []SelectionItem{
		{ID: "help", Label: "/help", Description: "Show commands"},
		{ID: "model", Label: "/model", Description: "Switch model"},
		{ID: "provider", Label: "/provider", Description: "Switch provider"},
		{ID: "clear", Label: "/clear", Description: "Clear conversation"},
		{ID: "compact", Label: "/compact", Description: "Compact conversation"},
		{ID: "cost", Label: "/cost", Description: "Show token usage"},
		{ID: "vim", Label: "/vim", Description: "Toggle vim mode"},
		{ID: "verbose", Label: "/verbose", Description: "Toggle verbose"},
		{ID: "exit", Label: "/exit", Description: "Exit application"},
	}
	m.selection.Show(SelectionHelpMenu, "Commands", items)
	if filter != "" {
		for _, r := range filter {
			m.selection.AddFilterChar(r)
		}
	}
}

// SetProviderInfo updates the provider and model info
func (m *Model) SetProviderInfo(provider, model string) {
	m.provider = provider
	m.model = model
}

// SetSessionID sets the current session ID
func (m *Model) SetSessionID(id string) {
	m.sessionID = id
}

// SetVerbose sets verbose mode
func (m *Model) SetVerbose(v bool) {
	m.verbose = v
}

// AddMessage adds a message to the conversation
func (m *Model) AddMessage(msg DisplayMessage) {
	m.messages = append(m.messages, msg)
	m.updateViewport()
}

// AddUserMessage adds a user message
func (m *Model) AddUserMessage(content string) {
	m.AddMessage(DisplayMessage{
		Type:      MessageTypeUser,
		Content:   content,
		Timestamp: time.Now(),
	})
}

// AddAssistantMessage adds an assistant message
func (m *Model) AddAssistantMessage(content string) {
	m.AddMessage(DisplayMessage{
		Type:      MessageTypeAssistant,
		Content:   content,
		Timestamp: time.Now(),
	})
}

// AddToolMessage adds a tool execution message
func (m *Model) AddToolMessage(toolName, description string, isError bool) {
	m.AddMessage(DisplayMessage{
		Type:        MessageTypeTool,
		Content:     "",
		Description: description, // File path, command, etc.
		ToolName:    toolName,
		IsError:     isError,
		Timestamp:   time.Now(),
	})
}

// AddSystemMessage adds a system message
func (m *Model) AddSystemMessage(content string) {
	m.AddMessage(DisplayMessage{
		Type:      MessageTypeSystem,
		Content:   content,
		Timestamp: time.Now(),
	})
}

// AddErrorMessage adds an error message
func (m *Model) AddErrorMessage(content string) {
	m.AddMessage(DisplayMessage{
		Type:      MessageTypeError,
		Content:   content,
		Timestamp: time.Now(),
		IsError:   true,
	})
}

// SetStreaming starts or stops streaming mode
func (m *Model) SetStreaming(streaming bool) {
	m.isStreaming = streaming
	if streaming {
		m.state = StateProcessing
		m.streamingContent = ""
		m.currentVerb = GetSpinnerVerb("default") // Reset to "Thinking"
	} else {
		m.state = StateInput
		m.currentVerb = ""
	}
}

// SetCurrentVerb sets the dynamic verb for the spinner
func (m *Model) SetCurrentVerb(action string) {
	m.currentVerb = GetSpinnerVerb(action)
}

// GetCurrentVerb returns the current spinner verb
func (m *Model) GetCurrentVerb() string {
	if m.currentVerb == "" {
		return GetSpinnerVerb("default")
	}
	return m.currentVerb
}

// AppendStreamContent appends content to the current stream
func (m *Model) AppendStreamContent(content string) {
	m.streamingContent += content
	m.updateViewport()
}

// UpdateTokens updates the token count
func (m *Model) UpdateTokens(input, output int) {
	m.tokens = input + output
}

// ShowPermissionPrompt shows a permission prompt
func (m *Model) ShowPermissionPrompt(req *PermissionRequest) {
	m.permissionRequest = req
	m.state = StatePermissionPrompt
}

// ClearMessages clears all messages
func (m *Model) ClearMessages() {
	m.messages = make([]DisplayMessage, 0)
	m.streamingContent = ""
	m.updateViewport()
}

// GetInput returns the current input text
func (m *Model) GetInput() string {
	return m.textarea.Value()
}

// ClearInput clears the input textarea
func (m *Model) ClearInput() {
	m.textarea.Reset()
}

// SetError sets an error message
func (m *Model) SetError(msg string) {
	m.errorMsg = msg
	m.state = StateError
}

// ClearError clears the error message
func (m *Model) ClearError() {
	m.errorMsg = ""
	if m.state == StateError {
		m.state = StateInput
	}
}

func (m *Model) updateViewport() {
	content := m.renderMessages()
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m *Model) renderMessages() string {
	var sb strings.Builder

	// Group consecutive tool messages
	var pendingTools []DisplayMessage
	flushTools := func() {
		if len(pendingTools) == 0 {
			return
		}
		// Render tools as a compact block with icons and details
		for _, tool := range pendingTools {
			var icon string
			var iconStyle lipgloss.Style
			// Empty Content means still running, non-empty means completed
			if tool.Content == "" {
				icon = "○"
				iconStyle = ToolSpinnerStyle
			} else if tool.IsError {
				icon = IconError
				iconStyle = ToolErrorStyle
			} else {
				icon = IconSuccess
				iconStyle = ToolSuccessStyle
			}
			sb.WriteString("  ")
			sb.WriteString(iconStyle.Render(icon))
			sb.WriteString(" ")
			sb.WriteString(ToolNameStyle.Render(tool.ToolName))

			// Always show description (file path, command) if available
			if tool.Description != "" {
				detail := tool.Description
				if idx := strings.Index(detail, "\n"); idx > 0 {
					detail = detail[:idx]
				}
				if len(detail) > 60 {
					detail = detail[:57] + "..."
				}
				sb.WriteString(" ")
				sb.WriteString(TextMutedStyle.Render(detail))
			}

			// Show error message if present
			if tool.IsError && tool.Content != "" {
				sb.WriteString("\n    ")
				errDetail := tool.Content
				if len(errDetail) > 80 {
					errDetail = errDetail[:77] + "..."
				}
				sb.WriteString(ErrorStyle.Render(errDetail))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		pendingTools = nil
	}

	for _, msg := range m.messages {
		switch msg.Type {
		case MessageTypeUser:
			flushTools()
			// User messages with subtle dark background (no label - like Claude Code)
			userStyle := UserMessageStyle.Width(m.viewport.Width - 2)
			sb.WriteString(userStyle.Render(msg.Content))
			sb.WriteString("\n\n")

		case MessageTypeAssistant:
			flushTools()
			// Assistant messages - just markdown, no label (like Claude Code)
			rendered := RenderMarkdown(msg.Content, m.viewport.Width)
			sb.WriteString(rendered)
			sb.WriteString("\n\n")

		case MessageTypeTool:
			pendingTools = append(pendingTools, msg)

		case MessageTypeSystem:
			flushTools()
			sb.WriteString(SystemMessageStyle.Render(msg.Content))
			sb.WriteString("\n\n")

		case MessageTypeError:
			flushTools()
			sb.WriteString(RenderError(msg.Content))
			sb.WriteString("\n\n")
		}
	}

	// Flush any remaining tools
	flushTools()

	// Add streaming content with cursor (no label - like Claude Code)
	if m.isStreaming && m.streamingContent != "" {
		rendered := RenderMarkdownStreaming(m.streamingContent, m.viewport.Width)
		sb.WriteString(rendered)
		// Blinking cursor indicator
		sb.WriteString(ToolSpinnerStyle.Render("▌"))
		sb.WriteString("\n")
	}

	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
}

// TickMsg is sent to keep the UI updating
type TickMsg struct{}

// PermissionRequestMsg is sent to request permission from the user
type PermissionRequestMsg struct {
	Request *PermissionRequest
}

// Tick returns a command that ticks continuously for smooth updates
func Tick() tea.Cmd {
	return tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}
