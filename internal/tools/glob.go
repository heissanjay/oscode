package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// GlobInput defines the input for the Glob tool
type GlobInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

// GlobTool finds files matching glob patterns
type GlobTool struct {
	BaseTool
	workDir string
}

// NewGlobTool creates a new Glob tool
func NewGlobTool(workDir string) *GlobTool {
	return &GlobTool{
		BaseTool: NewBaseTool(
			"Glob",
			"Fast file pattern matching tool. Supports glob patterns like '**/*.js' or 'src/**/*.ts'. Returns matching file paths sorted by modification time.",
			BuildSchema(map[string]interface{}{
				"pattern": StringProperty("The glob pattern to match files against (e.g., '**/*.go', 'src/**/*.ts')", true),
				"path":    StringProperty("Directory to search in (defaults to current working directory)", false),
			}, []string{"pattern"}),
			false, // Glob doesn't require permission
			CategorySearch,
		),
		workDir: workDir,
	}
}

func (t *GlobTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params GlobInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if params.Pattern == "" {
		return NewErrorResultString("pattern is required"), nil
	}

	// Determine search path
	searchPath := t.workDir
	if params.Path != "" {
		if filepath.IsAbs(params.Path) {
			searchPath = params.Path
		} else {
			searchPath = filepath.Join(t.workDir, params.Path)
		}
	}

	// Verify path exists
	if _, err := os.Stat(searchPath); os.IsNotExist(err) {
		return NewErrorResultString(fmt.Sprintf("Path does not exist: %s", searchPath)), nil
	}

	// Build full pattern
	fullPattern := filepath.Join(searchPath, params.Pattern)
	// Normalize separators for doublestar
	fullPattern = filepath.ToSlash(fullPattern)

	// Find matches
	matches, err := doublestar.FilepathGlob(fullPattern)
	if err != nil {
		return NewErrorResult(fmt.Errorf("glob error: %w", err)), nil
	}

	if len(matches) == 0 {
		return NewResult("No files matched the pattern"), nil
	}

	// Get file info and sort by modification time
	type fileWithTime struct {
		path    string
		modTime int64
	}

	files := make([]fileWithTime, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		// Skip directories
		if info.IsDir() {
			continue
		}

		// Make path relative to work dir
		relPath, err := filepath.Rel(t.workDir, match)
		if err != nil {
			relPath = match
		}

		files = append(files, fileWithTime{
			path:    relPath,
			modTime: info.ModTime().Unix(),
		})
	}

	// Sort by modification time (most recent first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	// Build output
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(f.path)
		sb.WriteString("\n")
	}

	result := NewResult(strings.TrimSuffix(sb.String(), "\n"))
	result.WithMetadata("count", len(files))

	return result, nil
}

// ExpandGlob expands a glob pattern and returns matching files
func ExpandGlob(pattern, baseDir string) ([]string, error) {
	fullPattern := filepath.Join(baseDir, pattern)
	fullPattern = filepath.ToSlash(fullPattern)

	matches, err := doublestar.FilepathGlob(fullPattern)
	if err != nil {
		return nil, err
	}

	// Filter out directories and make relative
	var files []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || info.IsDir() {
			continue
		}

		relPath, err := filepath.Rel(baseDir, match)
		if err != nil {
			relPath = match
		}
		files = append(files, relPath)
	}

	return files, nil
}
