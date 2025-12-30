package permissions

import (
	"fmt"
	"sync"

	"github.com/oscode-cli/oscode/internal/config"
)

// Mode represents the permission mode
type Mode string

const (
	ModeAuto Mode = "auto"    // Auto-accept allowed tools
	ModeAsk  Mode = "ask"     // Ask for all tools
	ModePlan Mode = "plan"    // Read-only mode
)

// PermissionCallback is called to request user permission
type PermissionCallback func(tool string, input map[string]interface{}, description string) (bool, error)

// Manager handles permission checking and enforcement
type Manager struct {
	mode            Mode
	ruleSet         *RuleSet
	sessionAllowed  map[string]bool // Tools allowed for this session
	callback        PermissionCallback
	skipPermissions bool
	mu              sync.RWMutex
}

// NewManager creates a new permission manager
func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		mode:           ModeAsk,
		ruleSet:        NewRuleSet(),
		sessionAllowed: make(map[string]bool),
	}

	// Parse permission mode
	switch cfg.PermissionMode {
	case "auto", "acceptEdits":
		m.mode = ModeAuto
	case "ask":
		m.mode = ModeAsk
	case "plan":
		m.mode = ModePlan
	}

	// Parse permission rules from config
	m.ruleSet.ParseRules(
		cfg.Permissions.Allow,
		cfg.Permissions.Ask,
		cfg.Permissions.Deny,
	)

	return m
}

// SetMode sets the permission mode
func (m *Manager) SetMode(mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = mode
}

// GetMode returns the current permission mode
func (m *Manager) GetMode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mode
}

// SetCallback sets the permission callback
func (m *Manager) SetCallback(callback PermissionCallback) {
	m.callback = callback
}

// SetSkipPermissions sets whether to skip all permissions
func (m *Manager) SetSkipPermissions(skip bool) {
	m.skipPermissions = skip
}

// AddRule adds a permission rule
func (m *Manager) AddRule(ruleStr string, action Action) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ruleSet.AddRule(ParseRule(ruleStr, action))
}

// AllowForSession allows a tool for the current session
func (m *Manager) AllowForSession(tool string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionAllowed[tool] = true
}

// Check checks if a tool execution is allowed
// Returns: allowed, error
func (m *Manager) Check(tool string, input map[string]interface{}) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Skip permissions if flag is set
	if m.skipPermissions {
		return true, nil
	}

	// Plan mode denies all write operations
	if m.mode == ModePlan {
		if isWriteOperation(tool) {
			return false, fmt.Errorf("write operations not allowed in plan mode")
		}
		return true, nil
	}

	// Check session-level allows
	if m.sessionAllowed[tool] {
		return true, nil
	}

	// Check rule-based permissions
	action := m.ruleSet.Check(tool, input)

	switch action {
	case ActionAllow:
		return true, nil
	case ActionDeny:
		return false, fmt.Errorf("operation denied by permission rules")
	case ActionAsk:
		// Need to ask user
		return false, nil
	}

	return false, nil
}

// RequestPermission requests permission from the user
func (m *Manager) RequestPermission(tool string, input map[string]interface{}) (bool, error) {
	if m.callback == nil {
		return false, fmt.Errorf("no permission callback configured")
	}

	// Generate description
	description := generateDescription(tool, input)

	return m.callback(tool, input, description)
}

// CheckAndRequest checks permission and requests if needed
func (m *Manager) CheckAndRequest(tool string, input map[string]interface{}) (bool, error) {
	allowed, err := m.Check(tool, input)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}

	// Need to request permission
	return m.RequestPermission(tool, input)
}

func isWriteOperation(tool string) bool {
	switch tool {
	case "Write", "Edit", "Bash", "NotebookEdit":
		return true
	default:
		return false
	}
}

func generateDescription(tool string, input map[string]interface{}) string {
	switch tool {
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			return fmt.Sprintf("Execute command: %s", cmd)
		}
		if desc, ok := input["description"].(string); ok {
			return desc
		}
		return "Execute shell command"

	case "Read":
		if path, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("Read file: %s", path)
		}
		return "Read file"

	case "Write":
		if path, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("Write file: %s", path)
		}
		return "Write file"

	case "Edit":
		if path, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("Edit file: %s", path)
		}
		return "Edit file"

	case "WebFetch":
		if url, ok := input["url"].(string); ok {
			return fmt.Sprintf("Fetch URL: %s", url)
		}
		return "Fetch web content"

	case "WebSearch":
		if query, ok := input["query"].(string); ok {
			return fmt.Sprintf("Web search: %s", query)
		}
		return "Perform web search"

	default:
		return fmt.Sprintf("Execute %s", tool)
	}
}

// GetCommandFromInput extracts the command from Bash tool input
func GetCommandFromInput(input map[string]interface{}) string {
	if cmd, ok := input["command"].(string); ok {
		return cmd
	}
	return ""
}

// GetFilePathFromInput extracts the file path from file tool input
func GetFilePathFromInput(input map[string]interface{}) string {
	if path, ok := input["file_path"].(string); ok {
		return path
	}
	return ""
}
