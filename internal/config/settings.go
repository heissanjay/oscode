package config

// Config represents the complete application configuration
type Config struct {
	// Default provider and model
	DefaultProvider string `json:"defaultProvider" mapstructure:"defaultProvider"`
	DefaultModel    string `json:"defaultModel" mapstructure:"defaultModel"`

	// Provider configurations
	Providers map[string]ProviderConfig `json:"providers" mapstructure:"providers"`

	// Permission settings
	Permissions PermissionConfig `json:"permissions" mapstructure:"permissions"`

	// Hook configurations
	Hooks HookConfig `json:"hooks" mapstructure:"hooks"`

	// MCP server configurations
	MCP MCPConfig `json:"mcp" mapstructure:"mcp"`

	// UI settings
	UI UIConfig `json:"ui" mapstructure:"ui"`

	// Session settings
	CleanupPeriodDays int `json:"cleanupPeriodDays" mapstructure:"cleanupPeriodDays"`

	// Runtime flags (not persisted)
	Verbose        bool   `json:"-" mapstructure:"-"`
	SystemPrompt   string `json:"-" mapstructure:"-"`
	PermissionMode string `json:"-" mapstructure:"-"`
}

// ProviderConfig contains settings for an LLM provider
type ProviderConfig struct {
	APIKey  string `json:"apiKey" mapstructure:"apiKey"`
	BaseURL string `json:"baseURL" mapstructure:"baseURL"`
	OrgID   string `json:"orgId" mapstructure:"orgId"`

	// Provider-specific options
	Options map[string]interface{} `json:"options" mapstructure:"options"`
}

// PermissionConfig defines permission rules
type PermissionConfig struct {
	// Rules that auto-allow tools
	Allow []string `json:"allow" mapstructure:"allow"`

	// Rules that require asking
	Ask []string `json:"ask" mapstructure:"ask"`

	// Rules that auto-deny tools
	Deny []string `json:"deny" mapstructure:"deny"`

	// Additional directories to allow access
	AdditionalDirectories []string `json:"additionalDirectories" mapstructure:"additionalDirectories"`

	// Default permission mode
	DefaultMode string `json:"defaultMode" mapstructure:"defaultMode"`
}

// HookConfig contains hook definitions
type HookConfig struct {
	PreToolUse        []HookDefinition `json:"PreToolUse" mapstructure:"PreToolUse"`
	PostToolUse       []HookDefinition `json:"PostToolUse" mapstructure:"PostToolUse"`
	PermissionRequest []HookDefinition `json:"PermissionRequest" mapstructure:"PermissionRequest"`
	UserPromptSubmit  []HookDefinition `json:"UserPromptSubmit" mapstructure:"UserPromptSubmit"`
	SessionStart      []HookDefinition `json:"SessionStart" mapstructure:"SessionStart"`
	SessionEnd        []HookDefinition `json:"SessionEnd" mapstructure:"SessionEnd"`
	Notification      []HookDefinition `json:"Notification" mapstructure:"Notification"`
	Stop              []HookDefinition `json:"Stop" mapstructure:"Stop"`
}

// HookDefinition defines a single hook
type HookDefinition struct {
	Matcher string       `json:"matcher" mapstructure:"matcher"`
	Hooks   []HookAction `json:"hooks" mapstructure:"hooks"`
}

// HookAction defines what a hook does
type HookAction struct {
	Type    string `json:"type" mapstructure:"type"` // "command" or "prompt"
	Command string `json:"command" mapstructure:"command"`
	Prompt  string `json:"prompt" mapstructure:"prompt"`
}

// MCPConfig contains MCP server configurations
type MCPConfig struct {
	Servers map[string]MCPServerConfig `json:"mcpServers" mapstructure:"mcpServers"`
}

// MCPServerConfig defines an MCP server
type MCPServerConfig struct {
	Transport string            `json:"type" mapstructure:"type"` // "http", "sse", "stdio"
	URL       string            `json:"url" mapstructure:"url"`
	Command   string            `json:"command" mapstructure:"command"`
	Args      []string          `json:"args" mapstructure:"args"`
	Env       map[string]string `json:"env" mapstructure:"env"`
	Headers   map[string]string `json:"headers" mapstructure:"headers"`
}

// UIConfig contains UI settings
type UIConfig struct {
	Theme          string `json:"theme" mapstructure:"theme"`
	ShowTokenCount bool   `json:"showTokenCount" mapstructure:"showTokenCount"`
	ShowCost       bool   `json:"showCost" mapstructure:"showCost"`
	VimMode        bool   `json:"vimMode" mapstructure:"vimMode"`
	OutputStyle    string `json:"outputStyle" mapstructure:"outputStyle"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultProvider: "openai",
		DefaultModel:    "gpt-4o",
		Providers: map[string]ProviderConfig{
			"anthropic": {
				APIKey:  "${ANTHROPIC_API_KEY}",
				BaseURL: "",
			},
			"openai": {
				APIKey:  "${OPENAI_API_KEY}",
				BaseURL: "",
			},
		},
		Permissions: PermissionConfig{
			Allow:       []string{"Read", "Glob", "Grep", "Task", "Write", "Edit", "WebFetch", "WebSearch"},
			Ask:         []string{"Bash"},  // Only shell commands need approval
			Deny:        []string{},
			DefaultMode: "auto",  // Auto-allow safe operations
		},
		Hooks: HookConfig{},
		MCP: MCPConfig{
			Servers: make(map[string]MCPServerConfig),
		},
		UI: UIConfig{
			Theme:          "dark",
			ShowTokenCount: true,
			ShowCost:       true,
			VimMode:        false,
			OutputStyle:    "normal",
		},
		CleanupPeriodDays: 30,
	}
}

// ModelAliases maps short names to full model identifiers
var ModelAliases = map[string]map[string]string{
	"anthropic": {
		"opus":   "claude-opus-4-20250514",
		"sonnet": "claude-sonnet-4-20250514",
		"haiku":  "claude-haiku-3-5-20241022",
	},
	"openai": {
		"gpt4":    "gpt-4-turbo-preview",
		"gpt4o":   "gpt-4o",
		"gpt4o-mini": "gpt-4o-mini",
		"o1":      "o1-preview",
		"o1-mini": "o1-mini",
	},
}

// ResolveModel resolves a model alias to its full name
func ResolveModel(provider, model string) string {
	if aliases, ok := ModelAliases[provider]; ok {
		if fullName, ok := aliases[model]; ok {
			return fullName
		}
	}
	return model
}
