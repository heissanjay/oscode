package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// BashInput defines the input for the Bash tool
type BashInput struct {
	Command         string `json:"command"`
	Description     string `json:"description,omitempty"`
	Timeout         int    `json:"timeout,omitempty"`         // in milliseconds
	RunInBackground bool   `json:"run_in_background,omitempty"`
}

// BashTool executes shell commands
type BashTool struct {
	BaseTool
	workDir         string
	currentDir      string // Current working directory (can change with cd)
	backgroundTasks map[string]*BackgroundTask
	taskMu          sync.Mutex
	env             []string
}

// BackgroundTask represents a background command execution
type BackgroundTask struct {
	ID        string
	Command   string
	StartTime time.Time
	Output    *bytes.Buffer
	Err       error
	Done      bool
	Cmd       *exec.Cmd
	mu        sync.Mutex
}

// NewBashTool creates a new Bash tool
func NewBashTool(workDir string) *BashTool {
	return &BashTool{
		BaseTool: NewBaseTool(
			"Bash",
			"Executes a shell command in a persistent shell session. Use for running builds, tests, git commands, and other terminal operations.",
			BuildSchema(map[string]interface{}{
				"command":           StringProperty("The shell command to execute", true),
				"description":       StringProperty("Brief description of what the command does (5-10 words)", false),
				"timeout":           IntProperty("Timeout in milliseconds (default: 120000, max: 600000)"),
				"run_in_background": BoolProperty("Run in background and return task ID"),
			}, []string{"command"}),
			true, // Bash requires permission
			CategoryExecution,
		),
		workDir:         workDir,
		currentDir:      workDir,
		backgroundTasks: make(map[string]*BackgroundTask),
		env:             os.Environ(),
	}
}

// SetEnv sets additional environment variables
func (t *BashTool) SetEnv(env map[string]string) {
	for k, v := range env {
		t.env = append(t.env, fmt.Sprintf("%s=%s", k, v))
	}
}

// GetCurrentDir returns the current working directory
func (t *BashTool) GetCurrentDir() string {
	return t.currentDir
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params BashInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if params.Command == "" {
		return NewErrorResultString("command is required"), nil
	}

	// Set default timeout
	timeout := params.Timeout
	if timeout <= 0 {
		timeout = 120000 // 2 minutes default
	}
	if timeout > 600000 {
		timeout = 600000 // 10 minutes max
	}

	// Handle background execution
	if params.RunInBackground {
		return t.executeBackground(params.Command)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	return t.executeCommand(ctx, params.Command)
}

func (t *BashTool) executeCommand(ctx context.Context, command string) (*Result, error) {
	var cmd *exec.Cmd

	// Determine shell based on OS
	if runtime.GOOS == "windows" {
		// Use cmd.exe on Windows
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		// Use bash on Unix-like systems
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	}

	// Set working directory
	cmd.Dir = t.currentDir
	cmd.Env = t.env

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()

	// Combine output
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	// Truncate if too long
	const maxOutput = 30000
	if len(output) > maxOutput {
		output = output[:maxOutput] + "\n... (output truncated)"
	}

	// Check for cd command to update current directory
	t.updateCurrentDir(command)

	// Handle errors
	if ctx.Err() == context.DeadlineExceeded {
		return NewErrorResultString("Command timed out"), nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result := NewResult(output)
			result.IsError = true
			result.WithMetadata("exit_code", exitErr.ExitCode())
			return result, nil
		}
		return NewErrorResult(err), nil
	}

	result := NewResult(output)
	result.WithMetadata("exit_code", 0)
	return result, nil
}

func (t *BashTool) executeBackground(command string) (*Result, error) {
	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())

	task := &BackgroundTask{
		ID:        taskID,
		Command:   command,
		StartTime: time.Now(),
		Output:    &bytes.Buffer{},
	}

	// Create command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("bash", "-c", command)
	}

	cmd.Dir = t.currentDir
	cmd.Env = t.env
	cmd.Stdout = task.Output
	cmd.Stderr = task.Output
	task.Cmd = cmd

	// Store task
	t.taskMu.Lock()
	t.backgroundTasks[taskID] = task
	t.taskMu.Unlock()

	// Start command in goroutine
	go func() {
		err := cmd.Run()
		task.mu.Lock()
		task.Err = err
		task.Done = true
		task.mu.Unlock()
	}()

	result := NewResult(fmt.Sprintf("Background task started with ID: %s", taskID))
	result.WithMetadata("task_id", taskID)
	return result, nil
}

