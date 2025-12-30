package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Colors - Claude Code inspired color palette
var (
	// Primary colors
	ColorPrimary    = lipgloss.Color("#D97706") // Orange/amber accent
	ColorSecondary  = lipgloss.Color("#6366F1") // Indigo
	ColorSuccess    = lipgloss.Color("#10B981") // Green
	ColorWarning    = lipgloss.Color("#F59E0B") // Yellow
	ColorError      = lipgloss.Color("#EF4444") // Red
	ColorInfo       = lipgloss.Color("#3B82F6") // Blue

	// Text colors
	ColorText        = lipgloss.Color("#E5E7EB") // Light gray
	ColorTextMuted   = lipgloss.Color("#9CA3AF") // Muted gray
	ColorTextDim     = lipgloss.Color("#6B7280") // Dim gray
	ColorTextBright  = lipgloss.Color("#F9FAFB") // Bright white

	// Background colors
	ColorBg          = lipgloss.Color("#111827") // Dark background
	ColorBgSecondary = lipgloss.Color("#1F2937") // Slightly lighter
	ColorBgHighlight = lipgloss.Color("#374151") // Highlight

	// Border colors
	ColorBorder      = lipgloss.Color("#374151")
	ColorBorderFocus = lipgloss.Color("#D97706")
)

// Styles
var (
	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	// Header styles
	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorTextBright).
			Bold(true)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	// Status bar styles
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Background(ColorBgSecondary).
			Padding(0, 1)

	StatusItemStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Padding(0, 1)

	StatusHighlightStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true)

	// Input styles
	InputPromptStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true)

	InputTextStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	InputPlaceholderStyle = lipgloss.NewStyle().
				Foreground(ColorTextDim).
				Italic(true)

	// Message styles
	UserMessageStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				PaddingLeft(2)

	AssistantMessageStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				PaddingLeft(2)

	SystemMessageStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted).
				Italic(true).
				PaddingLeft(2)

	// Tool execution styles
	ToolNameStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	ToolDescStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Italic(true)

	ToolSpinnerStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary)

	ToolResultStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			PaddingLeft(4)

	// Code block styles
	CodeBlockStyle = lipgloss.NewStyle().
			Background(ColorBgSecondary).
			Foreground(ColorText).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	InlineCodeStyle = lipgloss.NewStyle().
			Background(ColorBgSecondary).
			Foreground(ColorPrimary).
			Padding(0, 1)

	// Permission prompt styles
	PermissionBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorWarning).
				Padding(1, 2).
				MarginTop(1).
				MarginBottom(1)

	PermissionTitleStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	PermissionCommandStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(ColorBgSecondary).
				Padding(0, 1)

	PermissionKeyStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	// Error styles
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	ErrorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorError).
			Foreground(ColorError).
			Padding(1, 2)

	// Success styles
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	// Warning styles
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	// Info styles
	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	// Spinner characters
	SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	FocusedBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorderFocus).
			Padding(1, 2)

	// Help styles
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	// Diff styles
	DiffAddStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	DiffRemoveStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	DiffContextStyle = lipgloss.NewStyle().
				Foreground(ColorTextDim)

	// Markdown styles
	MarkdownH1Style = lipgloss.NewStyle().
			Foreground(ColorTextBright).
			Bold(true).
			Underline(true)

	MarkdownH2Style = lipgloss.NewStyle().
			Foreground(ColorTextBright).
			Bold(true)

	MarkdownH3Style = lipgloss.NewStyle().
			Foreground(ColorText).
			Bold(true)

	MarkdownLinkStyle = lipgloss.NewStyle().
				Foreground(ColorInfo).
				Underline(true)

	MarkdownBoldStyle = lipgloss.NewStyle().
				Bold(true)

	MarkdownItalicStyle = lipgloss.NewStyle().
				Italic(true)

	// Logo/branding
	LogoStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	VersionStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim)
)

// Width returns a style with a fixed width
func Width(style lipgloss.Style, width int) lipgloss.Style {
	return style.Width(width)
}

// MaxWidth returns a style with a maximum width
func MaxWidth(style lipgloss.Style, width int) lipgloss.Style {
	return style.MaxWidth(width)
}

// Centered returns a style that centers content
func Centered(style lipgloss.Style) lipgloss.Style {
	return style.Align(lipgloss.Center)
}

// RenderLogo returns the rendered application logo
func RenderLogo() string {
	logo := `
 ___  ___  ___ ___  ___  ___
/ _ \/ __|/ __/ _ \/ _ \/ _ \
\___/\__ \ (_| (_) \___/\___/
    |___/\___\___/           `
	return LogoStyle.Render(logo)
}

// RenderWelcome returns the welcome message
func RenderWelcome(version string) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		RenderLogo(),
		"",
		SubtitleStyle.Render("AI-powered coding assistant"),
		VersionStyle.Render("Version "+version),
		"",
		HelpDescStyle.Render("Type your message or use /help for commands"),
		"",
	)
}

// RenderKeyBinding renders a key binding hint
func RenderKeyBinding(key, desc string) string {
	return HelpKeyStyle.Render(key) + " " + HelpDescStyle.Render(desc)
}

// RenderStatusBar renders the status bar
func RenderStatusBar(provider, model string, tokens int, width int) string {
	left := StatusItemStyle.Render(provider + "/" + model)
	right := StatusItemStyle.Render(formatTokens(tokens))

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	spaces := ""
	for i := 0; i < gap; i++ {
		spaces += " "
	}

	return StatusBarStyle.Width(width).Render(left + spaces + right)
}

func formatTokens(tokens int) string {
	return lipgloss.NewStyle().Render("tokens: ") + StatusHighlightStyle.Render(formatNumber(tokens))
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}
