package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditInput defines the input for the Edit tool
type EditInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

// EditTool performs targeted edits on files
type EditTool struct {
	BaseTool
	workDir   string
	filesRead map[string]bool
}

// NewEditTool creates a new Edit tool
func NewEditTool(workDir string) *EditTool {
	return &EditTool{
		BaseTool: NewBaseTool(
			"Edit",
			"Performs exact string replacements in files. The old_string must match exactly (including whitespace). Use replace_all to replace all occurrences. You must read the file first before editing.",
			BuildSchema(map[string]interface{}{
				"file_path": StringProperty("The absolute path to the file to edit", true),
				"old_string": StringProperty("The exact text to replace", true),
				"new_string": StringProperty("The text to replace it with", true),
				"replace_all": BoolProperty("Replace all occurrences (default: false)"),
			}, []string{"file_path", "old_string", "new_string"}),
			true, // Edit requires permission
			CategoryFile,
		),
		workDir:   workDir,
		filesRead: make(map[string]bool),
	}
}

// MarkFileRead marks a file as having been read
func (t *EditTool) MarkFileRead(filePath string) {
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}
	filePath = filepath.Clean(filePath)
	t.filesRead[filePath] = true
}

// HasReadFile checks if a file has been read
func (t *EditTool) HasReadFile(filePath string) bool {
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}
	filePath = filepath.Clean(filePath)
	return t.filesRead[filePath]
}

func (t *EditTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params EditInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if params.FilePath == "" {
		return NewErrorResultString("file_path is required"), nil
	}
	if params.OldString == "" {
		return NewErrorResultString("old_string is required"), nil
	}
	if params.OldString == params.NewString {
		return NewErrorResultString("old_string and new_string must be different"), nil
	}

	// Resolve path
	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}
	filePath = filepath.Clean(filePath)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return NewErrorResultString(fmt.Sprintf("File not found: %s", params.FilePath)), nil
	}

	// Check if file was read
	if !t.HasReadFile(filePath) {
		return NewErrorResultString(fmt.Sprintf("Cannot edit '%s' without reading it first. Use the Read tool to read the file before editing.", params.FilePath)), nil
	}

	// Read current content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to read file: %w", err)), nil
	}

	contentStr := string(content)

	// Count occurrences
	count := strings.Count(contentStr, params.OldString)

	if count == 0 {
		return NewErrorResultString(fmt.Sprintf("The string to replace was not found in %s. Make sure old_string matches exactly, including whitespace and indentation.", params.FilePath)), nil
	}

	// Check for uniqueness if not replacing all
	if !params.ReplaceAll && count > 1 {
		return NewErrorResultString(fmt.Sprintf("Found %d occurrences of old_string in %s. Either provide more context to make it unique, or set replace_all to true.", count, params.FilePath)), nil
	}

	// Perform replacement
	var newContent string
	if params.ReplaceAll {
		newContent = strings.ReplaceAll(contentStr, params.OldString, params.NewString)
	} else {
		newContent = strings.Replace(contentStr, params.OldString, params.NewString, 1)
	}

	// Write back
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return NewErrorResult(fmt.Errorf("failed to write file: %w", err)), nil
	}

	var msg string
	if params.ReplaceAll {
		msg = fmt.Sprintf("Replaced %d occurrence(s) in %s", count, params.FilePath)
	} else {
		msg = fmt.Sprintf("Replaced 1 occurrence in %s", params.FilePath)
	}

	result := NewResult(msg)
	result.WithMetadata("replacements", count)
	result.WithMetadata("file", params.FilePath)

	return result, nil
}

// CreateDiff generates a diff-like representation of the edit
func CreateDiff(oldContent, newContent, filePath string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- %s\n", filePath))
	diff.WriteString(fmt.Sprintf("+++ %s\n", filePath))

	// Simple line-by-line diff (for display purposes)
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			if oldLine != "" {
				diff.WriteString(fmt.Sprintf("-%s\n", oldLine))
			}
			if newLine != "" {
				diff.WriteString(fmt.Sprintf("+%s\n", newLine))
			}
		}
	}

	return diff.String()
}
