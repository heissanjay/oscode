package ui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// Claude Code Style JSON
const claudeStyleJSON = `
{
	"document": {
		"color": "#F5F5F4",
		"margin": 0
	},
	"h1": {
		"color": "#F5F5F4",
		"bold": true,
		"underline": true
	},
	"h2": {
		"color": "#F5F5F4",
		"bold": true
	},
	"h3": {
		"color": "#A8A29E",
		"bold": true
	},
	"link": {
		"color": "#60A5FA",
		"underline": true
	},
	"code": {
		"color": "#60A5FA",
		"background_color": "#292524"
	},
	"code_block": {
		"margin": 1,
		"theme": "dracula"
	},
	"list": {
		"color": "#F5F5F4"
	},
	"item": {
		"color": "#C15F3C"
	},
	"block_quote": {
		"color": "#78716C",
		"indent": 2
	}
}
`

// RenderMarkdown renders markdown text using glamour
func RenderMarkdown(content string, width int) string {
	// If content is empty, return empty string
	if strings.TrimSpace(content) == "" {
		return ""
	}

	// Create a new renderer with our custom style
	r, err := glamour.NewTermRenderer(
		glamour.WithStylesFromJSONBytes([]byte(claudeStyleJSON)),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// Fallback to simple rendering if glamour fails
		// Try standard style
		r, _ = glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(width),
		)
		out, _ := r.Render(content)
		return strings.TrimRight(out, "\n")
	}

	out, err := r.Render(content)
	if err != nil {
		return content
	}

	// Trim trailing newline which glamour adds
	return strings.TrimRight(out, "\n")
}
