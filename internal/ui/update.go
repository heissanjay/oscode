package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Custom message types
type (
	// StreamTextMsg contains streamed text content
	StreamTextMsg struct {
		Content string
	}

	// StreamDoneMsg signals streaming is complete
	StreamDoneMsg struct {
		InputTokens  int
		OutputTokens int
	}

	// StreamErrorMsg signals a streaming error
	StreamErrorMsg struct {
		Error error
	}

	// ToolStartMsg signals a tool is starting
	ToolStartMsg struct {
		ToolName    string
		Description string
	}

	// ToolDoneMsg signals a tool completed
	ToolDoneMsg struct {
		ToolName string
		Result   string
		IsError  bool
	}

	// PermissionRequestMsg requests permission from user
	PermissionRequestMsg struct {
		Tool        string
		Description string
		Command     string
	}

	// ErrorMsg contains an error to display
	ErrorMsg struct {
		Error error
	}

	// ClearMsg signals to clear the screen
	ClearMsg struct{}

	// QuitMsg signals to quit
	QuitMsg struct{}
)

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update component sizes
		headerHeight := 3
		statusHeight := 1
		inputHeight := 4
		padding := 2
		viewportHeight := m.height - headerHeight - statusHeight - inputHeight - padding

		if viewportHeight < 5 {
			viewportHeight = 5
		}

		m.viewport.Width = m.width - 2
		m.viewport.Height = viewportHeight
		m.textarea.SetWidth(m.width - 4)

		if !m.ready {
			m.ready = true
		}

		m.updateViewport()
		return m, nil

	case TickMsg:
		// Continuous tick for smooth UI updates during streaming
		if m.isStreaming {
			cmds = append(cmds, Tick())
		}
		return m, tea.Batch(cmds...)

	case StreamTextMsg:
		m.AppendStreamContent(msg.Content)
		// Keep ticking during streaming for smooth updates
		if !m.isStreaming {
			m.SetStreaming(true)
			cmds = append(cmds, Tick())
		}
		return m, tea.Batch(cmds...)

	case StreamDoneMsg:
		// Finalize streaming
		if m.streamingContent != "" {
			m.AddAssistantMessage(m.streamingContent)
			m.streamingContent = ""
		}
		m.SetStreaming(false)
		m.UpdateTokens(msg.InputTokens, msg.OutputTokens)
		return m, nil

	case StreamErrorMsg:
		m.SetStreaming(false)
		m.AddErrorMessage(msg.Error.Error())
		return m, nil

	case ToolStartMsg:
		// Always show tool start with spinner effect
		m.AddToolMessage(msg.ToolName, "Running...", false)
		return m, nil

	case ToolDoneMsg:
		// Update the last tool message with result
		if len(m.messages) > 0 {
			lastIdx := len(m.messages) - 1
			if m.messages[lastIdx].Type == MessageTypeTool && m.messages[lastIdx].ToolName == msg.ToolName {
				m.messages[lastIdx].Content = msg.Result
				m.messages[lastIdx].IsError = msg.IsError
				m.updateViewport()
			} else {
				m.AddToolMessage(msg.ToolName, msg.Result, msg.IsError)
			}
		} else {
			m.AddToolMessage(msg.ToolName, msg.Result, msg.IsError)
		}
		return m, nil

	case PermissionRequestMsg:
		m.ShowPermissionPrompt(&PermissionRequest{
			Tool:        msg.Tool,
			Description: msg.Description,
			Command:     msg.Command,
		})
		return m, nil

	case ErrorMsg:
		m.AddErrorMessage(msg.Error.Error())
		return m, nil

	case ClearMsg:
		m.ClearMessages()
		return m, nil

	case QuitMsg:
		return m, tea.Quit

	case spinner.TickMsg:
		if m.state == StateProcessing || m.isStreaming {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	default:
		// Update textarea for cursor blink etc
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle permission prompt state
	if m.state == StatePermissionPrompt {
		return m.handlePermissionKeys(msg)
	}

	// Handle error state
	if m.state == StateError {
		m.ClearError()
		return m, nil
	}

	// Handle processing state (allow cancel)
	if m.state == StateProcessing {
		switch msg.String() {
		case "ctrl+c":
			m.SetStreaming(false)
			m.AddSystemMessage("Cancelled")
			return m, nil
		case "esc":
			m.SetStreaming(false)
			m.AddSystemMessage("Cancelled")
			return m, nil
		}
		return m, nil
	}

	// Handle vim mode
	if m.vimMode && m.vimNormal {
		return m.handleVimKeys(msg)
	}

	// Normal input mode
	switch msg.String() {
	case "ctrl+c":
		if m.textarea.Value() != "" {
			m.textarea.Reset()
			return m, nil
		}
		if m.onQuit != nil {
			m.onQuit()
		}
		return m, tea.Quit

	case "ctrl+d":
		if m.textarea.Value() == "" {
			if m.onQuit != nil {
				m.onQuit()
			}
			return m, tea.Quit
		}

	case "ctrl+l":
		m.ClearMessages()
		return m, nil

	case "ctrl+o":
		m.verbose = !m.verbose
		status := "off"
		if m.verbose {
			status = "on"
		}
		m.AddSystemMessage("Verbose mode: " + status)
		return m, nil

	case "enter":
		input := strings.TrimSpace(m.textarea.Value())
		if input == "" {
			return m, nil
		}

		// Add to history
		m.history = append(m.history, input)
		m.historyIndex = len(m.history)

		// Clear input
		m.textarea.Reset()

		// Add user message to display
		m.AddUserMessage(input)

		// Set processing state
		m.SetStreaming(true)

		// Call submit handler
		if m.onSubmit != nil {
			cmd := m.onSubmit(input)
			cmds = append(cmds, cmd)
		}

		return m, tea.Batch(cmds...)

	case "ctrl+j": // Insert newline
		m.textarea.InsertString("\n")
		return m, nil

	case "up":
		// History navigation
		if m.historyIndex > 0 {
			m.historyIndex--
			m.textarea.SetValue(m.history[m.historyIndex])
			m.textarea.CursorEnd()
		}
		return m, nil

	case "down":
		// History navigation
		if m.historyIndex < len(m.history)-1 {
			m.historyIndex++
			m.textarea.SetValue(m.history[m.historyIndex])
			m.textarea.CursorEnd()
		} else if m.historyIndex == len(m.history)-1 {
			m.historyIndex = len(m.history)
			m.textarea.Reset()
		}
		return m, nil

	case "esc":
		if m.vimMode {
			m.vimNormal = true
			return m, nil
		}

	case "pgup":
		m.viewport.ViewUp()
		return m, nil

	case "pgdown":
		m.viewport.ViewDown()
		return m, nil
	}

	// Update textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) handlePermissionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y":
		if m.onPermission != nil {
			m.onPermission(true)
		}
		m.permissionRequest = nil
		m.state = StateProcessing
		return m, nil

	case "n":
		if m.onPermission != nil {
			m.onPermission(false)
		}
		m.permissionRequest = nil
		m.state = StateInput
		m.AddSystemMessage("Permission denied")
		return m, nil

	case "a": // Allow all for session
		if m.onPermission != nil {
			m.onPermission(true)
		}
		// TODO: Add to session allow list
		m.permissionRequest = nil
		m.state = StateProcessing
		return m, nil

	case "ctrl+c", "esc":
		if m.onPermission != nil {
			m.onPermission(false)
		}
		m.permissionRequest = nil
		m.state = StateInput
		return m, nil
	}

	return m, nil
}

