package permissions

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Rule represents a permission rule
type Rule struct {
	Tool     string // Tool name (e.g., "Bash", "Read", "Edit")
	Pattern  string // Pattern to match (e.g., "npm:*", ".env*")
	Action   Action // Allow, Ask, or Deny
	IsRegex  bool   // Whether pattern is a regex
}

// Action represents a permission action
type Action int

const (
	ActionAllow Action = iota
	ActionAsk
	ActionDeny
)

// ParseRule parses a permission rule string
// Format: "Tool(pattern)" or "Tool"
// Examples: "Bash(npm:*)", "Read(.env*)", "Edit", "WebFetch"
func ParseRule(ruleStr string, action Action) *Rule {
	rule := &Rule{
		Action: action,
	}

	// Check for pattern in parentheses
	if idx := strings.Index(ruleStr, "("); idx != -1 {
		rule.Tool = ruleStr[:idx]
		end := strings.LastIndex(ruleStr, ")")
		if end > idx {
			rule.Pattern = ruleStr[idx+1 : end]
		}
	} else {
		rule.Tool = ruleStr
	}

	return rule
}

// Match checks if a rule matches a tool invocation
func (r *Rule) Match(tool string, input map[string]interface{}) bool {
	// Check tool name
	if r.Tool != tool && r.Tool != "*" {
		return false
	}

	// If no pattern, match all invocations of this tool
	if r.Pattern == "" {
		return true
	}

	// Match pattern based on tool type
	switch tool {
	case "Bash":
		return r.matchBashCommand(input)
	case "Read", "Write", "Edit":
		return r.matchFilePath(input)
	case "WebFetch", "WebSearch":
		return r.matchURL(input)
	default:
		return true
	}
}

func (r *Rule) matchBashCommand(input map[string]interface{}) bool {
	command, ok := input["command"].(string)
	if !ok {
		return false
	}

	// Pattern format: "command:*" means command starts with "command"
	if strings.HasSuffix(r.Pattern, ":*") {
		prefix := strings.TrimSuffix(r.Pattern, ":*")
		return strings.HasPrefix(command, prefix)
	}

	// Pattern format: "command" means exact match or starts with
	if strings.HasSuffix(r.Pattern, "*") {
		prefix := strings.TrimSuffix(r.Pattern, "*")
		return strings.HasPrefix(command, prefix)
	}

	// Exact match
	return command == r.Pattern || strings.HasPrefix(command, r.Pattern+" ")
}

func (r *Rule) matchFilePath(input map[string]interface{}) bool {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return false
	}

	// Normalize path
	filePath = filepath.Clean(filePath)

	// Glob pattern matching
	if strings.Contains(r.Pattern, "*") || strings.Contains(r.Pattern, "?") {
		matched, _ := filepath.Match(r.Pattern, filepath.Base(filePath))
		if matched {
			return true
		}

		// Also try matching full path
		matched, _ = filepath.Match(r.Pattern, filePath)
		return matched
	}

	// Check if path contains pattern
	return strings.Contains(filePath, r.Pattern)
}

func (r *Rule) matchURL(input map[string]interface{}) bool {
	url, ok := input["url"].(string)
	if !ok {
		return false
	}

	// Wildcard matching
	if strings.Contains(r.Pattern, "*") {
		pattern := strings.ReplaceAll(r.Pattern, "*", ".*")
		re, err := regexp.Compile("^" + pattern + "$")
		if err != nil {
			return false
		}
		return re.MatchString(url)
	}

	// Contains matching
	return strings.Contains(url, r.Pattern)
}

// RuleSet manages a collection of rules
type RuleSet struct {
	allowRules []*Rule
	askRules   []*Rule
	denyRules  []*Rule
}

// NewRuleSet creates a new rule set
func NewRuleSet() *RuleSet {
	return &RuleSet{
		allowRules: make([]*Rule, 0),
		askRules:   make([]*Rule, 0),
		denyRules:  make([]*Rule, 0),
	}
}

// AddRule adds a rule to the set
func (rs *RuleSet) AddRule(rule *Rule) {
	switch rule.Action {
	case ActionAllow:
		rs.allowRules = append(rs.allowRules, rule)
	case ActionAsk:
		rs.askRules = append(rs.askRules, rule)
	case ActionDeny:
		rs.denyRules = append(rs.denyRules, rule)
	}
}

// ParseRules parses permission rules from config
func (rs *RuleSet) ParseRules(allow, ask, deny []string) {
	for _, r := range allow {
		rs.AddRule(ParseRule(r, ActionAllow))
	}
	for _, r := range ask {
		rs.AddRule(ParseRule(r, ActionAsk))
	}
	for _, r := range deny {
		rs.AddRule(ParseRule(r, ActionDeny))
	}
}

// Check returns the action for a tool invocation
// Priority: Deny > Ask > Allow > Default
func (rs *RuleSet) Check(tool string, input map[string]interface{}) Action {
	// Check deny rules first
	for _, rule := range rs.denyRules {
		if rule.Match(tool, input) {
			return ActionDeny
		}
	}

	// Check ask rules
	for _, rule := range rs.askRules {
		if rule.Match(tool, input) {
			return ActionAsk
		}
	}

	// Check allow rules
	for _, rule := range rs.allowRules {
		if rule.Match(tool, input) {
			return ActionAllow
		}
	}

	// Default to ask
	return ActionAsk
}
