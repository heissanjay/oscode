package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// Claude Code Color Palette
var (
	// Primary Brand Color - "Crail" (Claude's signature orange)
	ColorCrail = lipgloss.Color("#C15F3C")

	// Secondary/Info - "Cloudy" (for metadata, timestamps)
	ColorCloudy = lipgloss.Color("#B1ADA1")

	// Background/Base - "Pampas" (off-white for light mode elements)
	ColorPampas = lipgloss.Color("#F4F3EE")

	// Token/Cost indicator - Orange
	ColorTokenOrange = lipgloss.Color("#E07A3A")

	// Heavy actions/errors - Red
	ColorHeavyRed = lipgloss.Color("#DC2626")

	// Success - Green
	ColorSuccess = lipgloss.Color("#16A34A")

	// Code/Syntax - Blue
	ColorCode = lipgloss.Color("#60A5FA")

	// Text colors for dark terminal
	ColorTextPrimary   = lipgloss.Color("#F5F5F4")
	ColorTextSecondary = lipgloss.Color("#A8A29E")
	ColorTextMuted     = lipgloss.Color("#78716C")

	// Diff colors (ANSI standard)
	ColorDiffAdd    = lipgloss.Color("#22C55E") // Green for additions
	ColorDiffRemove = lipgloss.Color("#EF4444") // Red for deletions
	ColorDiffHunk   = lipgloss.Color("#60A5FA") // Blue for hunk headers

	// Border/Separator
	ColorBorder = lipgloss.Color("#44403C")
)

// Claude Sparkle Spinner - the magical thinking animation
func ClaudeSpinner() spinner.Spinner {
	return spinner.Spinner{
		Frames: []string{"✢", "✶", "✻", "✽", "※", "✽", "✻", "✶"},
		FPS:    time.Second / 10, // Slow, deliberate pulse
	}
}

// SpinnerVerbs - context-aware dynamic verbs for the spinner
var SpinnerVerbs = map[string]string{
	"default":    "Thinking",
	"read":       "Reading",
	"write":      "Writing",
	"edit":       "Editing",
	"search":     "Exploring",
	"bash":       "Executing",
	"plan":       "Planning",
	"analyze":    "Analyzing",
	"discover":   "Discovering",
	"implement":  "Implementing",
	"review":     "Reviewing",
	"summarize":  "Summarizing",
	"initialize": "Initializing",
}

// GetSpinnerVerb returns the appropriate verb for a tool/action
func GetSpinnerVerb(action string) string {
	if verb, ok := SpinnerVerbs[action]; ok {
		return verb
	}
	return SpinnerVerbs["default"]
}

// === STYLES ===

// Claude Label Style - Bold brand name
var ClaudeLabelStyle = lipgloss.NewStyle().
	Foreground(ColorCrail).
	Bold(true)

// User Label Style
var UserLabelStyle = lipgloss.NewStyle().
	Foreground(ColorTextPrimary).
	Bold(true)

// Text Styles
var (
	TextPrimaryStyle = lipgloss.NewStyle().
				Foreground(ColorTextPrimary)

	TextSecondaryStyle = lipgloss.NewStyle().
				Foreground(ColorTextSecondary)

	TextMutedStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	TextCloudyStyle = lipgloss.NewStyle().
			Foreground(ColorCloudy)
)

// Input Styles
var (
	InputPromptStyle = lipgloss.NewStyle().
				Foreground(ColorCrail).
				Bold(true)

	InputCaretStyle = lipgloss.NewStyle().
			Foreground(ColorCrail)

	InputTextStyle = lipgloss.NewStyle().
			Foreground(ColorTextPrimary)

	InputPlaceholderStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted).
				Italic(true)
)

// Message Styles
var (
	UserMessageStyle = lipgloss.NewStyle().
				Foreground(ColorTextPrimary)

	AssistantMessageStyle = lipgloss.NewStyle().
				Foreground(ColorTextPrimary)

	SystemMessageStyle = lipgloss.NewStyle().
				Foreground(ColorCloudy).
				Italic(true)
)

// Tool Execution Styles
var (
	ToolNameStyle = lipgloss.NewStyle().
			Foreground(ColorCode).
			Bold(true)

	ToolSpinnerStyle = lipgloss.NewStyle().
				Foreground(ColorCrail)

	ToolVerbStyle = lipgloss.NewStyle().
			Foreground(ColorCloudy).
			Italic(true)

	ToolSuccessStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	ToolErrorStyle = lipgloss.NewStyle().
			Foreground(ColorHeavyRed)
)

