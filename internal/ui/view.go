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

	// Calculate heights
	height := m.height
	headerHeight := 1 // Minimal header
	inputHeight := 1  // Single line input
	statusHeight := 1 // Single line status bar
	
	// Adjust input height if rejection with feedback is active
	if m.state == StatePermissionPrompt && m.rejectingWithInput {
		inputHeight = 3
	}

	// Calculate viewport height
	// We reserve space for header, input, status, and potentially permission prompt
	availableHeight := height - headerHeight - inputHeight - statusHeight
	
	// If permission prompt is active, it takes up some space
	if m.state == StatePermissionPrompt && m.permissionRequest != nil {
		// Estimate height of permission prompt (border + content)
		// This is a bit dynamic, but we can set a max height or safe buffer
		permHeight := 10 // Rough estimate
		availableHeight -= permHeight
	}

	// Update viewport height if changed
	if m.viewport.Height != availableHeight && availableHeight > 0 {
		m.viewport.Height = availableHeight
	}

	var sections []string

	// 1. Header (Pinned Top)
	sections = append(sections, m.renderHeader())

	// 2. Main Content (Viewport)
	if len(m.messages) == 0 && !m.isStreaming {
		sections = append(sections, m.renderWelcome())
	} else {
		sections = append(sections, m.viewport.View())
	}

	// 3. Overlays (Permission, Error, Suggestions)
	// These are rendered "in flow" but functionally act as overlays in the stack
	
	// Permission prompt
	if m.state == StatePermissionPrompt && m.permissionRequest != nil {
		sections = append(sections, m.renderPermissionPrompt())
	}

	// Error display
	if m.state == StateError && m.errorMsg != "" {
		sections = append(sections, m.renderError())
	}

	// Command suggestions
	if m.showingSuggestions && len(m.suggestions) > 0 {
		sections = append(sections, m.renderSuggestions())
	}

	// Selection menu (if active)
	if m.IsSelectionActive() {
		sections = append(sections, m.selection.View(m.width))
	}

	// 4. Spacer (to push Input/Status to bottom if content is short)
	// In bubbletea with a viewport, the viewport usually fills the space. 
	// If we are not using viewport full height, we might need padding.
	// But sections are joined vertically.

	// 5. Input Area
	sections = append(sections, m.renderInput())

	// 6. Status Bar (Pinned Bottom)
	sections = append(sections, m.renderStatusBar())

	// Join all sections
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	// Minimal header - just the logo/name
	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		BorderBottom(true).
		BorderForeground(ColorBorder).
		Render(ClaudeLabelStyle.Render("Claude Code"))
}

func (m Model) renderWelcome() string {
	// Centered welcome message
	title := ClaudeLabelStyle.Copy().Render("Claude Code")
	subtitle := TextMutedStyle.Render("What can I help you with?")

	return lipgloss.Place(
		m.width,
		m.viewport.Height,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, title, "", subtitle),
	)
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

	// Title
	content.WriteString(PermissionTitleStyle.Render("Permission Required"))
	content.WriteString("\n\n")
	
	// Tool & Description
	content.WriteString(TextPrimaryStyle.Render("Claude wants to run "))
	content.WriteString(PermissionToolStyle.Render(req.Tool))
	
	if req.FilePath != "" {
		content.WriteString(" on ")
		content.WriteString(PermissionPathStyle.Render(req.FilePath))
	} else if req.Command != "" {
		content.WriteString(": ")
		content.WriteString(TextSecondaryStyle.Render(truncate(req.Command, 60)))
	}
	content.WriteString("\n\n")

	// Content Preview (Diff or New Content)
	if req.IsDiff && (req.OldContent != "" || req.NewContent != "") {
		content.WriteString(m.renderDiff(req.OldContent, req.NewContent))
		content.WriteString("\n")
	} else if req.NewContent != "" {
		// Preview for Write
		lines := strings.Split(req.NewContent, "\n")
		maxLines := 8
		for i, line := range lines {
			if i >= maxLines {
				content.WriteString(TextMutedStyle.Render(fmt.Sprintf("  ... and %d more lines", len(lines)-maxLines)))
				break
			}
			content.WriteString(DiffAddStyle.Render("+ " + line))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	// Options
	options := []struct {
		key   string
		label string
		desc  string
	}{
		{"y", "Yes", "Allow this operation"},
		{"a", "Always", "Allow all " + req.Tool + " operations for this session"},
		{"n", "No", "Reject operation"},
	}

	for i, opt := range options {
		content.WriteString(RenderPermissionOption(opt.key, opt.label, opt.desc, i == m.permissionChoice))
		content.WriteString("\n")
	}

	// Feedback input
	if m.rejectingWithInput {
		content.WriteString("\n")
		content.WriteString(TextMutedStyle.Render("Reason: "))
		content.WriteString(m.textarea.View())
	}

	return PermissionBoxStyle.Render(content.String())
}

func (m Model) renderDiff(oldContent, newContent string) string {
	var result strings.Builder
	
	// Using the helper from styles.go logic but implementing simple diff here
	// In a real scenario, use a diff library. Here we just show lines.
	
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	
	// Very basic comparison display
	maxLines := 10
	
	if len(oldLines) > 0 {
		for i, line := range oldLines {
			if i >= maxLines { break }
			result.WriteString(DiffRemoveStyle.Render("- " + line))
			result.WriteString("\n")
		}
	}
	
	if len(newLines) > 0 {
		for i, line := range newLines {
			if i >= maxLines { break }
			result.WriteString(DiffAddStyle.Render("+ " + line))
			result.WriteString("\n")
		}
	}
	
	return result.String()
}

func (m Model) renderError() string {
	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(RenderError(m.errorMsg))
}

func (m Model) renderInput() string {
	var prompt string
	
	if m.state == StateProcessing {
		// Show spinner with dynamic verb
		verb := m.GetCurrentVerb()
		prompt = RenderSpinnerWithVerb(m.spinner.View(), verb)
	} else {
		// Standard prompt
		prompt = InputPromptStyle.Render("> ")
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Render(prompt + InputTextStyle.Render(m.textarea.Value()))
}

func (m Model) renderStatusBar() string {
	// Status Line: Model | Tokens | Cost
	
	// Left: Model
	modelDisplay := m.model
	if strings.HasPrefix(modelDisplay, "claude-") {
		parts := strings.Split(modelDisplay, "-")
		if len(parts) >= 2 {
			modelDisplay = "Claude " + strings.Title(parts[1])
		}
	}
	
	return RenderStatusLine(modelDisplay, m.tokens, m.width)
}