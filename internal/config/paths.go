package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	AppName       = "oscode"
	ConfigFile    = "settings.json"
	MemoryFile    = "CLAUDE.md"
	LocalMemory   = "CLAUDE.local.md"
	SessionsDir   = "sessions"
	CommandsDir   = "commands"
	RulesDir      = "rules"
	MCPConfigFile = ".mcp.json"
)

// GetConfigDir returns the user's config directory for OSCode
func GetConfigDir() string {
	if dir := os.Getenv("OSCODE_CONFIG_DIR"); dir != "" {
		return dir
	}

	var configDir string
	switch runtime.GOOS {
	case "windows":
		configDir = os.Getenv("APPDATA")
		if configDir == "" {
			configDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, "Library", "Application Support")
	default: // linux and others
		configDir = os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, _ := os.UserHomeDir()
			configDir = filepath.Join(home, ".config")
		}
	}

	return filepath.Join(configDir, AppName)
}

// GetUserConfigDir returns ~/.oscode for user-level config
func GetUserConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "."+AppName)
}

// GetConfigPath returns the path to the main config file
func GetConfigPath() string {
	return filepath.Join(GetUserConfigDir(), ConfigFile)
}

// GetSessionsDir returns the path to the sessions directory
func GetSessionsDir() string {
	return filepath.Join(GetUserConfigDir(), SessionsDir)
}

// GetUserMemoryPath returns the path to the user's CLAUDE.md
func GetUserMemoryPath() string {
	return filepath.Join(GetUserConfigDir(), MemoryFile)
}

// GetUserCommandsDir returns the path to user's custom commands
func GetUserCommandsDir() string {
	return filepath.Join(GetUserConfigDir(), CommandsDir)
}

// GetUserRulesDir returns the path to user's rules
func GetUserRulesDir() string {
	return filepath.Join(GetUserConfigDir(), RulesDir)
}

// GetProjectConfigDir returns the .oscode directory in the current project
func GetProjectConfigDir(projectDir string) string {
	return filepath.Join(projectDir, "."+AppName)
}

// GetProjectSettingsPath returns the project-level settings.json path
func GetProjectSettingsPath(projectDir string) string {
	return filepath.Join(GetProjectConfigDir(projectDir), ConfigFile)
}

// GetProjectLocalSettingsPath returns the local (gitignored) settings path
func GetProjectLocalSettingsPath(projectDir string) string {
	return filepath.Join(GetProjectConfigDir(projectDir), "settings.local.json")
}

// GetProjectMemoryPath returns the project's CLAUDE.md path
func GetProjectMemoryPath(projectDir string) string {
	// Check .oscode/CLAUDE.md first, then CLAUDE.md in root
	oscodeMemory := filepath.Join(GetProjectConfigDir(projectDir), MemoryFile)
	if _, err := os.Stat(oscodeMemory); err == nil {
		return oscodeMemory
	}
	return filepath.Join(projectDir, MemoryFile)
}

// GetProjectLocalMemoryPath returns the project's local memory file
func GetProjectLocalMemoryPath(projectDir string) string {
	return filepath.Join(projectDir, LocalMemory)
}

// GetProjectCommandsDir returns the project's custom commands directory
func GetProjectCommandsDir(projectDir string) string {
	return filepath.Join(GetProjectConfigDir(projectDir), CommandsDir)
}

// GetProjectRulesDir returns the project's rules directory
func GetProjectRulesDir(projectDir string) string {
	return filepath.Join(GetProjectConfigDir(projectDir), RulesDir)
}

// GetMCPConfigPath returns the project's MCP config file path
func GetMCPConfigPath(projectDir string) string {
	return filepath.Join(projectDir, MCPConfigFile)
}

// EnsureConfigDirs creates necessary config directories
func EnsureConfigDirs() error {
	dirs := []string{
		GetUserConfigDir(),
		GetSessionsDir(),
		GetUserCommandsDir(),
		GetUserRulesDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}