func (m Model) handleVimKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "i":
		m.vimNormal = false
		return m, nil
	case "a":
		m.vimNormal = false
		// Move cursor right
		return m, nil
	case "I":
		m.vimNormal = false
		m.textarea.CursorStart()
		return m, nil
	case "A":
		m.vimNormal = false
		m.textarea.CursorEnd()
		return m, nil
	case "h":
		// Move left
		return m, nil
	case "l":
		// Move right
		return m, nil
	case "j":
		m.viewport.ViewDown()
		return m, nil
	case "k":
		m.viewport.ViewUp()
		return m, nil
	case "g":
		m.viewport.GotoTop()
		return m, nil
	case "G":
		m.viewport.GotoBottom()
		return m, nil
	case "d":
		// Delete line
		m.textarea.Reset()
		return m, nil
	case ":":
		// Command mode (future enhancement)
		return m, nil
	}

	return m, nil
}

// Focus focuses the textarea
func (m *Model) Focus() tea.Cmd {
	return m.textarea.Focus()
}

// Blur blurs the textarea
func (m *Model) Blur() {
	m.textarea.Blur()
}

// TextareaUpdate updates just the textarea component
func (m *Model) TextareaUpdate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return cmd
}

// TickSpinner returns a command to tick the spinner
func (m Model) TickSpinner() tea.Cmd {
	return m.spinner.Tick
}

// SendStreamText sends stream text to the UI
func SendStreamText(content string) tea.Cmd {
	return func() tea.Msg {
		return StreamTextMsg{Content: content}
	}
}

// SendStreamDone signals stream completion
func SendStreamDone(inputTokens, outputTokens int) tea.Cmd {
	return func() tea.Msg {
		return StreamDoneMsg{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		}
	}
}

// SendStreamError signals a stream error
func SendStreamError(err error) tea.Cmd {
	return func() tea.Msg {
		return StreamErrorMsg{Error: err}
	}
}

// SendToolStart signals tool start
func SendToolStart(name, desc string) tea.Cmd {
	return func() tea.Msg {
		return ToolStartMsg{ToolName: name, Description: desc}
	}
}

// SendToolDone signals tool completion
func SendToolDone(name, result string, isError bool) tea.Cmd {
	return func() tea.Msg {
		return ToolDoneMsg{ToolName: name, Result: result, IsError: isError}
	}
}

// SendPermissionRequest sends a permission request
func SendPermissionRequest(tool, desc, cmd string) tea.Cmd {
	return func() tea.Msg {
		return PermissionRequestMsg{Tool: tool, Description: desc, Command: cmd}
	}
}

// SendError sends an error message
func SendError(err error) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Error: err}
	}
}

// SendClear sends a clear message
func SendClear() tea.Cmd {
	return func() tea.Msg {
		return ClearMsg{}
	}
}

// SendQuit sends a quit message
func SendQuit() tea.Cmd {
	return func() tea.Msg {
		return QuitMsg{}
	}
}
