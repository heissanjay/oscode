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

	// Header - minimal like Claude Code
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

	// Command suggestions (if typing slash command)
	if m.showingSuggestions && len(m.suggestions) > 0 {
		sections = append(sections, m.renderSuggestions())
	}

	// Input area - clean single line
	sections = append(sections, m.renderInput())

	// Status bar - minimal
	sections = append(sections, m.renderStatusBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	// Very minimal header - just a thin line with model info
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))

	// Show model name (shortened)
	modelName := m.model
	if len(modelName) > 25 {
		modelName = modelName[:22] + "..."
	}

	header := accentStyle.Render("oscode") + dimStyle.Render(" · "+modelName)

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Foreground(lipgloss.Color("#4B5563")).
		Render(header)
}

func (m Model) renderContent() string {
	// Show welcome message if no messages yet
	if len(m.messages) == 0 && !m.isStreaming {
		return m.renderWelcome()
	}

	return m.viewport.View()
}

func (m Model) renderWelcome() string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	// Very simple welcome
	content := dimStyle.Render("What can I help you with?")

	return lipgloss.Place(
		m.viewport.Width,
		m.viewport.Height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

func (m Model) renderSuggestions() string {
	if len(m.suggestions) == 0 {
		return ""
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))

	var parts []string
	for i, s := range m.suggestions {
		if i >= 5 {
			break
		}
		label := s.Label
		if i == m.suggestionCursor {
			parts = append(parts, selectedStyle.Render(label))
		} else {
			parts = append(parts, dimStyle.Render(label))
		}
	}

	return lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(lipgloss.Color("#6B7280")).
		Render("  " + strings.Join(parts, "  "))
}

func (m Model) renderPermissionPrompt() string {
	req := m.permissionRequest
	if req == nil {
		return ""
	}

	// Styles
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706")).Bold(true)
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706")).Bold(true)
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	_ = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")) // removeStyle - used in renderDiff
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(1, 2).
		Width(m.width - 4)

	var content strings.Builder

	// Header: Tool name and file path
	content.WriteString(toolStyle.Render(req.Tool))
	if req.FilePath != "" {
		content.WriteString(" ")
		content.WriteString(pathStyle.Render(req.FilePath))
	} else if req.Command != "" {
		content.WriteString(dimStyle.Render(": " + truncate(req.Command, 60)))
	}
	content.WriteString("\n\n")

	// Show diff for file operations
	if req.IsDiff && (req.OldContent != "" || req.NewContent != "") {
		content.WriteString(m.renderDiff(req.OldContent, req.NewContent))
		content.WriteString("\n")
	} else if req.NewContent != "" {
		// Show preview for Write operations
		lines := strings.Split(req.NewContent, "\n")
		maxLines := 10
		if len(lines) > maxLines {
			for i := 0; i < maxLines; i++ {
				content.WriteString(addStyle.Render("+ " + lines[i]))
				content.WriteString("\n")
			}
			content.WriteString(dimStyle.Render(fmt.Sprintf("  ... and %d more lines", len(lines)-maxLines)))
			content.WriteString("\n")
		} else {
			for _, line := range lines {
				content.WriteString(addStyle.Render("+ " + line))
				content.WriteString("\n")
			}
		}
		content.WriteString("\n")
	}

	// Action options - Claude Code style
	options := []struct {
		key   string
		label string
		desc  string
	}{
		{"y", "Yes", "Apply this change"},
		{"a", "Yes, don't ask again", "Allow all " + req.Tool + " operations"},
		{"n", "No", "Reject and tell Claude what you want instead"},
	}

	for i, opt := range options {
		prefix := "  "
		if i == m.permissionChoice {
			prefix = selectedStyle.Render("▶ ")
			content.WriteString(prefix)
			content.WriteString(keyStyle.Render("["+opt.key+"] "))
			content.WriteString(selectedStyle.Render(opt.label))
		} else {
			content.WriteString(prefix)
			content.WriteString(keyStyle.Render("["+opt.key+"] "))
			content.WriteString(dimStyle.Render(opt.label))
		}
		content.WriteString(dimStyle.Render(" - " + opt.desc))
		content.WriteString("\n")
	}

	// If in reject mode, show input prompt
	if m.rejectingWithInput {
		content.WriteString("\n")
		content.WriteString(dimStyle.Render("Tell Claude what you want: "))
		content.WriteString(m.textarea.View())
	}

	return borderStyle.Render(content.String())
}

func (m Model) renderDiff(oldContent, newContent string) string {
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	removeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	var result strings.Builder

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Simple diff display
	maxLines := 15
	shown := 0

	// Show removed lines
	for _, line := range oldLines {
		if shown >= maxLines {
			result.WriteString(dimStyle.Render(fmt.Sprintf("  ... %d more lines removed", len(oldLines)-shown)))
			result.WriteString("\n")
			break
		}
		result.WriteString(removeStyle.Render("- " + line))
		result.WriteString("\n")
		shown++
	}

	// Show added lines
	shown = 0
	for _, line := range newLines {
		if shown >= maxLines {
			result.WriteString(dimStyle.Render(fmt.Sprintf("  ... %d more lines added", len(newLines)-shown)))
			result.WriteString("\n")
			break
		}
		result.WriteString(addStyle.Render("+ " + line))
		result.WriteString("\n")
		shown++
	}

	return result.String()
}

func (m Model) renderError() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Padding(0, 1).
		Render("Error: " + m.errorMsg)
}

func (m Model) renderInput() string {
	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D97706"))

	var prompt string
	if m.state == StateProcessing {
		prompt = promptStyle.Render(m.spinner.View() + " ")
	} else {
		prompt = promptStyle.Render("> ")
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Render(prompt + m.textarea.View())
}

func (m Model) renderStatusBar() string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))

	var parts []string

	// Status during processing
	switch m.state {
	case StateProcessing:
		if m.isStreaming && m.streamingContent != "" {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("●"))
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706")).Render("○"))
		}
	case StatePermissionPrompt:
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render("?"))
	}

	// Scroll indicator if not at bottom
	if m.viewport.YOffset > 0 {
		total := m.viewport.TotalLineCount() - m.viewport.Height
		if total > 0 {
			pct := int(float64(m.viewport.YOffset) / float64(total) * 100)
			parts = append(parts, dimStyle.Render(fmt.Sprintf("↑%d%%", pct)))
		}
	}

	// Token count
	if m.tokens > 0 {
		parts = append(parts, dimStyle.Render(formatTokenCount(m.tokens)))
	}

	statusLine := strings.Join(parts, dimStyle.Render(" · "))

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Render(statusLine)
}

func formatTokenCount(tokens int) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d tok", tokens)
	}
	if tokens < 1000000 {
		return fmt.Sprintf("%.1fk tok", float64(tokens)/1000)
	}
	return fmt.Sprintf("%.1fM tok", float64(tokens)/1000000)
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
		{"Up/Down", "History / Scroll"},
		{"PgUp/PgDn", "Scroll page"},
	}

	for _, s := range shortcuts {
		content.WriteString(keyStyle.Render(fmt.Sprintf("%-12s", s.key)))
		content.WriteString(descStyle.Render(s.desc))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(titleStyle.Render("Commands"))
	content.WriteString("\n\n")

	commands := []struct {
		cmd  string
		desc string
	}{
		{"/help", "Show help"},
		{"/clear", "Clear conversation"},
		{"/model", "Switch model"},
		{"/exit", "Exit"},
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
