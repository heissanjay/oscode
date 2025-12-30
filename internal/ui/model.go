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
	Type      MessageType
	Content   string
	Timestamp time.Time
	ToolName  string
	IsError   bool
}

// PermissionRequest represents a pending permission request
type PermissionRequest struct {
	Tool        string
	Description string
	Command     string
	Callback    func(bool)
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
	permissionRequest *PermissionRequest

	// Streaming state
	streamingContent string
	isStreaming      bool

	// Verbose mode
	verbose bool

	// Vim mode
	vimMode   bool
	vimNormal bool

	// Error message
	errorMsg string

	// Event handlers (set by app)
	onSubmit         func(string) tea.Cmd
	onPermission     func(bool)
	onQuit           func()
	onModelChange    func(string)
	onProviderChange func(string)
}

// NewModel creates a new UI model
func NewModel() Model {
	// Create textarea for input - single line, clean look
	ta := textarea.New()
	ta.Placeholder = "Message oscode..."
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.SetWidth(80)
	ta.SetHeight(1) // Single line by default
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)
	ta.Focus()

	// Create smooth spinner
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		FPS:    time.Millisecond * 80,
	}
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))

	// Create viewport for messages
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)

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
			{ID: "claude-sonnet-4-20250514", Label: "claude-sonnet-4", Description: "Best for coding tasks", Selected: true},
			{ID: "claude-opus-4-20250514", Label: "claude-opus-4", Description: "Most capable"},
			{ID: "claude-haiku-3-5-20241022", Label: "claude-haiku-3.5", Description: "Fast and efficient"},
			{ID: "gpt-4o", Label: "gpt-4o", Description: "OpenAI flagship"},
			{ID: "gpt-4o-mini", Label: "gpt-4o-mini", Description: "Fast OpenAI model"},
			{ID: "o1-preview", Label: "o1-preview", Description: "Reasoning model"},
		},
		availableProviders: []SelectionItem{
			{ID: "anthropic", Label: "Anthropic", Description: "Claude models", Selected: true},
			{ID: "openai", Label: "OpenAI", Description: "GPT models"},
		},
	}
}

// SetHandlers sets the event handlers
func (m *Model) SetHandlers(onSubmit func(string) tea.Cmd, onPermission func(bool), onQuit func()) {
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
func (m *Model) AddToolMessage(toolName, content string, isError bool) {
	m.AddMessage(DisplayMessage{
		Type:      MessageTypeTool,
		Content:   content,
		ToolName:  toolName,
		IsError:   isError,
		Timestamp: time.Now(),
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
	} else {
		m.state = StateInput
	}
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

	// Styles for Claude Code-like appearance
	userPromptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D97706")).
		Bold(true)

	userTextStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	assistantStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	toolNameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6366F1")).
		Bold(true)

	toolRunningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Italic(true)

	toolResultStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	successIcon := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Render("✓")

	errorIcon := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Render("✗")

	systemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Italic(true)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444"))

	for _, msg := range m.messages {
		switch msg.Type {
		case MessageTypeUser:
			sb.WriteString(userPromptStyle.Render("> "))
			sb.WriteString(userTextStyle.Render(msg.Content))
			sb.WriteString("\n\n")

		case MessageTypeAssistant:
			sb.WriteString(assistantStyle.Render(msg.Content))
			sb.WriteString("\n\n")

		case MessageTypeTool:
			if msg.Content == "Running..." {
				// Tool in progress
				sb.WriteString("  ")
				sb.WriteString(m.spinner.View())
				sb.WriteString(" ")
				sb.WriteString(toolNameStyle.Render(msg.ToolName))
				sb.WriteString(" ")
				sb.WriteString(toolRunningStyle.Render("running..."))
				sb.WriteString("\n")
			} else {
				// Tool completed
				if msg.IsError {
					sb.WriteString("  ")
					sb.WriteString(errorIcon)
				} else {
					sb.WriteString("  ")
					sb.WriteString(successIcon)
				}
				sb.WriteString(" ")
				sb.WriteString(toolNameStyle.Render(msg.ToolName))
				if msg.Content != "" && msg.Content != "Running..." {
					// Show truncated result
					result := truncate(msg.Content, 200)
					if result != "" {
						sb.WriteString("\n    ")
						sb.WriteString(toolResultStyle.Render(result))
					}
				}
				sb.WriteString("\n")
			}

		case MessageTypeSystem:
			sb.WriteString(systemStyle.Render("  " + msg.Content))
			sb.WriteString("\n\n")

		case MessageTypeError:
			sb.WriteString(errorStyle.Render("  Error: " + msg.Content))
			sb.WriteString("\n\n")
		}
	}

	// Add streaming content with cursor
	if m.isStreaming && m.streamingContent != "" {
		sb.WriteString(assistantStyle.Render(m.streamingContent))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D97706")).
			Render("▌"))
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

// Tick returns a command that ticks continuously for smooth updates
func Tick() tea.Cmd {
	return tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}
