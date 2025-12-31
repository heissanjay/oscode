package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderMarkdown renders markdown content with code block styling
func RenderMarkdown(content string, width int) string {
	// Styles
	codeBlockStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1F2937")).
		Foreground(lipgloss.Color("#E5E7EB")).
		Padding(0, 1)

	codeHeaderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Background(lipgloss.Color("#1F2937")).
		Padding(0, 1)

	inlineCodeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#374151")).
		Foreground(lipgloss.Color("#F9FAFB")).
		Padding(0, 1)

	boldStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F9FAFB"))

	// Process code blocks first (```language ... ```)
	codeBlockRegex := regexp.MustCompile("(?s)```(\\w*)\\n?(.*?)```")
	content = codeBlockRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := codeBlockRegex.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}
		lang := matches[1]
		code := strings.TrimSpace(matches[2])

		var result strings.Builder
		result.WriteString("\n")
		if lang != "" {
			result.WriteString(codeHeaderStyle.Render(" " + lang + " "))
			result.WriteString("\n")
		}

		// Render code lines with background
		lines := strings.Split(code, "\n")
		maxLen := 0
		for _, line := range lines {
			if len(line) > maxLen {
				maxLen = len(line)
			}
		}
		if maxLen < width-4 {
			maxLen = width - 4
		}

		for _, line := range lines {
			// Pad line to consistent width
			padded := line + strings.Repeat(" ", maxLen-len(line))
			result.WriteString(codeBlockStyle.Render(padded))
			result.WriteString("\n")
		}
		return result.String()
	})

	// Process inline code (`code`)
	inlineCodeRegex := regexp.MustCompile("`([^`]+)`")
	content = inlineCodeRegex.ReplaceAllStringFunc(content, func(match string) string {
		code := strings.Trim(match, "`")
		return inlineCodeStyle.Render(code)
	})

	// Process bold (**text** or __text__)
	boldRegex := regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`)
	content = boldRegex.ReplaceAllStringFunc(content, func(match string) string {
		text := strings.Trim(match, "*_")
		return boldStyle.Render(text)
	})

	// Process bullet points
	lines := strings.Split(content, "\n")
	var result []string
	bulletStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		indent := line[:len(line)-len(trimmed)]

		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			result = append(result, indent+bulletStyle.Render("•")+" "+trimmed[2:])
		} else if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && trimmed[1] == '.' {
			// Numbered list
			result = append(result, indent+bulletStyle.Render(trimmed[:2])+" "+trimmed[3:])
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}
