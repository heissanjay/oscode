package setup

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/heissanjay/oscode/internal/config"
)

// Colors matching OSCode theme
var (
	colorCrail      = lipgloss.Color("#C15F3C")
	colorTextMuted  = lipgloss.Color("#78716C")
	colorTextPrimary = lipgloss.Color("#F5F5F4")
	colorSuccess    = lipgloss.Color("#16A34A")
	colorBorder     = lipgloss.Color("#44403C")
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
		Foreground(colorCrail).
		Bold(true)

	subtitleStyle = lipgloss.NewStyle().
		Foreground(colorTextMuted)

	optionStyle = lipgloss.NewStyle().
		Foreground(colorTextPrimary)

	selectedStyle = lipgloss.NewStyle().
		Foreground(colorCrail).
		Bold(true)

	hintStyle = lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Italic(true)

	successStyle = lipgloss.NewStyle().
		Foreground(colorSuccess)

	boxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2)
)

// Step represents the current setup step
type Step int

const (
	StepWelcome Step = iota
	StepProvider
	StepAPIKey
	StepDone
)

// Result holds the setup result
type Result struct {
	Skipped  bool
	Provider string
	APIKey   string
}

// Model represents the setup wizard state
type Model struct {
	step     Step
	provider string
	apiKey   textinput.Model
	config   *config.Config
	width    int
	height   int
	result   Result
	quitting bool
	err      error
}

// NewModel creates a new setup wizard model
func NewModel(cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "sk-..."
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.CharLimit = 256
	ti.Width = 50

	// Style the input
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorCrail)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorTextPrimary)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorTextMuted)

	return Model{
		step:     StepWelcome,
		provider: "anthropic",
		apiKey:   ti,
		config:   cfg,
		width:    80,
		height:   24,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			m.result.Skipped = true
			return m, tea.Quit

		case "s", "S":
			if m.step == StepWelcome || m.step == StepProvider {
				m.result.Skipped = true
				return m, tea.Quit
			}

		case "1":
			if m.step == StepProvider {
				m.provider = "anthropic"
				m.step = StepAPIKey
				m.apiKey.Focus()
				return m, textinput.Blink
			}

		case "2":
			if m.step == StepProvider {
				m.provider = "openai"
				m.step = StepAPIKey
				m.apiKey.Focus()
				return m, textinput.Blink
			}

		case "enter":
			switch m.step {
			case StepWelcome:
				m.step = StepProvider
				return m, nil

			case StepAPIKey:
				if m.apiKey.Value() != "" {
					m.result.Provider = m.provider
					m.result.APIKey = m.apiKey.Value()

					// Save to config
					if err := m.saveConfig(); err != nil {
						m.err = err
						return m, nil
					}

					m.step = StepDone
					return m, tea.Quit
				}

			case StepDone:
				return m, tea.Quit
			}

		case "esc":
			if m.step == StepAPIKey {
				m.step = StepProvider
				m.apiKey.Reset()
				return m, nil
			}
			if m.step == StepProvider {
				m.step = StepWelcome
				return m, nil
			}
		}
	}

	// Update text input if in API key step
	if m.step == StepAPIKey {
		var cmd tea.Cmd
		m.apiKey, cmd = m.apiKey.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the UI
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var content strings.Builder

	switch m.step {
	case StepWelcome:
		content.WriteString(m.renderWelcome())
	case StepProvider:
		content.WriteString(m.renderProviderSelection())
	case StepAPIKey:
		content.WriteString(m.renderAPIKeyInput())
	case StepDone:
		content.WriteString(m.renderDone())
	}

	return content.String()
}

func (m Model) renderWelcome() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("  Welcome to OSCode!"))
	sb.WriteString("\n\n")
	sb.WriteString(subtitleStyle.Render("  An AI-powered coding assistant for your terminal."))
	sb.WriteString("\n\n")
	sb.WriteString(optionStyle.Render("  Let's get you set up with an LLM provider."))
	sb.WriteString("\n\n")
	sb.WriteString(hintStyle.Render("  Press Enter to continue, or S to skip setup"))
	sb.WriteString("\n")

	return sb.String()
}

func (m Model) renderProviderSelection() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("  Choose your LLM provider:"))
	sb.WriteString("\n\n")

	providers := []struct {
		key   string
		name  string
		desc  string
	}{
		{"1", "Anthropic", "Claude models (Recommended)"},
		{"2", "OpenAI", "GPT models"},
	}

	for _, p := range providers {
		prefix := "  "
		style := optionStyle
		if (p.key == "1" && m.provider == "anthropic") || (p.key == "2" && m.provider == "openai") {
			prefix = "  "
			style = selectedStyle
		}

		sb.WriteString(prefix)
		sb.WriteString(selectedStyle.Render("[" + p.key + "] "))
		sb.WriteString(style.Render(p.name))
		sb.WriteString(subtitleStyle.Render(" - " + p.desc))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("  Press 1 or 2 to select, S to skip, Esc to go back"))
	sb.WriteString("\n")

	return sb.String()
}

func (m Model) renderAPIKeyInput() string {
	var sb strings.Builder

	providerName := "Anthropic"
	keyHint := "Get key: console.anthropic.com"
	if m.provider == "openai" {
		providerName = "OpenAI"
		keyHint = "Get key: platform.openai.com/api-keys"
	}

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("  Enter your " + providerName + " API Key:"))
	sb.WriteString("\n\n")
	sb.WriteString("  ")
	sb.WriteString(m.apiKey.View())
	sb.WriteString("\n\n")
	sb.WriteString(hintStyle.Render("  " + keyHint))
	sb.WriteString("\n\n")

	if m.err != nil {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#DC2626")).Render("  Error: " + m.err.Error()))
		sb.WriteString("\n\n")
	}

	sb.WriteString(hintStyle.Render("  Press Enter to save, Esc to go back"))
	sb.WriteString("\n")

	return sb.String()
}

func (m Model) renderDone() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(successStyle.Render("  ✓ Setup complete!"))
	sb.WriteString("\n\n")
	sb.WriteString(subtitleStyle.Render("  Your API key has been saved to: " + config.GetConfigPath()))
	sb.WriteString("\n\n")
	sb.WriteString(optionStyle.Render("  You're ready to use OSCode."))
	sb.WriteString("\n")

	return sb.String()
}

func (m *Model) saveConfig() error {
	// Update provider config
	providerConfig := m.config.Providers[m.provider]
	providerConfig.APIKey = m.result.APIKey
	m.config.Providers[m.provider] = providerConfig

	// Set as default provider
	m.config.DefaultProvider = m.provider

	// Set default model based on provider
	if m.provider == "anthropic" {
		m.config.DefaultModel = "claude-sonnet-4-20250514"
	} else {
		m.config.DefaultModel = "gpt-4o"
	}

	return config.Save(m.config)
}

// GetResult returns the setup result
func (m Model) GetResult() Result {
	return m.result
}

// NeedsSetup checks if setup is needed
func NeedsSetup(cfg *config.Config) bool {
	// Check if any provider has a valid API key
	for _, provider := range cfg.Providers {
		apiKey := provider.APIKey
		// Skip placeholder/template values
		if apiKey != "" &&
		   !strings.HasPrefix(apiKey, "${") &&
		   apiKey != "your-api-key-here" {
			return false
		}
	}
	return true
}

// Run runs the setup wizard and returns the result
func Run(cfg *config.Config) (Result, error) {
	m := NewModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return Result{Skipped: true}, err
	}

	return finalModel.(Model).GetResult(), nil
}
