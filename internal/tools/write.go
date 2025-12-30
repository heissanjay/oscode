package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteInput defines the input for the Write tool
type WriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// WriteTool writes files to the filesystem
type WriteTool struct {
	BaseTool
	workDir     string
	filesRead   map[string]bool // Track which files have been read
}

// NewWriteTool creates a new Write tool
func NewWriteTool(workDir string) *WriteTool {
	return &WriteTool{
		BaseTool: NewBaseTool(
			"Write",
			"Writes content to a file. Creates the file if it doesn't exist, or overwrites if it does. You must read a file before overwriting it.",
			BuildSchema(map[string]interface{}{
				"file_path": StringProperty("The absolute path to the file to write", true),
				"content":   StringProperty("The content to write to the file", true),
			}, []string{"file_path", "content"}),
			true, // Write requires permission
			CategoryFile,
		),
		workDir:   workDir,
		filesRead: make(map[string]bool),
	}
}

// MarkFileRead marks a file as having been read
func (t *WriteTool) MarkFileRead(filePath string) {
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}
	filePath = filepath.Clean(filePath)
	t.filesRead[filePath] = true
}

// HasReadFile checks if a file has been read
func (t *WriteTool) HasReadFile(filePath string) bool {
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}
	filePath = filepath.Clean(filePath)
	return t.filesRead[filePath]
}

func (t *WriteTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params WriteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if params.FilePath == "" {
		return NewErrorResultString("file_path is required"), nil
	}

	// Resolve path
	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}
	filePath = filepath.Clean(filePath)

	// Check if file exists and hasn't been read
	if _, err := os.Stat(filePath); err == nil {
		// File exists - check if it was read
		if !t.HasReadFile(filePath) {
			return NewErrorResultString(fmt.Sprintf("Cannot overwrite '%s' without reading it first. Use the Read tool to read the file before writing.", params.FilePath)), nil
		}
	}

	// Create parent directories if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewErrorResult(fmt.Errorf("failed to create directory: %w", err)), nil
	}

	// Write the file
	if err := os.WriteFile(filePath, []byte(params.Content), 0644); err != nil {
		return NewErrorResult(fmt.Errorf("failed to write file: %w", err)), nil
	}

	// Mark as read for future writes
	t.filesRead[filePath] = true

	// Calculate some stats
	lines := 1
	for _, c := range params.Content {
		if c == '\n' {
			lines++
		}
	}

	result := NewResult(fmt.Sprintf("Successfully wrote %d bytes (%d lines) to %s", len(params.Content), lines, params.FilePath))
	result.WithMetadata("bytes_written", len(params.Content))
	result.WithMetadata("lines", lines)

	return result, nil
}
