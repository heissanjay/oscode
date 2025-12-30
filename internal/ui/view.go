package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Main content area (messages or welcome)
	sections = append(sections, m.renderContent())

	// Selection menu (if active) - overlays content
	if m.IsSelectionActive() {
		sections = append(sections, m.selection.View(m.width))
	}

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
	// Clean minimal header like Claude Code
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D97706")).
		Bold(true).
		Render("◉ oscode")

	var info []string
	if m.model != "" {
		// Show shortened model name
		modelName := m.model
		if len(modelName) > 25 {
			modelName = modelName[:22] + "..."
		}
		info = append(info, lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render(modelName))
	}

	right := strings.Join(info, " ")

	// Calculate spacing
	titleWidth := lipgloss.Width(title)
	rightWidth := lipgloss.Width(right)
	gap := m.width - titleWidth - rightWidth - 2
	if gap < 1 {
		gap = 1
	}

	headerContent := title + strings.Repeat(" ", gap) + right

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("#374151")).
		Render(headerContent)
}

func (m Model) renderContent() string {
	// Show welcome message if no messages yet
	if len(m.messages) == 0 && !m.isStreaming {
		return m.renderWelcome()
	}

	return m.viewport.View()
}

func (m Model) renderWelcome() string {
	// Centered welcome like Claude Code
	logo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D97706")).
		Bold(true).
		Render("◉ oscode")

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render("AI-powered coding assistant")

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D97706")).
		Bold(true)

	hints := []string{
		"Type a message to start chatting",
		keyStyle.Render("/model") + hintStyle.Render(" to switch models"),
		keyStyle.Render("/help") + hintStyle.Render(" or ") + keyStyle.Render("Tab") + hintStyle.Render(" for commands"),
	}

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		logo,
		"",
		subtitle,
		"",
		"",
		hintStyle.Render(hints[0]),
		hintStyle.Render(hints[1]),
		hintStyle.Render(hints[2]),
		"",
	)

	return lipgloss.Place(
		m.viewport.Width,
		m.viewport.Height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

func (m Model) renderPermissionPrompt() string {
	req := m.permissionRequest

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)

	cmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB")).
		Background(lipgloss.Color("#1F2937")).
		Padding(0, 1)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Bold(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render("⚠ Permission Required"))
	content.WriteString("\n\n")
	content.WriteString("Claude wants to run:\n\n")
	content.WriteString("  ")
	content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6366F1")).Bold(true).Render(req.Tool))
	if req.Command != "" {
		content.WriteString("\n  ")
		content.WriteString(cmdStyle.Render(req.Command))
	}
	content.WriteString("\n\n")

	keys := []string{
		keyStyle.Render("[Y]") + " Allow",
		keyStyle.Render("[N]") + " Deny",
		keyStyle.Render("[A]") + " Allow All",
	}
	content.WriteString(strings.Join(keys, "  "))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F59E0B")).
		Padding(1, 2).
		Width(m.width - 4).
		Render(content.String())
}

func (m Model) renderError() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#EF4444")).
		Padding(0, 2).
		Width(m.width - 4).
		Render("Error: " + m.errorMsg)
}

func (m Model) renderInput() string {
	// Input prompt indicator
	var promptIndicator string
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706")).Bold(true)

	switch m.state {
	case StateProcessing:
		// Show animated spinner during processing
		promptIndicator = m.spinner.View() + " "
	case StatePermissionPrompt:
		promptIndicator = promptStyle.Render("? ")
	default:
		promptIndicator = promptStyle.Render("> ")
	}

	// Build input line
	inputContent := promptIndicator + m.textarea.View()

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(lipgloss.Color("#374151")).
		Render(inputContent)
}

func (m Model) renderStatusBar() string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))

	// Left side: provider/model
	var left string
	if m.provider != "" {
		left = dimStyle.Render(m.provider)
	}

	// Center: status
	var center string
	switch m.state {
	case StateProcessing:
		center = accentStyle.Render("thinking...")
	case StatePermissionPrompt:
		center = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render("awaiting permission")
	case StateError:
		center = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("error")
	}

	// Right side: tokens + help
	var rightParts []string
	if m.tokens > 0 {
		rightParts = append(rightParts, dimStyle.Render(fmt.Sprintf("%s tokens", formatTokenCount(m.tokens))))
	}
	rightParts = append(rightParts, dimStyle.Render("^C quit"))
	right := strings.Join(rightParts, dimStyle.Render(" • "))

	// Calculate spacing
	leftWidth := lipgloss.Width(left)
	centerWidth := lipgloss.Width(center)
	rightWidth := lipgloss.Width(right)
	totalContent := leftWidth + centerWidth + rightWidth

	availableSpace := m.width - totalContent - 4
	if availableSpace < 2 {
		availableSpace = 2
	}

	leftGap := availableSpace / 2
	rightGap := availableSpace - leftGap

	var statusLine string
	if center != "" {
		statusLine = left + strings.Repeat(" ", leftGap) + center + strings.Repeat(" ", rightGap) + right
	} else {
		statusLine = left + strings.Repeat(" ", availableSpace) + right
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Background(lipgloss.Color("#1F2937")).
		Render(statusLine)
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

// RenderHelp renders the help screen
func RenderHelp() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB")).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D97706")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	var content strings.Builder

	content.WriteString(titleStyle.Render("Keyboard Shortcuts"))
	content.WriteString("\n\n")

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"Enter", "Submit message"},
		{"Ctrl+J", "Insert newline"},
		{"Ctrl+C", "Cancel/Quit"},
		{"Ctrl+L", "Clear screen"},
		{"↑/↓", "History navigation"},
		{"PgUp/PgDn", "Scroll messages"},
	}

	for _, s := range shortcuts {
		content.WriteString(keyStyle.Render(fmt.Sprintf("%-12s", s.key)))
		content.WriteString(descStyle.Render(s.desc))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(titleStyle.Render("Slash Commands"))
	content.WriteString("\n\n")

	commands := []struct {
		cmd  string
		desc string
	}{
		{"/help", "Show this help"},
		{"/clear", "Clear conversation"},
		{"/model", "Switch model"},
		{"/exit", "Exit application"},
	}

	for _, c := range commands {
		content.WriteString(keyStyle.Render(fmt.Sprintf("%-12s", c.cmd)))
		content.WriteString(descStyle.Render(c.desc))
		content.WriteString("\n")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(1, 2).
		Render(content.String())
}