// Status Line Styles - "Oh My Posh" inspired minimal status bar
var (
	StatusLineStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	StatusModelStyle = lipgloss.NewStyle().
				Foreground(ColorCrail).
				Bold(true)

	StatusTokenStyle = lipgloss.NewStyle().
				Foreground(ColorTokenOrange)

	StatusSeparatorStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)

	StatusIndicatorStyle = lipgloss.NewStyle().
				Foreground(ColorCrail)
)

// Permission Prompt Styles
var (
	PermissionBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorCrail).
				Padding(1, 2)

	PermissionTitleStyle = lipgloss.NewStyle().
				Foreground(ColorCrail).
				Bold(true)

	PermissionToolStyle = lipgloss.NewStyle().
				Foreground(ColorCode).
				Bold(true)

	PermissionPathStyle = lipgloss.NewStyle().
				Foreground(ColorTextSecondary)

	PermissionKeyStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	PermissionSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorCrail).
				Bold(true)

	PermissionDescStyle = lipgloss.NewStyle().
				Foreground(ColorCloudy)
)

// Diff Styles - Standard ANSI colors
var (
	DiffAddStyle = lipgloss.NewStyle().
			Foreground(ColorDiffAdd)

	DiffRemoveStyle = lipgloss.NewStyle().
			Foreground(ColorDiffRemove)

	DiffHunkStyle = lipgloss.NewStyle().
			Foreground(ColorDiffHunk)

	DiffContextStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted)

	DiffLineNumStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted).
				Width(4)
)

// Error & Success Styles
var (
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorHeavyRed)

	ErrorIconStyle = lipgloss.NewStyle().
			Foreground(ColorHeavyRed).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	SuccessIconStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorTokenOrange)
)

// Code Block Styles
var (
	CodeBlockStyle = lipgloss.NewStyle().
			Foreground(ColorTextPrimary).
			Background(lipgloss.Color("#1C1917")).
			Padding(1, 2)

	InlineCodeStyle = lipgloss.NewStyle().
			Foreground(ColorCode).
			Background(lipgloss.Color("#292524")).
			Padding(0, 1)
)

// Markdown Header Styles
var (
	MarkdownH1Style = lipgloss.NewStyle().
			Foreground(ColorTextPrimary).
			Bold(true).
			Underline(true).
			MarginTop(1).
			MarginBottom(1)

	MarkdownH2Style = lipgloss.NewStyle().
			Foreground(ColorTextPrimary).
			Bold(true).
			MarginTop(1)

	MarkdownH3Style = lipgloss.NewStyle().
			Foreground(ColorTextSecondary).
			Bold(true)

	MarkdownLinkStyle = lipgloss.NewStyle().
				Foreground(ColorCode).
				Underline(true)

	MarkdownBoldStyle = lipgloss.NewStyle().
				Bold(true)

	MarkdownItalicStyle = lipgloss.NewStyle().
				Italic(true)
)

// Selection/Menu Styles
var (
	SelectionBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(1, 2)

	SelectionItemStyle = lipgloss.NewStyle().
				Foreground(ColorTextSecondary)

	SelectionSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorCrail).
				Bold(true)

	SelectionDescStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted)
)

// Help Styles
var (
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorCrail).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorCloudy)

	HelpSeparatorStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)
)

// === ICONS ===
const (
	IconSuccess   = "✓"
	IconError     = "✖"
	IconWarning   = "⚠"
	IconInfo      = "ℹ"
	IconArrow     = "→"
	IconBullet    = "•"
	IconChevron   = "›"
	IconCheck     = "✔"
	IconCross     = "✘"
	IconSpinner   = "◌"
	IconSelected  = "▶"
	IconUnselected = " "
	IconTokens    = "↓"
	IconCost      = "$"
)

// === HELPER FUNCTIONS ===

// RenderClaudeLabel renders the "Claude" sender label
func RenderClaudeLabel() string {
	return ClaudeLabelStyle.Render("Claude")
}

// RenderUserLabel renders the "You" sender label
func RenderUserLabel() string {
	return UserLabelStyle.Render("You")
}

// RenderSuccess renders a success message with checkmark
func RenderSuccess(msg string) string {
	return SuccessIconStyle.Render(IconSuccess) + " " + SuccessStyle.Render(msg)
}

// RenderError renders an error message with X mark
func RenderError(msg string) string {
	return ErrorIconStyle.Render(IconError) + " " + ErrorStyle.Render("Error: "+msg)
}

