package commands

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Command represents a slash command
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Usage       string
	Handler     CommandHandler
	Hidden      bool // Don't show in /help
}

// CommandHandler handles a command execution
type CommandHandler func(ctx *Context, args string) error

// Context provides context for command execution
type Context struct {
	// Application references
	Session    interface{} // *session.Session
	Config     interface{} // *config.Config
	Provider   interface{} // llm.Provider
	ToolRegistry interface{} // *tools.Registry

	// UI callbacks
	Print       func(string)
	PrintError  func(string)
	Clear       func()
	SetModel    func(string)
	SetProvider func(string)

	// Session controls
	Exit       func()
	Reload     func()
}

// Registry manages slash commands
type Registry struct {
	commands map[string]*Command
	aliases  map[string]string
	mu       sync.RWMutex
}

// NewRegistry creates a new command registry
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]*Command),
		aliases:  make(map[string]string),
	}
}

// Register adds a command to the registry
func (r *Registry) Register(cmd *Command) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.commands[cmd.Name] = cmd

	for _, alias := range cmd.Aliases {
		r.aliases[alias] = cmd.Name
	}
}

// Get returns a command by name or alias
func (r *Registry) Get(name string) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check direct match
	if cmd, ok := r.commands[name]; ok {
		return cmd, true
	}

	// Check alias
	if cmdName, ok := r.aliases[name]; ok {
		if cmd, ok := r.commands[cmdName]; ok {
			return cmd, true
		}
	}

	return nil, false
}

// List returns all registered commands
func (r *Registry) List() []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmds := make([]*Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}

	// Sort by name
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name < cmds[j].Name
	})

	return cmds
}

// Execute runs a command
func (r *Registry) Execute(ctx *Context, input string) error {
	// Parse command and args
	input = strings.TrimPrefix(input, "/")
	parts := strings.SplitN(input, " ", 2)
	name := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	cmd, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("unknown command: /%s (use /help for available commands)", name)
	}

	return cmd.Handler(ctx, args)
}

// IsCommand checks if input is a slash command
func IsCommand(input string) bool {
	return strings.HasPrefix(input, "/")
}

// DefaultRegistry is the global command registry
var DefaultRegistry = NewRegistry()

// Register registers a command in the default registry
func Register(cmd *Command) {
	DefaultRegistry.Register(cmd)
}

// Execute executes a command from the default registry
func Execute(ctx *Context, input string) error {
	return DefaultRegistry.Execute(ctx, input)
}

// GetCommandName extracts the command name from input
func GetCommandName(input string) string {
	input = strings.TrimPrefix(input, "/")
	parts := strings.SplitN(input, " ", 2)
	return parts[0]
}

// GetCommandArgs extracts the arguments from input
func GetCommandArgs(input string) string {
	input = strings.TrimPrefix(input, "/")
	parts := strings.SplitN(input, " ", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}
