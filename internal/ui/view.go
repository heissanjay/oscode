package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "\n  " + ToolSpinnerStyle.Render("✢") + " " + ToolVerbStyle.Render("Initializing...")
	}

	// Calculate fixed element heights
	headerHeight := 2
	inputHeight := 3
	statusHeight := 1

	// For welcome screen - compact layout, input follows content naturally
	if len(m.messages) == 0 && !m.isStreaming {
		// Build welcome layout - content flows naturally, not forced to bottom
		welcomeStyle := lipgloss.NewStyle().
			Width(m.width).
			Padding(1, 2)

		welcomeContent := lipgloss.JoinVertical(lipgloss.Left,
			BrandLabelStyle.Render("OSCode"),
			"",
			TextMutedStyle.Render("What can I help you with?"),
			TextCloudyStyle.Render("Type a message or use /help for commands"),
		)

		// Input with borders
		borderStyle := lipgloss.NewStyle().Foreground(ColorBorder)
		borderLine := borderStyle.Render(strings.Repeat("─", m.width))

		inputStyle := lipgloss.NewStyle().Width(m.width).Padding(0, 1)
		var prompt string
		if m.state == StateProcessing || m.isStreaming {
			prompt = RenderSpinnerWithVerb(m.spinner.View(), m.GetCurrentVerb())
		} else {
			prompt = InputPromptStyle.Render("> ") + m.textarea.View()
		}
		inputBlock := lipgloss.JoinVertical(lipgloss.Left,
			borderLine,
			inputStyle.Render(prompt),
			borderLine,
		)

		// Status bar
		statusBar := m.renderStatusBar()

		// Join everything - this flows naturally without filling viewport height
		return lipgloss.JoinVertical(lipgloss.Left,
			welcomeStyle.Render(welcomeContent),
			inputBlock,
			statusBar,
		)
	}

	// For conversation mode - content flows naturally, scrolls only when needed
	var sections []string

	// 1. Header
	sections = append(sections, m.renderHeader())

	// 2. Content area - render messages directly, use viewport only for scrolling when needed
	overlayHeight := 0
	if m.state == StatePermissionPrompt && m.permissionRequest != nil {
		overlayHeight = 12
	}

	availableHeight := m.height - headerHeight - inputHeight - statusHeight - overlayHeight
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Get the actual rendered content
	content := m.renderMessages()
	contentLines := strings.Count(content, "\n") + 1

	// Only use viewport (with padding) when content exceeds available height
	if contentLines > availableHeight {
		// Content is longer than screen - use viewport for scrolling
		if m.viewport.Height != availableHeight {
			m.viewport.Height = availableHeight
		}
		sections = append(sections, m.viewport.View())
	} else {
		// Content fits - render directly without padding (flows naturally)
		sections = append(sections, content)
	}

	// 3. Overlays
	if m.state == StatePermissionPrompt && m.permissionRequest != nil {
		sections = append(sections, m.renderPermissionPrompt())
	}

	if m.state == StateError && m.errorMsg != "" {
		sections = append(sections, m.renderError())
	}

	if m.showingSuggestions && len(m.suggestions) > 0 {
		sections = append(sections, m.renderSuggestions())
	}

	if m.IsSelectionActive() {
		sections = append(sections, m.selection.View(m.width))
	}

	// 4. Input Area
	sections = append(sections, m.renderInput())

	// 5. Status Bar
	sections = append(sections, m.renderStatusBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	borderStyle := lipgloss.NewStyle().
		Width(m.width).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	return borderStyle.Render(BrandLabelStyle.Render("OSCode"))
}

func (m Model) renderWelcome() string {
	var content strings.Builder

	// Welcome message - clean and simple, no forced padding
	content.WriteString("\n")
	content.WriteString("  ")
	content.WriteString(BrandLabelStyle.Render("OSCode"))
	content.WriteString("\n\n")
	content.WriteString("  ")
	content.WriteString(TextMutedStyle.Render("What can I help you with?"))
	content.WriteString("\n")
	content.WriteString("  ")
	content.WriteString(TextCloudyStyle.Render("Type a message or use /help for commands"))
	content.WriteString("\n\n")

	return content.String()
}

func (m Model) renderSuggestions() string {
	if len(m.suggestions) == 0 {
		return ""
	}

	var parts []string
	for i, s := range m.suggestions {
		if i >= 5 {
			break
		}
		if i == m.suggestionCursor {
			parts = append(parts, SelectionSelectedStyle.Render(s.Label))
		} else {
			parts = append(parts, TextSecondaryStyle.Render(s.Label))
		}
	}

	return lipgloss.NewStyle().
		Padding(0, 1).
		Render("  " + strings.Join(parts, "  "))
}

