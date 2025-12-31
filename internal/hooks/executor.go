package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/heissanjay/oscode/internal/config"
)

// Event represents a hook event type
type Event string

const (
	EventPreToolUse        Event = "PreToolUse"
	EventPostToolUse       Event = "PostToolUse"
	EventPermissionRequest Event = "PermissionRequest"
	EventUserPromptSubmit  Event = "UserPromptSubmit"
	EventSessionStart      Event = "SessionStart"
	EventSessionEnd        Event = "SessionEnd"
	EventNotification      Event = "Notification"
	EventStop              Event = "Stop"
)

// Context holds context for hook execution
type Context struct {
	Event     Event
	ToolName  string
	Input     map[string]interface{}
	Result    string
	IsError   bool
	SessionID string
	WorkDir   string
	Prompt    string
	Message   string
}

// Result represents the result of hook execution
type Result struct {
	Continue bool                   // Whether to continue with the operation
	Message  string                 // Optional message from the hook
	Modified map[string]interface{} // Modified input (for pre hooks)
}

// Executor handles hook execution
type Executor struct {
	config  config.HookConfig
	workDir string
	env     []string
}

// NewExecutor creates a new hook executor
func NewExecutor(cfg config.HookConfig, workDir string) *Executor {
	return &Executor{
		config:  cfg,
		workDir: workDir,
		env:     os.Environ(),
	}
}

// Execute runs hooks for the given event
func (e *Executor) Execute(ctx context.Context, hookCtx Context) (*Result, error) {
	hooks := e.getHooksForEvent(hookCtx.Event)
	if len(hooks) == 0 {
		return &Result{Continue: true}, nil
	}

	result := &Result{Continue: true}

	for _, hookDef := range hooks {
		// Check if hook matches the context
		if !e.matchesContext(hookDef.Matcher, hookCtx) {
			continue
		}

		// Execute each hook action
		for _, action := range hookDef.Hooks {
			actionResult, err := e.executeAction(ctx, action, hookCtx)
			if err != nil {
				return nil, fmt.Errorf("hook execution failed: %w", err)
			}

			// If any hook says don't continue, stop
			if !actionResult.Continue {
				result.Continue = false
				result.Message = actionResult.Message
				return result, nil
			}

			// Merge modified input if present
			if actionResult.Modified != nil {
				result.Modified = actionResult.Modified
			}
		}
	}

	return result, nil
}

func (e *Executor) getHooksForEvent(event Event) []config.HookDefinition {
	switch event {
	case EventPreToolUse:
		return e.config.PreToolUse
	case EventPostToolUse:
		return e.config.PostToolUse
	case EventPermissionRequest:
		return e.config.PermissionRequest
	case EventUserPromptSubmit:
		return e.config.UserPromptSubmit
	case EventSessionStart:
		return e.config.SessionStart
	case EventSessionEnd:
		return e.config.SessionEnd
	case EventNotification:
		return e.config.Notification
	case EventStop:
		return e.config.Stop
	default:
		return nil
	}
}

func (e *Executor) matchesContext(matcher string, ctx Context) bool {
	if matcher == "" || matcher == "*" {
		return true
	}

	// Match by tool name for tool-related events
	if ctx.ToolName != "" {
		// Support glob-like patterns
		if matched, _ := matchPattern(matcher, ctx.ToolName); matched {
			return true
		}
	}

	return false
}

func matchPattern(pattern, value string) (bool, error) {
	// Convert glob pattern to regex
	// * matches any characters
	// ? matches single character
	regexPattern := "^"
	for _, ch := range pattern {
		switch ch {
		case '*':
			regexPattern += ".*"
		case '?':
			regexPattern += "."
		case '.', '(', ')', '[', ']', '{', '}', '+', '^', '$', '|', '\\':
			regexPattern += "\\" + string(ch)
		default:
			regexPattern += string(ch)
		}
	}
	regexPattern += "$"

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false, err
	}

	return re.MatchString(value), nil
}

func (e *Executor) executeAction(ctx context.Context, action config.HookAction, hookCtx Context) (*Result, error) {
	switch action.Type {
	case "command":
		return e.executeCommand(ctx, action.Command, hookCtx)
	case "prompt":
		return e.executePrompt(action.Prompt, hookCtx)
	default:
		return nil, fmt.Errorf("unknown hook action type: %s", action.Type)
	}
}

func (e *Executor) executeCommand(ctx context.Context, command string, hookCtx Context) (*Result, error) {
	// Expand variables in command
	command = e.expandVariables(command, hookCtx)

	// Create command with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	}

	cmd.Dir = e.workDir
	cmd.Env = e.buildEnv(hookCtx)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &Result{
		Continue: true,
		Message:  strings.TrimSpace(stdout.String()),
	}

	// Non-zero exit code means don't continue
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				result.Continue = false
				result.Message = strings.TrimSpace(stderr.String())
				if result.Message == "" {
					result.Message = stdout.String()
				}
			}
		} else {
			return nil, err
		}
	}

	// Try to parse output as JSON for modified input
	var modified map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &modified); err == nil {
		result.Modified = modified
	}

	return result, nil
}

func (e *Executor) executePrompt(prompt string, hookCtx Context) (*Result, error) {
	// Expand variables in prompt
	prompt = e.expandVariables(prompt, hookCtx)

	// For now, just return the prompt as a message
	// In a full implementation, this would inject into the conversation
	return &Result{
		Continue: true,
		Message:  prompt,
	}, nil
}

func (e *Executor) expandVariables(s string, ctx Context) string {
	// Replace common variables
	replacements := map[string]string{
		"${TOOL_NAME}":  ctx.ToolName,
		"${EVENT}":      string(ctx.Event),
		"${SESSION_ID}": ctx.SessionID,
		"${WORK_DIR}":   ctx.WorkDir,
		"${PROMPT}":     ctx.Prompt,
		"${MESSAGE}":    ctx.Message,
		"${RESULT}":     ctx.Result,
	}

	for key, value := range replacements {
		s = strings.ReplaceAll(s, key, value)
	}

	// Also handle input as JSON
	if ctx.Input != nil {
		inputJSON, _ := json.Marshal(ctx.Input)
		s = strings.ReplaceAll(s, "${INPUT}", string(inputJSON))
	}

	return s
}

func (e *Executor) buildEnv(ctx Context) []string {
	env := make([]string, len(e.env))
	copy(env, e.env)

	// Add hook-specific environment variables
	env = append(env,
		fmt.Sprintf("OSCODE_HOOK_EVENT=%s", ctx.Event),
		fmt.Sprintf("OSCODE_TOOL_NAME=%s", ctx.ToolName),
		fmt.Sprintf("OSCODE_SESSION_ID=%s", ctx.SessionID),
		fmt.Sprintf("OSCODE_WORK_DIR=%s", ctx.WorkDir),
	)

	if ctx.Input != nil {
		inputJSON, _ := json.Marshal(ctx.Input)
		env = append(env, fmt.Sprintf("OSCODE_INPUT=%s", string(inputJSON)))
	}

	if ctx.Result != "" {
		env = append(env, fmt.Sprintf("OSCODE_RESULT=%s", ctx.Result))
	}

	return env
}

// HasHooks returns true if any hooks are configured for the event
func (e *Executor) HasHooks(event Event) bool {
	return len(e.getHooksForEvent(event)) > 0
}
