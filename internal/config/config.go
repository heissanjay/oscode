package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Load loads the configuration from all sources with proper precedence
func Load() (*Config, error) {
	// Ensure config directories exist
	if err := EnsureConfigDirs(); err != nil {
		return nil, fmt.Errorf("failed to create config directories: %w", err)
	}

	// Start with default config
	cfg := DefaultConfig()

	// Load user settings (~/.oscode/settings.json)
	if err := loadConfigFile(GetConfigPath(), cfg); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load user config: %w", err)
	}

	// Load project settings (.oscode/settings.json)
	cwd, _ := os.Getwd()
	projectConfig := GetProjectSettingsPath(cwd)
	if err := loadConfigFile(projectConfig, cfg); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load project config: %w", err)
	}

	// Load project local settings (.oscode/settings.local.json)
	localConfig := GetProjectLocalSettingsPath(cwd)
	if err := loadConfigFile(localConfig, cfg); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load local config: %w", err)
	}

	// Resolve environment variables in config
	resolveEnvVars(cfg)

	return cfg, nil
}

// loadConfigFile loads a JSON config file and merges it into the config
func loadConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Parse into a map first for merging
	var fileConfig map[string]interface{}
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("invalid JSON in %s: %w", path, err)
	}

	// Use viper for merging
	v := viper.New()
	v.SetConfigType("json")
	if err := v.MergeConfigMap(fileConfig); err != nil {
		return err
	}

	// Unmarshal back to struct
	return v.Unmarshal(cfg)
}

// resolveEnvVars resolves ${VAR} and ${VAR:-default} patterns in config values
func resolveEnvVars(cfg *Config) {
	// Resolve provider API keys
	for name, provider := range cfg.Providers {
		provider.APIKey = expandEnvVar(provider.APIKey)
		provider.BaseURL = expandEnvVar(provider.BaseURL)
		cfg.Providers[name] = provider
	}

	// Resolve MCP server configurations
	for name, server := range cfg.MCP.Servers {
		server.URL = expandEnvVar(server.URL)
		server.Command = expandEnvVar(server.Command)
		for i, arg := range server.Args {
			server.Args[i] = expandEnvVar(arg)
		}
		for key, val := range server.Env {
			server.Env[key] = expandEnvVar(val)
		}
		cfg.MCP.Servers[name] = server
	}
}

// expandEnvVar expands ${VAR} and ${VAR:-default} patterns
func expandEnvVar(s string) string {
	if !strings.HasPrefix(s, "${") {
		return s
	}

	// Handle ${VAR:-default} pattern
	s = strings.TrimPrefix(s, "${")
	s = strings.TrimSuffix(s, "}")

	parts := strings.SplitN(s, ":-", 2)
	varName := parts[0]
	defaultVal := ""
	if len(parts) > 1 {
		defaultVal = parts[1]
	}

	if val := os.Getenv(varName); val != "" {
		return val
	}
	return defaultVal
}

// Save saves the current configuration to the user config file
func Save(cfg *Config) error {
	path := GetConfigPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetAPIKey returns the API key for a provider
func (c *Config) GetAPIKey(provider string) string {
	if p, ok := c.Providers[provider]; ok {
		return p.APIKey
	}
	return ""
}

// GetBaseURL returns the base URL for a provider
func (c *Config) GetBaseURL(provider string) string {
	if p, ok := c.Providers[provider]; ok {
		return p.BaseURL
	}
	return ""
}

// GetModel returns the resolved model name
func (c *Config) GetModel() string {
	return ResolveModel(c.DefaultProvider, c.DefaultModel)
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.DefaultProvider == "" {
		return fmt.Errorf("no default provider configured")
	}

	apiKey := c.GetAPIKey(c.DefaultProvider)
	if apiKey == "" {
		return fmt.Errorf("no API key configured for provider %s", c.DefaultProvider)
	}

	return nil
}

// CreateDefaultConfigFile creates a default config file if it doesn't exist
func CreateDefaultConfigFile() error {
	path := GetConfigPath()

	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		return nil // File exists
	}

	cfg := DefaultConfig()

	// Don't save actual API keys in default config
	cfg.Providers["anthropic"] = ProviderConfig{
		APIKey: "${ANTHROPIC_API_KEY}",
	}
	cfg.Providers["openai"] = ProviderConfig{
		APIKey: "${OPENAI_API_KEY}",
	}

	return Save(cfg)
}
