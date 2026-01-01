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

	case tea.MouseMsg:
		// Handle mouse scrolling
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.viewport.LineUp(3)
				return m, nil
			case tea.MouseButtonWheelDown:
				m.viewport.LineDown(3)
				return m, nil
			}
		}
		// Pass mouse events to viewport
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update component sizes - OSCode compact layout
		headerHeight := 1  // Single line header
		statusHeight := 1  // Single line status
		inputHeight := 1   // Single line input
		padding := 2       // Top/bottom padding
		viewportHeight := m.height - headerHeight - statusHeight - inputHeight - padding

		if viewportHeight < 5 {
			viewportHeight = 5
		}

		m.viewport.Width = m.width - 2
		m.viewport.Height = viewportHeight
		m.textarea.SetWidth(m.width - 6) // Account for prompt indicator

		if !m.ready {
			m.ready = true
		}

		m.updateViewport()
		return m, nil

	case TickMsg:
		// Continuous tick for smooth UI updates during streaming
		if m.isStreaming || m.state == StateProcessing {
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

	case PermissionRequestMsg:
		// Show permission prompt
		m.permissionRequest = msg.Request
		m.permissionChoice = 0
		m.rejectingWithInput = false
		m.state = StatePermissionPrompt
		return m, nil

	case StreamDoneMsg:
		// IMPORTANT: First stop streaming, THEN add message to avoid double rendering
		content := m.streamingContent
		m.streamingContent = ""
		m.SetStreaming(false)

		// Now add the message (viewport will render without streaming content)
		if content != "" {
			m.AddAssistantMessage(content)
		}
		m.UpdateTokens(msg.InputTokens, msg.OutputTokens)
		return m, nil

	case StreamErrorMsg:
		m.streamingContent = ""
		m.SetStreaming(false)
		m.AddErrorMessage(msg.Error.Error())
		return m, nil

	case ToolStartMsg:
		// Show tool start with description (file path, command, etc.)
		m.AddToolMessage(msg.ToolName, msg.Description, false)
		return m, m.spinner.Tick

	case ToolDoneMsg:
		// Update the last tool message with result (keep description, update status)
		if len(m.messages) > 0 {
			lastIdx := len(m.messages) - 1
			if m.messages[lastIdx].Type == MessageTypeTool && m.messages[lastIdx].ToolName == msg.ToolName {
				m.messages[lastIdx].Content = msg.Result // Store result/error in Content
				m.messages[lastIdx].IsError = msg.IsError
				m.updateViewport()
			} else {
				// Tool message doesn't exist, create with result as description fallback
				m.AddToolMessage(msg.ToolName, msg.Result, msg.IsError)
			}
		} else {
			m.AddToolMessage(msg.ToolName, msg.Result, msg.IsError)
		}
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
		// Update spinner and keep it ticking during processing/streaming
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.state == StateProcessing || m.isStreaming {
			cmds = append(cmds, cmd)
			// Force viewport update to show spinner animation
			m.updateViewport()
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

	// Handle selection menu first if active
	if m.IsSelectionActive() {
		return m.handleSelectionKeys(msg)
	}

	// Handle permission prompt state
	if m.state == StatePermissionPrompt {
		return m.handlePermissionKeys(msg)
	}

	// Handle error state
	if m.state == StateError {
		m.ClearError()
		return m, nil
	}

	// Handle processing state (allow cancel and scrolling)
	if m.state == StateProcessing || m.isStreaming {
		switch msg.String() {
		case "ctrl+c", "esc":
			m.streamingContent = ""
			m.SetStreaming(false)
			m.AddSystemMessage("Cancelled")
			return m, nil
		case "up", "k":
			m.viewport.LineUp(1)
			return m, nil
		case "down", "j":
			m.viewport.LineDown(1)
			return m, nil
		case "pgup":
			m.viewport.ViewUp()
			return m, nil
		case "pgdown":
			m.viewport.ViewDown()
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
			m.showingSuggestions = false
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

		// Hide suggestions if showing
		m.showingSuggestions = false

		// Handle slash commands interactively
		if strings.HasPrefix(input, "/") {
			m.textarea.Reset()
			cmd := strings.TrimPrefix(input, "/")
			cmd = strings.ToLower(strings.TrimSpace(cmd))

			// Get command name (first word)
			parts := strings.SplitN(cmd, " ", 2)
			cmdName := parts[0]

			switch cmdName {
			case "model", "m":
				m.ShowModelSelection()
				return m, nil

			case "provider", "p":
				m.ShowProviderSelection()
				return m, nil

			case "help", "h", "?":
				m.ShowHelpMenu()
				return m, nil

			case "clear", "cls":
				m.ClearMessages()
				m.AddSystemMessage("Conversation cleared")
				return m, nil

			case "exit", "quit", "q":
				if m.onQuit != nil {
					m.onQuit()
				}
				return m, tea.Quit

			case "vim":
				m.vimMode = !m.vimMode
				if m.vimMode {
					m.AddSystemMessage("Vim mode enabled")
				} else {
					m.AddSystemMessage("Vim mode disabled")
				}
				return m, nil

			case "verbose", "v":
				m.verbose = !m.verbose
				if m.verbose {
					m.AddSystemMessage("Verbose mode enabled")
				} else {
					m.AddSystemMessage("Verbose mode disabled")
				}
				return m, nil

			case "cost":
				m.AddSystemMessage("Token usage: " + FormatTokenCount(m.tokens) + " tokens")
				return m, nil

			case "compact":
				m.AddSystemMessage("Conversation compacting not yet implemented")
				return m, nil

			default:
				m.AddSystemMessage("Unknown command: /" + cmdName + " (use /help)")
				return m, nil
			}
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

		// Start spinner ticking
		cmds = append(cmds, m.spinner.Tick, Tick())

		// Call submit handler
		if m.onSubmit != nil {
			cmd := m.onSubmit(input)
			cmds = append(cmds, cmd)
		}

		return m, tea.Batch(cmds...)

	case "ctrl+j": // Insert newline
		m.textarea.InsertString("\n")
		return m, nil

	case "tab":
		// If showing inline suggestions, select the current one
		if m.showingSuggestions && len(m.suggestions) > 0 {
			selected := m.suggestions[m.suggestionCursor]
			m.textarea.SetValue(selected.Label)
			m.textarea.CursorEnd()
			m.showingSuggestions = false
			return m, nil
		}
		// Show command palette if input starts with /
		input := m.textarea.Value()
		if strings.HasPrefix(input, "/") {
			filter := strings.TrimPrefix(input, "/")
			m.ShowCommandPalette(filter)
			return m, nil
		}
		return m, nil

	case "up":
		// Navigate suggestions if showing
		if m.showingSuggestions && len(m.suggestions) > 0 {
			if m.suggestionCursor > 0 {
				m.suggestionCursor--
			} else {
				m.suggestionCursor = len(m.suggestions) - 1
			}
			return m, nil
		}
		// History navigation when not showing suggestions
		if m.historyIndex > 0 {
			m.historyIndex--
			m.textarea.SetValue(m.history[m.historyIndex])
			m.textarea.CursorEnd()
		}
		return m, nil

	case "down":
		// Navigate suggestions if showing
		if m.showingSuggestions && len(m.suggestions) > 0 {
			if m.suggestionCursor < len(m.suggestions)-1 {
				m.suggestionCursor++
			} else {
				m.suggestionCursor = 0
			}
			return m, nil
		}
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
		if m.showingSuggestions {
			m.showingSuggestions = false
			return m, nil
		}
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

	// Check if we should show suggestions (when typing slash commands)
	input := m.textarea.Value()
	if strings.HasPrefix(input, "/") && len(input) > 1 {
		m.showingSuggestions = true
		m.updateSuggestions(strings.TrimPrefix(input, "/"))
	} else {
		m.showingSuggestions = false
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handlePermissionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Guard against nil permission request
	if m.permissionRequest == nil {
		m.state = StateInput
		return m, nil
	}

	// If typing feedback for rejection
	if m.rejectingWithInput {
		switch msg.String() {
		case "enter":
			// Submit the feedback
			feedback := m.textarea.Value()
			resp := PermissionResponse{Allowed: false, Feedback: feedback}
			if m.permissionRequest != nil && m.permissionRequest.Callback != nil {
				m.permissionRequest.Callback(resp)
			}
			if m.onPermission != nil {
				m.onPermission(resp)
			}
			m.permissionRequest = nil
			m.rejectingWithInput = false
			m.state = StateInput
			m.textarea.Reset()
			if feedback != "" {
				m.AddSystemMessage("Permission denied: " + feedback)
			} else {
				m.AddSystemMessage("Permission denied")
			}
			return m, nil
		case "esc":
			// Cancel feedback, go back to selection
			m.rejectingWithInput = false
			m.textarea.Reset()
			return m, nil
		default:
			// Pass to textarea
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
	}

	switch strings.ToLower(msg.String()) {
	case "y":
		// Quick shortcut for yes
		resp := PermissionResponse{Allowed: true}
		if m.permissionRequest != nil && m.permissionRequest.Callback != nil {
			m.permissionRequest.Callback(resp)
		}
		if m.onPermission != nil {
			m.onPermission(resp)
		}
		m.permissionRequest = nil
		m.permissionChoice = 0
		m.state = StateProcessing
		return m, nil

	case "a":
		// Quick shortcut for yes, don't ask again
		resp := PermissionResponse{Allowed: true, DontAskAgain: true}
		if m.permissionRequest != nil && m.permissionRequest.Callback != nil {
			m.permissionRequest.Callback(resp)
		}
		if m.onPermission != nil {
			m.onPermission(resp)
		}
		m.permissionRequest = nil
		m.permissionChoice = 0
		m.state = StateProcessing
		return m, nil

	case "n":
		// Quick shortcut for no - go to feedback mode
		m.permissionChoice = 2
		m.rejectingWithInput = true
		m.textarea.Focus()
		m.textarea.SetValue("")
		return m, nil

	case "up", "k":
		if m.permissionChoice > 0 {
			m.permissionChoice--
		}
		return m, nil

	case "down", "j":
		if m.permissionChoice < 2 {
			m.permissionChoice++
		}
		return m, nil

	case "enter":
		// Execute selected option
		switch m.permissionChoice {
		case 0: // Yes
			resp := PermissionResponse{Allowed: true}
			if m.permissionRequest != nil && m.permissionRequest.Callback != nil {
				m.permissionRequest.Callback(resp)
			}
			if m.onPermission != nil {
				m.onPermission(resp)
			}
			m.permissionRequest = nil
			m.state = StateProcessing
			return m, nil
		case 1: // Yes, don't ask again
			resp := PermissionResponse{Allowed: true, DontAskAgain: true}
			if m.permissionRequest != nil && m.permissionRequest.Callback != nil {
				m.permissionRequest.Callback(resp)
			}
			if m.onPermission != nil {
				m.onPermission(resp)
			}
			m.permissionRequest = nil
			m.state = StateProcessing
			return m, nil
		case 2: // No with feedback
			m.rejectingWithInput = true
			m.textarea.Focus()
			m.textarea.SetValue("")
			return m, nil
		}

	case "ctrl+c", "esc":
		resp := PermissionResponse{Allowed: false}
		if m.permissionRequest != nil && m.permissionRequest.Callback != nil {
			m.permissionRequest.Callback(resp)
		}
		if m.onPermission != nil {
			m.onPermission(resp)
		}
		m.permissionRequest = nil
		m.permissionChoice = 0
		m.state = StateInput
		return m, nil
	}

	return m, nil
}

func (m Model) handleSelectionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.selection.MoveUp()
		return m, nil

	case "down", "j":
		m.selection.MoveDown()
		return m, nil

	case "enter":
		item := m.selection.Select()
		if item != nil {
			selState := m.selection.State
			m.selection.Hide()

			switch selState {
			case SelectionModelMenu:
				m.model = item.ID
				if m.onModelChange != nil {
					m.onModelChange(item.ID)
				}
				m.AddSystemMessage("Model set to: " + item.Label)

			case SelectionProviderMenu:
				m.provider = item.ID
				if m.onProviderChange != nil {
					m.onProviderChange(item.ID)
				}
				m.AddSystemMessage("Provider set to: " + item.Label)

			case SelectionHelpMenu:
				// Clear the textarea first
				m.textarea.Reset()

				// Execute the selected command
				switch item.ID {
				case "help":
					m.ShowHelpMenu()
				case "clear":
					m.ClearMessages()
					m.AddSystemMessage("Conversation cleared")
				case "model":
					m.ShowModelSelection()
				case "provider":
					m.ShowProviderSelection()
				case "vim":
					m.vimMode = !m.vimMode
					if m.vimMode {
						m.AddSystemMessage("Vim mode enabled")
					} else {
						m.AddSystemMessage("Vim mode disabled")
					}
				case "verbose":
					m.verbose = !m.verbose
					if m.verbose {
						m.AddSystemMessage("Verbose mode enabled")
					} else {
						m.AddSystemMessage("Verbose mode disabled")
					}
				case "cost":
					m.AddSystemMessage("Token usage: " + FormatTokenCount(m.tokens) + " tokens")
				case "compact":
					m.AddSystemMessage("Conversation compacting not yet implemented")
				case "exit":
					if m.onQuit != nil {
						m.onQuit()
					}
					return m, tea.Quit
				default:
					m.AddSystemMessage("Command not yet implemented: " + item.ID)
				}
			}
		}
		return m, nil

	case "esc", "ctrl+c", "q":
		m.selection.Hide()
		return m, nil

	case "backspace":
		m.selection.RemoveFilterChar()
		return m, nil

	default:
		// Type to filter
		if len(msg.String()) == 1 {
			r := rune(msg.String()[0])
			if r >= 32 && r < 127 {
				m.selection.AddFilterChar(r)
			}
		}
		return m, nil
	}
}

func (m Model) handleVimKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "i":
		m.vimNormal = false
		return m, nil
	case "a":
		m.vimNormal = false
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
		return m, nil
	case "l":
		return m, nil
	case "j":
		m.viewport.LineDown(1)
		return m, nil
	case "k":
		m.viewport.LineUp(1)
		return m, nil
	case "g":
		m.viewport.GotoTop()
		return m, nil
	case "G":
		m.viewport.GotoBottom()
		return m, nil
	case "d":
		m.textarea.Reset()
		return m, nil
	case "ctrl+d":
		m.viewport.HalfViewDown()
		return m, nil
	case "ctrl+u":
		m.viewport.HalfViewUp()
		return m, nil
	case ":":
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
func SendPermissionRequest(req *PermissionRequest) tea.Cmd {
	return func() tea.Msg {
		return PermissionRequestMsg{Request: req}
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
