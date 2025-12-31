package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// LSPInput defines the input for the LSP tool
type LSPInput struct {
	Operation string `json:"operation"` // "hover", "definition", "references", "completion", "diagnostics"
	FilePath  string `json:"file_path"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
}

// LSPTool provides Language Server Protocol operations
type LSPTool struct {
	BaseTool
	workDir string
}

// NewLSPTool creates a new LSP tool
func NewLSPTool(workDir string) *LSPTool {
	return &LSPTool{
		BaseTool: NewBaseTool(
			"LSP",
			"Query Language Server Protocol for code intelligence. Operations: 'hover' for documentation, 'definition' to find where symbol is defined, 'references' to find usages, 'diagnostics' for errors/warnings.",
			BuildSchema(map[string]interface{}{
				"operation": StringProperty("LSP operation: 'hover', 'definition', 'references', 'diagnostics'", true),
				"file_path": StringProperty("Path to the file", true),
				"line":      IntProperty("Line number (1-based)"),
				"column":    IntProperty("Column number (1-based)"),
			}, []string{"operation", "file_path"}),
			false,
			CategorySearch,
		),
		workDir: workDir,
	}
}

func (t *LSPTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params LSPInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if params.Operation == "" {
		return NewErrorResultString("operation is required"), nil
	}
	if params.FilePath == "" {
		return NewErrorResultString("file_path is required"), nil
	}

	// Resolve path
	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}

	// Detect language and run appropriate command
	ext := strings.ToLower(filepath.Ext(filePath))

	switch params.Operation {
	case "hover":
		return t.getHover(ctx, filePath, ext, params.Line, params.Column)
	case "definition":
		return t.getDefinition(ctx, filePath, ext, params.Line, params.Column)
	case "references":
		return t.getReferences(ctx, filePath, ext, params.Line, params.Column)
	case "diagnostics":
		return t.getDiagnostics(ctx, filePath, ext)
	default:
		return NewErrorResultString(fmt.Sprintf("unknown operation: %s", params.Operation)), nil
	}
}

func (t *LSPTool) getHover(ctx context.Context, filePath, ext string, line, col int) (*Result, error) {
	// For Go files, use gopls
	if ext == ".go" {
		return t.goHover(ctx, filePath, line, col)
	}

	// For TypeScript/JavaScript, try to use tsserver info
	if ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" {
		return t.tsHover(ctx, filePath, line, col)
	}

	return NewResult("Hover not available for this file type. Try using CodeSearch instead."), nil
}

func (t *LSPTool) getDefinition(ctx context.Context, filePath, ext string, line, col int) (*Result, error) {
	// For Go files
	if ext == ".go" {
		return t.goDefinition(ctx, filePath, line, col)
	}

	// For TypeScript/JavaScript
	if ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" {
		return t.tsDefinition(ctx, filePath, line, col)
	}

	return NewResult("Definition lookup not available for this file type. Try using CodeSearch with search_type='definition'."), nil
}

func (t *LSPTool) getReferences(ctx context.Context, filePath, ext string, line, col int) (*Result, error) {
	// For Go files
	if ext == ".go" {
		return t.goReferences(ctx, filePath, line, col)
	}

	return NewResult("References not available for this file type. Try using CodeSearch with search_type='reference'."), nil
}

func (t *LSPTool) getDiagnostics(ctx context.Context, filePath, ext string) (*Result, error) {
	switch ext {
	case ".go":
		return t.goDiagnostics(ctx, filePath)
	case ".ts", ".tsx":
		return t.tsDiagnostics(ctx, filePath)
	case ".py":
		return t.pyDiagnostics(ctx, filePath)
	default:
		return NewResult("Diagnostics not available for this file type."), nil
	}
}

// Go-specific implementations using go tools
func (t *LSPTool) goHover(ctx context.Context, filePath string, line, col int) (*Result, error) {
	// Use go doc for hover info
	cmd := exec.CommandContext(ctx, "go", "doc", "-short", filePath)
	cmd.Dir = t.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return NewResult(fmt.Sprintf("Could not get documentation: %s", string(output))), nil
	}
	return NewResult(string(output)), nil
}

func (t *LSPTool) goDefinition(ctx context.Context, filePath string, line, col int) (*Result, error) {
	// Use guru for definition
	pos := fmt.Sprintf("%s:#%d", filePath, line*100+col)
	cmd := exec.CommandContext(ctx, "guru", "definition", pos)
	cmd.Dir = t.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback: suggest using CodeSearch
		return NewResult(fmt.Sprintf("Could not find definition. Try using CodeSearch with search_type='definition'.\nError: %s", string(output))), nil
	}
	return NewResult(string(output)), nil
}

func (t *LSPTool) goReferences(ctx context.Context, filePath string, line, col int) (*Result, error) {
	// Use guru for references
	pos := fmt.Sprintf("%s:#%d", filePath, line*100+col)
	cmd := exec.CommandContext(ctx, "guru", "referrers", pos)
	cmd.Dir = t.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return NewResult(fmt.Sprintf("Could not find references. Try using CodeSearch.\nError: %s", string(output))), nil
	}
	return NewResult(string(output)), nil
}

func (t *LSPTool) goDiagnostics(ctx context.Context, filePath string) (*Result, error) {
	// Run go vet and go build for diagnostics
	var sb strings.Builder

	// go vet
	cmd := exec.CommandContext(ctx, "go", "vet", filePath)
	cmd.Dir = t.workDir
	output, _ := cmd.CombinedOutput()
	if len(output) > 0 {
		sb.WriteString("go vet:\n")
		sb.WriteString(string(output))
		sb.WriteString("\n")
	}

	// go build (dry run)
	cmd = exec.CommandContext(ctx, "go", "build", "-n", filePath)
	cmd.Dir = t.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		sb.WriteString("Build errors:\n")
		sb.WriteString(string(output))
	}

	if sb.Len() == 0 {
		return NewResult("No diagnostics found. Code looks good!"), nil
	}

	return NewResult(sb.String()), nil
}

// TypeScript-specific implementations
func (t *LSPTool) tsHover(ctx context.Context, filePath string, line, col int) (*Result, error) {
	return NewResult("TypeScript hover requires a running language server. Try using CodeSearch to find related code."), nil
}

func (t *LSPTool) tsDefinition(ctx context.Context, filePath string, line, col int) (*Result, error) {
	return NewResult("TypeScript definition requires a running language server. Try using CodeSearch with search_type='definition'."), nil
}

func (t *LSPTool) tsDiagnostics(ctx context.Context, filePath string) (*Result, error) {
	// Run tsc for type checking
	cmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit", filePath)
	cmd.Dir = t.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return NewResult(fmt.Sprintf("TypeScript diagnostics:\n%s", string(output))), nil
	}
	return NewResult("No TypeScript errors found."), nil
}

// Python-specific implementations
func (t *LSPTool) pyDiagnostics(ctx context.Context, filePath string) (*Result, error) {
	var sb strings.Builder

	// Try pyflakes
	cmd := exec.CommandContext(ctx, "pyflakes", filePath)
	cmd.Dir = t.workDir
	output, _ := cmd.CombinedOutput()
	if len(output) > 0 {
		sb.WriteString("pyflakes:\n")
		sb.WriteString(string(output))
		sb.WriteString("\n")
	}

	// Try mypy
	cmd = exec.CommandContext(ctx, "mypy", filePath)
	cmd.Dir = t.workDir
	output, _ = cmd.CombinedOutput()
	if len(output) > 0 {
		sb.WriteString("mypy:\n")
		sb.WriteString(string(output))
	}

	if sb.Len() == 0 {
		return NewResult("No Python diagnostics found."), nil
	}

	return NewResult(sb.String()), nil
}
