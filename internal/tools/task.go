package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// TaskInput defines the input for the Task tool
type TaskInput struct {
	Description  string `json:"description"`
	Prompt       string `json:"prompt"`
	SubagentType string `json:"subagent_type"`
	Model        string `json:"model,omitempty"`
	Background   bool   `json:"run_in_background,omitempty"`
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	AgentID string `json:"agent_id"`
	Result  string `json:"result"`
	Status  string `json:"status"`
}

// TaskExecutor is called to execute a subagent task
type TaskExecutor func(ctx context.Context, input TaskInput) (*TaskResult, error)

// TaskTool spawns subagent tasks
type TaskTool struct {
	BaseTool
	executor TaskExecutor
}

// NewTaskTool creates a new Task tool
func NewTaskTool(executor TaskExecutor) *TaskTool {
	return &TaskTool{
		BaseTool: NewBaseTool(
			"Task",
			"Launch a new agent to handle complex, multi-step tasks autonomously. Use for research, exploration, and specialized tasks.",
			BuildSchema(map[string]interface{}{
				"description": StringProperty("Short (3-5 word) description of the task", true),
				"prompt":      StringProperty("Detailed task description with all necessary context", true),
				"subagent_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of agent to use: 'general-purpose', 'Explore', 'Plan'",
					"enum":        []string{"general-purpose", "Explore", "Plan"},
				},
				"model":             StringProperty("Optional model override (sonnet, opus, haiku)", false),
				"run_in_background": BoolProperty("Run agent in background"),
			}, []string{"description", "prompt", "subagent_type"}),
			false, // Task doesn't require permission
			CategoryAgent,
		),
		executor: executor,
	}
}

func (t *TaskTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params TaskInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if params.Prompt == "" {
		return NewErrorResultString("prompt is required"), nil
	}
	if params.SubagentType == "" {
		params.SubagentType = "general-purpose"
	}

	if t.executor == nil {
		return NewErrorResultString("Task executor not configured"), nil
	}

	result, err := t.executor(ctx, params)
	if err != nil {
		return NewErrorResult(err), nil
	}

	output := NewResult(result.Result)
	output.WithMetadata("agent_id", result.AgentID)
	output.WithMetadata("status", result.Status)

	return output, nil
}

// TodoInput defines the input for the TodoWrite tool
type TodoInput struct {
	Todos []TodoItem `json:"todos"`
}

// TodoItem represents a single todo item
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"` // pending, in_progress, completed
	ActiveForm string `json:"activeForm"`
}

// TodoCallback is called when todos are updated
type TodoCallback func(todos []TodoItem)

// TodoWriteTool manages the todo list
type TodoWriteTool struct {
	BaseTool
	callback TodoCallback
	todos    []TodoItem
}

// NewTodoWriteTool creates a new TodoWrite tool
func NewTodoWriteTool(callback TodoCallback) *TodoWriteTool {
	return &TodoWriteTool{
		BaseTool: NewBaseTool(
			"TodoWrite",
			"Create and manage a structured task list for tracking progress on complex tasks.",
			BuildSchema(map[string]interface{}{
				"todos": map[string]interface{}{
					"type":        "array",
					"description": "The updated todo list",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content":    StringProperty("Task description in imperative form", true),
							"status":     StringProperty("Task status: pending, in_progress, completed", true),
							"activeForm": StringProperty("Present continuous form of the task", true),
						},
						"required": []string{"content", "status", "activeForm"},
					},
				},
			}, []string{"todos"}),
			false, // TodoWrite doesn't require permission
			CategoryOther,
		),
		callback: callback,
		todos:    make([]TodoItem, 0),
	}
}

func (t *TodoWriteTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params TodoInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	t.todos = params.Todos

	if t.callback != nil {
		t.callback(t.todos)
	}

	return NewResult("Todos updated successfully"), nil
}

// GetTodos returns the current todo list
func (t *TodoWriteTool) GetTodos() []TodoItem {
	return t.todos
}

// AskUserInput defines the input for the AskUserQuestion tool
type AskUserInput struct {
	Questions []Question `json:"questions"`
}

// Question represents a question to ask the user
type Question struct {
	Question    string   `json:"question"`
	Header      string   `json:"header"`
	Options     []Option `json:"options"`
	MultiSelect bool     `json:"multiSelect"`
}

// Option represents an answer option
type Option struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// AskUserCallback is called to ask the user a question
type AskUserCallback func(questions []Question) (map[string]string, error)

// AskUserQuestionTool asks the user questions
type AskUserQuestionTool struct {
	BaseTool
	callback AskUserCallback
}

// NewAskUserQuestionTool creates a new AskUserQuestion tool
func NewAskUserQuestionTool(callback AskUserCallback) *AskUserQuestionTool {
	return &AskUserQuestionTool{
		BaseTool: NewBaseTool(
			"AskUserQuestion",
			"Ask the user questions to gather preferences, clarify requirements, or get decisions.",
			BuildSchema(map[string]interface{}{
				"questions": map[string]interface{}{
					"type":        "array",
					"description": "Questions to ask (1-4 questions)",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"question":    StringProperty("The complete question to ask", true),
							"header":      StringProperty("Short label (max 12 chars)", true),
							"options":     ArrayProperty("Available choices (2-4 options)", "object"),
							"multiSelect": BoolProperty("Allow multiple selections"),
						},
						"required": []string{"question", "header", "options", "multiSelect"},
					},
				},
			}, []string{"questions"}),
			false,
			CategoryOther,
		),
		callback: callback,
	}
}

func (t *AskUserQuestionTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params AskUserInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if t.callback == nil {
		return NewErrorResultString("User question callback not configured"), nil
	}

	answers, err := t.callback(params.Questions)
	if err != nil {
		return NewErrorResult(err), nil
	}

	// Format answers
	result, _ := json.Marshal(answers)
	return NewResult(string(result)), nil
}
