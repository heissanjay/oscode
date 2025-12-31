package tools

import (
	"context"
	"encoding/json"
)

// PlanModeCallback is called when entering or exiting plan mode
type PlanModeCallback func(entering bool)

// EnterPlanModeTool allows the AI to switch into planning/read-only mode
type EnterPlanModeTool struct {
	BaseTool
	callback PlanModeCallback
}

// NewEnterPlanModeTool creates a new EnterPlanMode tool
func NewEnterPlanModeTool(callback PlanModeCallback) *EnterPlanModeTool {
	return &EnterPlanModeTool{
		BaseTool: NewBaseTool(
			"EnterPlanMode",
			`Use this tool proactively when you're about to start a non-trivial implementation task. Getting user sign-off on your approach before writing code prevents wasted effort and ensures alignment.

When to use:
- New feature implementation requiring architectural decisions
- Multiple valid approaches exist for the task
- Code modifications that affect existing behavior
- Multi-file changes (more than 2-3 files)
- Unclear requirements needing exploration

When NOT to use:
- Single-line or few-line fixes
- Simple, specific instructions from user
- Pure research/exploration tasks`,
			BuildSchema(map[string]interface{}{}, []string{}),
			false, // Doesn't require permission - just changes mode
			CategoryAgent,
		),
		callback: callback,
	}
}

func (t *EnterPlanModeTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	if t.callback != nil {
		t.callback(true) // entering plan mode
	}

	result := NewResult(`Plan mode activated. You are now in read-only mode for exploration and planning.

In plan mode:
- You can use Read, Glob, Grep, CodeSearch, and LSP tools
- You cannot use Write, Edit, or Bash tools (except for read-only commands)
- Design your implementation approach
- Use AskUserQuestion to clarify requirements
- Call ExitPlanMode when ready to implement

Write your plan and then call ExitPlanMode to get user approval.`)

	result.WithMetadata("mode", "plan")
	return result, nil
}

// ExitPlanModeTool allows the AI to exit planning mode and proceed with implementation
type ExitPlanModeTool struct {
	BaseTool
	callback PlanModeCallback
}

// NewExitPlanModeTool creates a new ExitPlanMode tool
func NewExitPlanModeTool(callback PlanModeCallback) *ExitPlanModeTool {
	return &ExitPlanModeTool{
		BaseTool: NewBaseTool(
			"ExitPlanMode",
			`Use this tool when you have finished writing your plan and are ready for user approval.

Only use this tool when:
- You have explored the codebase and understand the task
- You have written a clear implementation plan
- You have resolved any ambiguities with AskUserQuestion
- The task requires writing code (not just research)

The user will review your plan before you can proceed with implementation.`,
			BuildSchema(map[string]interface{}{}, []string{}),
			false, // Doesn't require permission
			CategoryAgent,
		),
		callback: callback,
	}
}

func (t *ExitPlanModeTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	if t.callback != nil {
		t.callback(false) // exiting plan mode
	}

	result := NewResult(`Plan mode deactivated. Awaiting user approval.

The user will review your plan. Once approved, you can proceed with implementation.`)

	result.WithMetadata("mode", "normal")
	return result, nil
}
