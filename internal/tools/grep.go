package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// GrepInput defines the input for the Grep tool
type GrepInput struct {
	Pattern      string `json:"pattern"`
	Path         string `json:"path,omitempty"`
	Glob         string `json:"glob,omitempty"`
	Type         string `json:"type,omitempty"`
	OutputMode   string `json:"output_mode,omitempty"`
	ContextBefore int   `json:"B,omitempty"`
	ContextAfter  int   `json:"A,omitempty"`
	Context       int   `json:"C,omitempty"`
	ShowLines    *bool  `json:"n,omitempty"`
	IgnoreCase   bool   `json:"i,omitempty"`
	Multiline    bool   `json:"multiline,omitempty"`
	HeadLimit    int    `json:"head_limit,omitempty"`
	Offset       int    `json:"offset,omitempty"`
}

// GrepTool searches for patterns in files
type GrepTool struct {
	BaseTool
	workDir string
}

// NewGrepTool creates a new Grep tool
func NewGrepTool(workDir string) *GrepTool {
	return &GrepTool{
		BaseTool: NewBaseTool(
			"Grep",
			"A powerful search tool for finding patterns in file contents. Supports regex, file type filtering, and various output modes.",
			BuildSchema(map[string]interface{}{
				"pattern": StringProperty("Regular expression pattern to search for", true),
				"path":    StringProperty("File or directory to search in (defaults to current directory)", false),
				"glob":    StringProperty("Glob pattern to filter files (e.g., '*.js', '**/*.tsx')", false),
				"type":    StringProperty("File type to search (e.g., 'js', 'py', 'go')", false),
				"output_mode": map[string]interface{}{
					"type":        "string",
					"description": "Output mode: 'content' (matching lines), 'files_with_matches' (file paths only), 'count'",
					"enum":        []string{"content", "files_with_matches", "count"},
				},
				"B":          IntProperty("Lines of context before match"),
				"A":          IntProperty("Lines of context after match"),
				"C":          IntProperty("Lines of context before and after match"),
				"n":          BoolProperty("Show line numbers (default: true)"),
				"i":          BoolProperty("Case insensitive search"),
				"multiline":  BoolProperty("Enable multiline mode"),
				"head_limit": IntProperty("Limit output to first N entries"),
				"offset":     IntProperty("Skip first N entries"),
			}, []string{"pattern"}),
			false, // Grep doesn't require permission
			CategorySearch,
		),
		workDir: workDir,
	}
}

func (t *GrepTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params GrepInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if params.Pattern == "" {
		return NewErrorResultString("pattern is required"), nil
	}

	// Try using ripgrep if available
	if hasRipgrep() {
		return t.executeWithRipgrep(ctx, params)
	}

	// Fall back to native Go implementation
	return t.executeNative(ctx, params)
}

func hasRipgrep() bool {
	_, err := exec.LookPath("rg")
	return err == nil
}

func (t *GrepTool) executeWithRipgrep(ctx context.Context, params GrepInput) (*Result, error) {
	args := []string{}

	// Output mode
	outputMode := params.OutputMode
	if outputMode == "" {
		outputMode = "files_with_matches"
	}

	switch outputMode {
	case "files_with_matches":
		args = append(args, "-l")
	case "count":
		args = append(args, "-c")
	case "content":
		// Default rg behavior
	}

	// Context
	if params.Context > 0 {
		args = append(args, "-C", fmt.Sprintf("%d", params.Context))
	} else {
		if params.ContextBefore > 0 {
			args = append(args, "-B", fmt.Sprintf("%d", params.ContextBefore))
		}
		if params.ContextAfter > 0 {
			args = append(args, "-A", fmt.Sprintf("%d", params.ContextAfter))
		}
	}

	// Line numbers
	showLines := params.ShowLines == nil || *params.ShowLines
	if showLines && outputMode == "content" {
		args = append(args, "-n")
	}

	// Case insensitive
	if params.IgnoreCase {
		args = append(args, "-i")
	}

	// Multiline
	if params.Multiline {
		args = append(args, "-U", "--multiline-dotall")
	}

	// File type
	if params.Type != "" {
		args = append(args, "--type", params.Type)
	}

	// Glob pattern
	if params.Glob != "" {
		args = append(args, "--glob", params.Glob)
	}

	// Pattern
	args = append(args, params.Pattern)

	// Path
	searchPath := t.workDir
	if params.Path != "" {
		if filepath.IsAbs(params.Path) {
			searchPath = params.Path
		} else {
			searchPath = filepath.Join(t.workDir, params.Path)
		}
	}
	args = append(args, searchPath)

	// Execute ripgrep
	cmd := exec.CommandContext(ctx, "rg", args...)
	output, err := cmd.Output()

	// rg returns exit code 1 for no matches, which is not an error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return NewResult("No matches found"), nil
		}
		return NewErrorResult(fmt.Errorf("ripgrep error: %w", err)), nil
	}

	result := string(output)

	// Apply offset and limit
	if params.Offset > 0 || params.HeadLimit > 0 {
		lines := strings.Split(result, "\n")

		start := params.Offset
		if start > len(lines) {
			start = len(lines)
		}

		end := len(lines)
		if params.HeadLimit > 0 && start+params.HeadLimit < end {
			end = start + params.HeadLimit
		}

		result = strings.Join(lines[start:end], "\n")
	}

	return NewResult(strings.TrimSuffix(result, "\n")), nil
}