func (m Model) renderPermissionPrompt() string {
	req := m.permissionRequest
	if req == nil {
		return ""
	}

	var content strings.Builder

	// Title with icon
	content.WriteString(PermissionTitleStyle.Render("⚡ Permission Required"))
	content.WriteString("\n\n")

	// Tool & Target
	content.WriteString(TextPrimaryStyle.Render("OSCode wants to run "))
	content.WriteString(PermissionToolStyle.Render(req.Tool))

	if req.FilePath != "" {
		content.WriteString(" on ")
		content.WriteString(PermissionPathStyle.Render(req.FilePath))
	} else if req.Command != "" {
		content.WriteString(":\n  ")
		content.WriteString(InlineCodeStyle.Render(truncate(req.Command, m.width-20)))
	}
	content.WriteString("\n\n")

	// Content Preview
	if req.IsDiff && (req.OldContent != "" || req.NewContent != "") {
		content.WriteString(m.renderUnifiedDiff(req.OldContent, req.NewContent))
		content.WriteString("\n")
	} else if req.NewContent != "" {
		content.WriteString(m.renderNewContent(req.NewContent))
		content.WriteString("\n")
	}

	// Options
	options := []struct {
		key   string
		label string
		desc  string
	}{
		{"y", "Yes", "Allow this operation"},
		{"a", "Always", "Allow all " + req.Tool + " for this session"},
		{"n", "No", "Reject and provide feedback"},
	}

	for i, opt := range options {
		content.WriteString(RenderPermissionOption(opt.key, opt.label, opt.desc, i == m.permissionChoice))
		content.WriteString("\n")
	}

	// Feedback input mode
	if m.rejectingWithInput {
		content.WriteString("\n")
		content.WriteString(TextMutedStyle.Render("Tell OSCode what to do instead: "))
		content.WriteString(m.textarea.View())
	}

	boxStyle := PermissionBoxStyle.Width(m.width - 4)
	return boxStyle.Render(content.String())
}

func (m Model) renderUnifiedDiff(oldContent, newContent string) string {
	var result strings.Builder

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Diff header
	result.WriteString(DiffHunkStyle.Render("@@ Changes @@"))
	result.WriteString("\n")

	maxLines := 8

	// Show removed lines
	for i, line := range oldLines {
		if i >= maxLines {
			remaining := len(oldLines) - maxLines
			if remaining > 0 {
				result.WriteString(TextMutedStyle.Render(fmt.Sprintf("  ... %d more lines removed", remaining)))
				result.WriteString("\n")
			}
			break
		}
		result.WriteString(DiffRemoveStyle.Render("- " + line))
		result.WriteString("\n")
	}

	// Show added lines
	for i, line := range newLines {
		if i >= maxLines {
			remaining := len(newLines) - maxLines
			if remaining > 0 {
				result.WriteString(TextMutedStyle.Render(fmt.Sprintf("  ... %d more lines added", remaining)))
				result.WriteString("\n")
			}
			break
		}
		result.WriteString(DiffAddStyle.Render("+ " + line))
		result.WriteString("\n")
	}

	return result.String()
}

func (m Model) renderNewContent(content string) string {
	var result strings.Builder

	lines := strings.Split(content, "\n")
	maxLines := 10

	for i, line := range lines {
		if i >= maxLines {
			remaining := len(lines) - maxLines
			result.WriteString(TextMutedStyle.Render(fmt.Sprintf("  ... and %d more lines", remaining)))
			result.WriteString("\n")
			break
		}
		result.WriteString(DiffAddStyle.Render("+ " + line))
		result.WriteString("\n")
	}

	return result.String()
}

func (m Model) renderError() string {
	errorBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorHeavyRed).
		Padding(0, 1).
		Width(m.width - 4)

	return errorBox.Render(RenderError(m.errorMsg))
}

func (m Model) renderInput() string {
	// Border line style
	borderStyle := lipgloss.NewStyle().
		Foreground(ColorBorder)
	borderLine := borderStyle.Render(strings.Repeat("─", m.width))

	// Input area style
	inputStyle := lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1)

	var prompt string

	if m.state == StateProcessing || m.isStreaming {
		// Sparkle spinner with dynamic verb
		verb := m.GetCurrentVerb()
		prompt = RenderSpinnerWithVerb(m.spinner.View(), verb)
	} else if m.state == StatePermissionPrompt && m.rejectingWithInput {
		// In feedback mode, show the textarea
		prompt = InputPromptStyle.Render("> ") + m.textarea.View()
	} else {
		// Normal input mode with Crail-colored prompt
		prompt = InputPromptStyle.Render("> ") + m.textarea.View()
	}

	inputContent := inputStyle.Render(prompt)

	// Return input with top and bottom borders
	return lipgloss.JoinVertical(lipgloss.Left,
		borderLine,
		inputContent,
		borderLine,
	)
}

func (m Model) renderStatusBar() string {
	// Format model name nicely
	modelDisplay := m.model
	if strings.HasPrefix(modelDisplay, "claude-") {
		parts := strings.Split(modelDisplay, "-")
		if len(parts) >= 2 {
			// claude-sonnet-4 -> Claude Sonnet 4
			name := parts[1]
			if len(name) > 0 {
				name = strings.ToUpper(name[:1]) + name[1:]
			}
			modelDisplay = "Claude " + name
			if len(parts) >= 3 {
				modelDisplay += " " + parts[2]
			}
		}
	} else if strings.HasPrefix(modelDisplay, "gpt-") {
		modelDisplay = strings.ToUpper(modelDisplay)
	}

	return RenderStatusLine(modelDisplay, m.tokens, m.width)
}

// formatTokenCount formats tokens for display - re-export for usage
func formatTokenCount(tokens int) string {
	return FormatTokenCount(tokens)
}
