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
	textarea textarea.Model
	viewport viewport.Model
	spinner  spinner.Model

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
	onSubmit     func(string) tea.Cmd
	onPermission func(bool)
	onQuit       func()
}

// NewModel creates a new UI model
func NewModel() Model {
	// Create textarea for input
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send, Ctrl+J for newline)"
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)
	ta.Focus()

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: SpinnerFrames,
		FPS:    time.Millisecond * 80,
	}
	s.Style = ToolSpinnerStyle

	// Create viewport for messages
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle()

	return Model{
		state:        StateInput,
		textarea:     ta,
		viewport:     vp,
		spinner:      s,
		messages:     make([]DisplayMessage, 0),
		history:      make([]string, 0),
		historyIndex: -1,
		width:        80,
		height:       24,
		ready:        true, // Start ready immediately
	}
}

// SetHandlers sets the event handlers
func (m *Model) SetHandlers(onSubmit func(string) tea.Cmd, onPermission func(bool), onQuit func()) {
	m.onSubmit = onSubmit
	m.onPermission = onPermission
	m.onQuit = onQuit
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

	for _, msg := range m.messages {
		switch msg.Type {
		case MessageTypeUser:
			sb.WriteString(InputPromptStyle.Render("> "))
			sb.WriteString(UserMessageStyle.Render(msg.Content))
			sb.WriteString("\n\n")

		case MessageTypeAssistant:
			sb.WriteString(AssistantMessageStyle.Render(msg.Content))
			sb.WriteString("\n\n")

		case MessageTypeTool:
			if msg.IsError {
				sb.WriteString(ErrorStyle.Render("✗ "))
			} else {
				sb.WriteString(SuccessStyle.Render("○ "))
			}
			sb.WriteString(ToolNameStyle.Render(msg.ToolName))
			if msg.Content != "" {
				sb.WriteString("\n")
				sb.WriteString(ToolResultStyle.Render(truncate(msg.Content, 500)))
			}
			sb.WriteString("\n")

		case MessageTypeSystem:
			sb.WriteString(SystemMessageStyle.Render(msg.Content))
			sb.WriteString("\n\n")

		case MessageTypeError:
			sb.WriteString(ErrorStyle.Render("Error: " + msg.Content))
			sb.WriteString("\n\n")
		}
	}

	// Add streaming content
	if m.isStreaming && m.streamingContent != "" {
		sb.WriteString(AssistantMessageStyle.Render(m.streamingContent))
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
		tea.EnterAltScreen,
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