func (t *GrepTool) executeNative(ctx context.Context, params GrepInput) (*Result, error) {
	// Compile regex
	flags := ""
	if params.IgnoreCase {
		flags = "(?i)"
	}
	if params.Multiline {
		flags += "(?s)"
	}

	re, err := regexp.Compile(flags + params.Pattern)
	if err != nil {
		return NewErrorResult(fmt.Errorf("invalid regex: %w", err)), nil
	}

	// Get search path
	searchPath := t.workDir
	if params.Path != "" {
		if filepath.IsAbs(params.Path) {
			searchPath = params.Path
		} else {
			searchPath = filepath.Join(t.workDir, params.Path)
		}
	}

	// Collect files to search
	var files []string

	info, err := os.Stat(searchPath)
	if err != nil {
		return NewErrorResult(err), nil
	}

	if info.IsDir() {
		// Walk directory
		err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}
			if info.IsDir() {
				// Skip hidden directories
				if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
					return filepath.SkipDir
				}
				return nil
			}

			// Apply glob filter
			if params.Glob != "" {
				matched, _ := filepath.Match(params.Glob, info.Name())
				if !matched {
					return nil
				}
			}

			// Apply type filter
			if params.Type != "" && !matchesType(path, params.Type) {
				return nil
			}

			files = append(files, path)
			return nil
		})
		if err != nil {
			return NewErrorResult(err), nil
		}
	} else {
		files = []string{searchPath}
	}

	// Search files
	outputMode := params.OutputMode
	if outputMode == "" {
		outputMode = "files_with_matches"
	}

	var results []string
	matchCount := 0

	for _, filePath := range files {
		matches := t.searchFile(filePath, re, params)

		if len(matches) > 0 {
			relPath, _ := filepath.Rel(t.workDir, filePath)
			if relPath == "" {
				relPath = filePath
			}

			switch outputMode {
			case "files_with_matches":
				results = append(results, relPath)
			case "count":
				results = append(results, fmt.Sprintf("%s:%d", relPath, len(matches)))
			case "content":
				for _, match := range matches {
					results = append(results, fmt.Sprintf("%s:%s", relPath, match))
				}
			}

			matchCount += len(matches)
		}
	}

	// Apply offset and limit
	start := params.Offset
	if start > len(results) {
		start = len(results)
	}

	end := len(results)
	if params.HeadLimit > 0 && start+params.HeadLimit < end {
		end = start + params.HeadLimit
	}

	results = results[start:end]

	if len(results) == 0 {
		return NewResult("No matches found"), nil
	}

	result := NewResult(strings.Join(results, "\n"))
	result.WithMetadata("match_count", matchCount)
	return result, nil
}

func (t *GrepTool) searchFile(filePath string, re *regexp.Regexp, params GrepInput) []string {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var matches []string
	showLines := params.ShowLines == nil || *params.ShowLines

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if re.MatchString(line) {
			if showLines {
				matches = append(matches, fmt.Sprintf("%d:%s", lineNum, line))
			} else {
				matches = append(matches, line)
			}
		}
	}

	return matches
}

func matchesType(path, fileType string) bool {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")

	typeMap := map[string][]string{
		"js":     {"js", "jsx", "mjs"},
		"ts":     {"ts", "tsx"},
		"py":     {"py", "pyw"},
		"go":     {"go"},
		"rust":   {"rs"},
		"java":   {"java"},
		"c":      {"c", "h"},
		"cpp":    {"cpp", "cc", "cxx", "hpp", "hh"},
		"rb":     {"rb"},
		"php":    {"php"},
		"html":   {"html", "htm"},
		"css":    {"css", "scss", "sass", "less"},
		"json":   {"json"},
		"yaml":   {"yaml", "yml"},
		"md":     {"md", "markdown"},
		"sh":     {"sh", "bash"},
	}

	if exts, ok := typeMap[fileType]; ok {
		for _, e := range exts {
			if ext == e {
				return true
			}
		}
		return false
	}

	// Direct extension match
	return ext == fileType
}

// GetShellName returns the appropriate shell for the current OS
func GetShellName() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "bash"
}
