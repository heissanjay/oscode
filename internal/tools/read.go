package tools

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadInput defines the input for the Read tool
type ReadInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// ReadTool reads files from the filesystem
type ReadTool struct {
	BaseTool
	workDir string
}

// NewReadTool creates a new Read tool
func NewReadTool(workDir string) *ReadTool {
	return &ReadTool{
		BaseTool: NewBaseTool(
			"Read",
			"Reads a file from the filesystem. Supports text files, images (PNG, JPG, GIF, WebP), and PDFs. Returns file content with line numbers for text files.",
			BuildSchema(map[string]interface{}{
				"file_path": StringProperty("The absolute path to the file to read", true),
				"offset":    IntProperty("Line number to start reading from (1-based). Optional."),
				"limit":     IntProperty("Number of lines to read. Optional, defaults to 2000."),
			}, []string{"file_path"}),
			false, // Read doesn't require permission by default
			CategoryFile,
		),
		workDir: workDir,
	}
}

func (t *ReadTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params ReadInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	// Resolve path
	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.workDir, filePath)
	}

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewErrorResultString(fmt.Sprintf("File not found: %s", params.FilePath)), nil
		}
		return NewErrorResult(err), nil
	}

	// Check if it's a directory
	if info.IsDir() {
		return NewErrorResultString(fmt.Sprintf("%s is a directory, not a file. Use ls command via Bash tool to list directory contents.", params.FilePath)), nil
	}

	// Detect file type by extension
	ext := strings.ToLower(filepath.Ext(filePath))

	// Handle images
	if isImageExtension(ext) {
		return t.readImage(filePath, ext)
	}

	// Handle PDFs
	if ext == ".pdf" {
		return t.readPDF(filePath)
	}

	// Handle Jupyter notebooks
	if ext == ".ipynb" {
		return t.readNotebook(filePath)
	}

	// Handle text files
	return t.readTextFile(filePath, params.Offset, params.Limit)
}

func (t *ReadTool) readTextFile(filePath string, offset, limit int) (*Result, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return NewErrorResult(err), nil
	}
	defer file.Close()

	// Default limit
	if limit <= 0 {
		limit = 2000
	}

	// Default offset (1-based)
	if offset <= 0 {
		offset = 1
	}

	var lines []string
	scanner := bufio.NewScanner(file)

	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max line size

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum < offset {
			continue
		}
		if lineNum >= offset+limit {
			break
		}

		line := scanner.Text()
		// Truncate very long lines
		if len(line) > 2000 {
			line = line[:2000] + "... (truncated)"
		}

		// Format with line number (cat -n style)
		lines = append(lines, fmt.Sprintf("%6d\t%s", lineNum, line))
	}

	if err := scanner.Err(); err != nil {
		return NewErrorResult(fmt.Errorf("error reading file: %w", err)), nil
	}

	if len(lines) == 0 {
		return NewResult("(empty file)"), nil
	}

	content := strings.Join(lines, "\n")
	result := NewResult(content)
	result.WithMetadata("lines_read", len(lines))
	result.WithMetadata("start_line", offset)

	return result, nil
}

func (t *ReadTool) readImage(filePath, ext string) (*Result, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return NewErrorResult(err), nil
	}

	// Get media type
	mediaType := getMediaType(ext)

	// Encode as base64
	encoded := base64.StdEncoding.EncodeToString(data)

	// Return as special format that can be processed for vision
	result := NewResult(fmt.Sprintf("[Image: %s, %d bytes, %s]", filepath.Base(filePath), len(data), mediaType))
	result.WithMetadata("type", "image")
	result.WithMetadata("media_type", mediaType)
	result.WithMetadata("base64", encoded)
	result.WithMetadata("size", len(data))

	return result, nil
}

func (t *ReadTool) readPDF(filePath string) (*Result, error) {
	// For now, return a placeholder
	// In a full implementation, we'd use a PDF parsing library
	info, _ := os.Stat(filePath)
	return NewResult(fmt.Sprintf("[PDF: %s, %d bytes - PDF parsing not yet implemented]", filepath.Base(filePath), info.Size())), nil
}

func (t *ReadTool) readNotebook(filePath string) (*Result, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return NewErrorResult(err), nil
	}

	// Parse notebook JSON
	var notebook struct {
		Cells []struct {
			CellType string   `json:"cell_type"`
			Source   []string `json:"source"`
			Outputs  []struct {
				OutputType string   `json:"output_type"`
				Text       []string `json:"text"`
			} `json:"outputs"`
		} `json:"cells"`
	}

	if err := json.Unmarshal(data, &notebook); err != nil {
		return NewErrorResult(fmt.Errorf("invalid notebook format: %w", err)), nil
	}

	var content strings.Builder
	for i, cell := range notebook.Cells {
		content.WriteString(fmt.Sprintf("--- Cell %d (%s) ---\n", i+1, cell.CellType))
		for _, line := range cell.Source {
			content.WriteString(line)
		}
		content.WriteString("\n")

		// Include outputs for code cells
		if cell.CellType == "code" && len(cell.Outputs) > 0 {
			content.WriteString("Output:\n")
			for _, output := range cell.Outputs {
				for _, line := range output.Text {
					content.WriteString(line)
				}
			}
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	return NewResult(content.String()), nil
}

func isImageExtension(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	default:
		return false
	}
}

func getMediaType(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
