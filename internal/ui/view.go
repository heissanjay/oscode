package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Main content area
	sections = append(sections, m.renderContent())

	// Permission prompt (if active)
	if m.state == StatePermissionPrompt && m.permissionRequest != nil {
		sections = append(sections, m.renderPermissionPrompt())
	}

	// Error display (if any)
	if m.state == StateError && m.errorMsg != "" {
		sections = append(sections, m.renderError())
	}

	// Input area
	sections = append(sections, m.renderInput())

	// Status bar
	sections = append(sections, m.renderStatusBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	title := LogoStyle.Render("oscode")

	var info []string
	if m.provider != "" && m.model != "" {
		info = append(info, SubtitleStyle.Render(m.provider+"/"+m.model))
	}
	if m.sessionID != "" {
		info = append(info, VersionStyle.Render("session: "+truncate(m.sessionID, 8)))
	}

	right := strings.Join(info, " | ")

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}
	spaces := strings.Repeat(" ", gap)

	header := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(ColorBorder).
		Width(m.width).
		Render(title + spaces + right)

	return header
}

func (m Model) renderContent() string {
	// Show welcome message if no messages yet
	if len(m.messages) == 0 && !m.isStreaming {
		welcome := RenderWelcome("1.0.0")
		return lipgloss.Place(
			m.viewport.Width,
			m.viewport.Height,
			lipgloss.Center,
			lipgloss.Center,
			welcome,
		)
	}

	// Render the viewport with messages
	content := m.viewport.View()

	// Add spinner if processing
	if m.state == StateProcessing {
		spinnerLine := m.spinner.View() + " " + ToolDescStyle.Render("Processing...")
		content = lipgloss.JoinVertical(lipgloss.Left, content, spinnerLine)
	}

	return content
}

func (m Model) renderPermissionPrompt() string {
	req := m.permissionRequest

	var content strings.Builder
	content.WriteString(PermissionTitleStyle.Render("⚠ Permission Required"))
	content.WriteString("\n\n")
	content.WriteString("Claude wants to execute:\n\n")
	content.WriteString("  ")
	content.WriteString(ToolNameStyle.Render(req.Tool))
	if req.Command != "" {
		content.WriteString(": ")
		content.WriteString(PermissionCommandStyle.Render(req.Command))
	}
	content.WriteString("\n\n")

	if req.Description != "" {
		content.WriteString(ToolDescStyle.Render(req.Description))
		content.WriteString("\n\n")
	}

	// Key bindings
	keys := []string{
		PermissionKeyStyle.Render("[Y]") + " Allow",
		PermissionKeyStyle.Render("[N]") + " Deny",
		PermissionKeyStyle.Render("[A]") + " Allow All",
	}
	content.WriteString(strings.Join(keys, "  "))

	return PermissionBoxStyle.Width(m.width - 4).Render(content.String())
}

func (m Model) renderError() string {
	return ErrorBoxStyle.Width(m.width - 4).Render(m.errorMsg)
}

func (m Model) renderInput() string {
	var prefix string

	switch m.state {
	case StateProcessing:
		prefix = ToolSpinnerStyle.Render("⋯ ")
	case StatePermissionPrompt:
		prefix = WarningStyle.Render("? ")
	default:
		if m.vimMode && m.vimNormal {
			prefix = InfoStyle.Render("N ")
		} else {
			prefix = ""
		}
	}

	inputArea := prefix + m.textarea.View()

	// Add border
	bordered := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(ColorBorder).
		Width(m.width).
		Render(inputArea)

	return bordered
}

func (m Model) renderStatusBar() string {
	// Left side: provider/model info
	var left []string
	if m.provider != "" {
		left = append(left, m.provider)
	}
	if m.model != "" {
		// Shorten model name
		modelShort := m.model
		if len(modelShort) > 20 {
			modelShort = modelShort[:17] + "..."
		}
		left = append(left, modelShort)
	}
	leftStr := StatusItemStyle.Render(strings.Join(left, "/"))

	// Center: mode indicator
	var mode string
	switch m.state {
	case StateProcessing:
		mode = InfoStyle.Render("processing")
	case StatePermissionPrompt:
		mode = WarningStyle.Render("permission")
	case StateError:
		mode = ErrorStyle.Render("error")
	default:
		if m.vimMode {
			if m.vimNormal {
				mode = InfoStyle.Render("NORMAL")
			} else {
				mode = SuccessStyle.Render("INSERT")
			}
		}
	}

	// Right side: token count and help
	var right []string
	if m.tokens > 0 {
		right = append(right, fmt.Sprintf("%s tokens", formatTokenCount(m.tokens)))
	}
	right = append(right, HelpKeyStyle.Render("Ctrl+C")+" quit")
	rightStr := StatusItemStyle.Render(strings.Join(right, " | "))

	// Calculate spacing
	leftWidth := lipgloss.Width(leftStr)
	modeWidth := lipgloss.Width(mode)
	rightWidth := lipgloss.Width(rightStr)

	totalContent := leftWidth + modeWidth + rightWidth
	gap := m.width - totalContent - 4
	if gap < 2 {
		gap = 2
	}

	leftGap := gap / 2
	rightGap := gap - leftGap

	var statusContent string
	if mode != "" {
		statusContent = leftStr + strings.Repeat(" ", leftGap) + mode + strings.Repeat(" ", rightGap) + rightStr
	} else {
		statusContent = leftStr + strings.Repeat(" ", gap) + rightStr
	}

	return StatusBarStyle.Width(m.width).Render(statusContent)
}

func formatTokenCount(tokens int) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d", tokens)
	}
	if tokens < 1000000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
}

// RenderWelcomeScreen renders the initial welcome screen
func (m Model) RenderWelcomeScreen() string {
	welcome := RenderWelcome("1.0.0")

	// Center it
	return lipgloss.Place(
		m.width,
		m.height/2,
		lipgloss.Center,
		lipgloss.Center,
		welcome,
	)
}

// RenderHelp renders the help screen
func RenderHelp() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("Keyboard Shortcuts"))
	content.WriteString("\n\n")

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"Enter", "Submit message"},
		{"Ctrl+J", "Insert newline"},
		{"Ctrl+C", "Cancel/Quit"},
		{"Ctrl+D", "Exit (if empty)"},
		{"Ctrl+L", "Clear screen"},
		{"Ctrl+O", "Toggle verbose"},
		{"Up/Down", "History navigation"},
		{"PgUp/PgDn", "Scroll messages"},
		{"Esc", "Enter vim normal mode"},
	}

	for _, s := range shortcuts {
		content.WriteString(RenderKeyBinding(s.key, s.desc))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(TitleStyle.Render("Slash Commands"))
	content.WriteString("\n\n")

	commands := []struct {
		cmd  string
		desc string
	}{
		{"/help", "Show this help"},
		{"/clear", "Clear conversation"},
		{"/model", "Switch model"},
		{"/provider", "Switch provider"},
		{"/exit", "Exit application"},
		{"/compact", "Compact conversation"},
		{"/cost", "Show token usage"},
		{"/vim", "Toggle vim mode"},
	}

	for _, c := range commands {
		content.WriteString(HelpKeyStyle.Render(c.cmd))
		content.WriteString(" ")
		content.WriteString(HelpDescStyle.Render(c.desc))
		content.WriteString("\n")
	}

	return BoxStyle.Render(content.String())
}