// GetBackgroundTask returns a background task by ID
func (t *BashTool) GetBackgroundTask(taskID string) (*BackgroundTask, bool) {
	t.taskMu.Lock()
	defer t.taskMu.Unlock()
	task, ok := t.backgroundTasks[taskID]
	return task, ok
}

// GetTaskOutput returns the output of a background task
func (t *BashTool) GetTaskOutput(taskID string, wait bool) (*Result, error) {
	task, ok := t.GetBackgroundTask(taskID)
	if !ok {
		return NewErrorResultString(fmt.Sprintf("Task not found: %s", taskID)), nil
	}

	if wait {
		// Wait for task to complete
		for {
			task.mu.Lock()
			done := task.Done
			task.mu.Unlock()
			if done {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	output := task.Output.String()
	result := NewResult(output)

	if task.Done {
		result.WithMetadata("status", "completed")
		if task.Err != nil {
			if exitErr, ok := task.Err.(*exec.ExitError); ok {
				result.WithMetadata("exit_code", exitErr.ExitCode())
			} else {
				result.IsError = true
			}
		} else {
			result.WithMetadata("exit_code", 0)
		}
	} else {
		result.WithMetadata("status", "running")
	}

	return result, nil
}

// ListBackgroundTasks returns all background tasks
func (t *BashTool) ListBackgroundTasks() []*BackgroundTask {
	t.taskMu.Lock()
	defer t.taskMu.Unlock()

	tasks := make([]*BackgroundTask, 0, len(t.backgroundTasks))
	for _, task := range t.backgroundTasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// KillTask kills a background task
func (t *BashTool) KillTask(taskID string) error {
	task, ok := t.GetBackgroundTask(taskID)
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	if task.Done {
		return nil
	}

	if task.Cmd != nil && task.Cmd.Process != nil {
		return task.Cmd.Process.Kill()
	}

	return nil
}

// updateCurrentDir checks if the command changed the directory
func (t *BashTool) updateCurrentDir(command string) {
	// Simple parsing for cd commands
	command = strings.TrimSpace(command)

	// Handle various cd patterns
	var newDir string

	if strings.HasPrefix(command, "cd ") {
		parts := strings.SplitN(command, " ", 2)
		if len(parts) > 1 {
			newDir = strings.TrimSpace(parts[1])
			// Remove quotes
			newDir = strings.Trim(newDir, "\"'")
		}
	} else if command == "cd" || command == "cd ~" {
		// cd alone goes to home
		home, _ := os.UserHomeDir()
		newDir = home
	}

	if newDir == "" {
		return
	}

	// Handle ~ expansion
	if strings.HasPrefix(newDir, "~") {
		home, _ := os.UserHomeDir()
		newDir = filepath.Join(home, newDir[1:])
	}

	// Make absolute
	if !filepath.IsAbs(newDir) {
		newDir = filepath.Join(t.currentDir, newDir)
	}

	// Clean and verify
	newDir = filepath.Clean(newDir)
	if info, err := os.Stat(newDir); err == nil && info.IsDir() {
		t.currentDir = newDir
	}
}

// BashOutputTool retrieves output from background tasks
type BashOutputTool struct {
	BaseTool
	bashTool *BashTool
}

// NewBashOutputTool creates a new BashOutput tool
func NewBashOutputTool(bashTool *BashTool) *BashOutputTool {
	return &BashOutputTool{
		BaseTool: NewBaseTool(
			"BashOutput",
			"Retrieves output from a background shell task",
			BuildSchema(map[string]interface{}{
				"task_id": StringProperty("The ID of the background task", true),
				"wait":    BoolProperty("Wait for task to complete (default: true)"),
			}, []string{"task_id"}),
			false, // Doesn't require permission
			CategoryExecution,
		),
		bashTool: bashTool,
	}
}

func (t *BashOutputTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params struct {
		TaskID string `json:"task_id"`
		Wait   *bool  `json:"wait,omitempty"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	wait := true
	if params.Wait != nil {
		wait = *params.Wait
	}

	return t.bashTool.GetTaskOutput(params.TaskID, wait)
}