// RenderWarning renders a warning message
func RenderWarning(msg string) string {
	return WarningStyle.Render(IconWarning + " " + msg)
}

// RenderSpinnerWithVerb renders the sparkle spinner with a dynamic verb
func RenderSpinnerWithVerb(frame string, verb string) string {
	return ToolSpinnerStyle.Render(frame) + " " + ToolVerbStyle.Render(verb+"...")
}

// RenderToolStatus renders a tool execution status line
func RenderToolStatus(icon, toolName string, isRunning bool) string {
	var iconStyle lipgloss.Style
	if isRunning {
		iconStyle = ToolSpinnerStyle
	} else {
		iconStyle = ToolSuccessStyle
	}
	return "  " + iconStyle.Render(icon) + " " + ToolNameStyle.Render(toolName)
}

// RenderStatusLine renders the bottom status line
func RenderStatusLine(model string, tokens int, width int) string {
	// Left side: model name
	modelPart := StatusModelStyle.Render(model)

	// Right side: token count
	tokenPart := StatusTokenStyle.Render(fmt.Sprintf("%s %s", IconTokens, FormatTokenCount(tokens)))

	// Calculate padding
	leftWidth := lipgloss.Width(modelPart)
	rightWidth := lipgloss.Width(tokenPart)
	padding := width - leftWidth - rightWidth - 2

	if padding < 1 {
		padding = 1
	}

	// Build status line
	sep := StatusSeparatorStyle.Render(" · ")
	spaces := ""
	for i := 0; i < padding; i++ {
		spaces += " "
	}

	return StatusLineStyle.Render(modelPart + spaces + sep + tokenPart)
}

// FormatTokenCount formats token count for display
func FormatTokenCount(tokens int) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d tok", tokens)
	}
	if tokens < 1000000 {
		return fmt.Sprintf("%.1fk tok", float64(tokens)/1000)
	}
	return fmt.Sprintf("%.1fM tok", float64(tokens)/1000000)
}

// RenderDiffLine renders a single diff line with appropriate coloring
func RenderDiffLine(line string) string {
	if len(line) == 0 {
		return ""
	}

	switch line[0] {
	case '+':
		return DiffAddStyle.Render(line)
	case '-':
		return DiffRemoveStyle.Render(line)
	case '@':
		return DiffHunkStyle.Render(line)
	default:
		return DiffContextStyle.Render(line)
	}
}

// RenderPermissionOption renders a permission option with selection indicator
func RenderPermissionOption(key, label, desc string, selected bool) string {
	var prefix, labelStyle string
	if selected {
		prefix = PermissionSelectedStyle.Render(IconSelected + " ")
		labelStyle = PermissionSelectedStyle.Render(label)
	} else {
		prefix = "  "
		labelStyle = TextSecondaryStyle.Render(label)
	}

	keyPart := PermissionKeyStyle.Render("[" + key + "]")
	descPart := PermissionDescStyle.Render(" - " + desc)

	return prefix + keyPart + " " + labelStyle + descPart
}

// RenderWelcome renders the welcome screen
func RenderWelcome() string {
	title := ClaudeLabelStyle.Render("Claude Code")
	subtitle := TextMutedStyle.Render("What can I help you with?")

	return lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		title,
		"",
		subtitle,
		"",
	)
}

// RenderHelp renders the help screen
func RenderHelp() string {
	titleStyle := TextPrimaryStyle.Bold(true)
	var content string

	content += titleStyle.Render("Keyboard Shortcuts") + "\n\n"

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
		content += fmt.Sprintf("  %s  %s\n",
			HelpKeyStyle.Render(fmt.Sprintf("%-12s", s.key)),
			HelpDescStyle.Render(s.desc))
	}

	content += "\n" + titleStyle.Render("Commands") + "\n\n"

	commands := []struct {
		cmd  string
		desc string
	}{
		{"/help", "Show help"},
		{"/clear", "Clear conversation"},
		{"/model", "Switch model"},
		{"/init", "Create CLAUDE.md"},
		{"/compact", "Compact conversation"},
		{"/review", "Code review mode"},
		{"/exit", "Exit"},
	}

	for _, c := range commands {
		content += fmt.Sprintf("  %s  %s\n",
			HelpKeyStyle.Render(fmt.Sprintf("%-12s", c.cmd)),
			HelpDescStyle.Render(c.desc))
	}

	return SelectionBoxStyle.Render(content)
}
