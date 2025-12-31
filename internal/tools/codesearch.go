package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CodeSearchInput defines the input for the CodeSearch tool
type CodeSearchInput struct {
	Query       string   `json:"query"`
	FilePattern string   `json:"file_pattern,omitempty"`
	MaxResults  int      `json:"max_results,omitempty"`
	SearchType  string   `json:"search_type,omitempty"` // "symbol", "definition", "reference", "text"
}

// CodeSearchResult represents a search result
type CodeSearchResult struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Match      string `json:"match"`
	Context    string `json:"context"`
	SymbolType string `json:"symbol_type,omitempty"`
}

// CodeSearchTool searches for code patterns
type CodeSearchTool struct {
	BaseTool
	workDir string
}

// NewCodeSearchTool creates a new CodeSearch tool
func NewCodeSearchTool(workDir string) *CodeSearchTool {
	return &CodeSearchTool{
		BaseTool: NewBaseTool(
			"CodeSearch",
			"Search for code patterns, symbols, definitions, and references. Supports regex patterns and filtering by file type. Use search_type='symbol' for function/class names, 'definition' for declarations, 'reference' for usages.",
			BuildSchema(map[string]interface{}{
				"query":        StringProperty("The search pattern (supports regex)", true),
				"file_pattern": StringProperty("Glob pattern for files to search (e.g., '*.go', '**/*.ts')", false),
				"max_results":  IntProperty("Maximum number of results (default: 50)"),
				"search_type":  StringProperty("Type of search: 'text', 'symbol', 'definition', 'reference' (default: 'text')", false),
			}, []string{"query"}),
			false, // CodeSearch doesn't require permission
			CategorySearch,
		),
		workDir: workDir,
	}
}

func (t *CodeSearchTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params CodeSearchInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	if params.Query == "" {
		return NewErrorResultString("query is required"), nil
	}

	if params.MaxResults <= 0 {
		params.MaxResults = 50
	}

	if params.SearchType == "" {
		params.SearchType = "text"
	}

	// Build regex pattern based on search type
	var pattern *regexp.Regexp
	var err error

	switch params.SearchType {
	case "symbol":
		// Match word boundaries for symbol search
		pattern, err = regexp.Compile(`\b` + regexp.QuoteMeta(params.Query) + `\b`)
	case "definition":
		// Match common definition patterns
		pattern, err = t.buildDefinitionPattern(params.Query)
	case "reference":
		// Match usages (not definitions)
		pattern, err = regexp.Compile(`\b` + regexp.QuoteMeta(params.Query) + `\b`)
	default:
		// Text search with regex support
		pattern, err = regexp.Compile(params.Query)
	}

	if err != nil {
		return NewErrorResult(fmt.Errorf("invalid search pattern: %w", err)), nil
	}

	// Find files to search
	files, err := t.findFiles(params.FilePattern)
	if err != nil {
		return NewErrorResult(err), nil
	}

	// Search files
	var results []CodeSearchResult
	for _, file := range files {
		if len(results) >= params.MaxResults {
			break
		}

		fileResults, err := t.searchFile(file, pattern, params.SearchType, params.MaxResults-len(results))
		if err != nil {
			continue // Skip files with errors
		}
		results = append(results, fileResults...)
	}

	if len(results) == 0 {
		return NewResult(fmt.Sprintf("No matches found for '%s'", params.Query)), nil
	}

	// Format output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d result(s) for '%s':\n\n", len(results), params.Query))

	for i, r := range results {
		relPath, _ := filepath.Rel(t.workDir, r.File)
		if relPath == "" {
			relPath = r.File
		}

		sb.WriteString(fmt.Sprintf("%d. %s:%d:%d\n", i+1, relPath, r.Line, r.Column))
		if r.SymbolType != "" {
			sb.WriteString(fmt.Sprintf("   Type: %s\n", r.SymbolType))
		}
		sb.WriteString(fmt.Sprintf("   %s\n", strings.TrimSpace(r.Context)))
		sb.WriteString("\n")
	}

	result := NewResult(sb.String())
	result.WithMetadata("count", len(results))
	result.WithMetadata("query", params.Query)

	return result, nil
}

func (t *CodeSearchTool) findFiles(pattern string) ([]string, error) {
	if pattern == "" {
		// Default: search common source files
		pattern = "**/*.{go,js,ts,tsx,jsx,py,java,c,cpp,h,hpp,rs,rb,php,swift,kt,scala}"
	}

	var files []string

	// Simple glob implementation
	err := filepath.Walk(t.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			// Skip common non-source directories
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" ||
				name == ".idea" || name == "__pycache__" || name == ".next" ||
				name == "dist" || name == "build" || name == "target" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches pattern
		if matchesPattern(path, pattern, t.workDir) {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by modification time (most recent first)
	sort.Slice(files, func(i, j int) bool {
		iInfo, _ := os.Stat(files[i])
		jInfo, _ := os.Stat(files[j])
		if iInfo == nil || jInfo == nil {
			return false
		}
		return iInfo.ModTime().After(jInfo.ModTime())
	})

	return files, nil
}

func matchesPattern(path, pattern, workDir string) bool {
	relPath, _ := filepath.Rel(workDir, path)
	if relPath == "" {
		relPath = path
	}

	// Handle brace expansion like *.{go,js,ts}
	if strings.Contains(pattern, "{") {
		patterns := expandBraces(pattern)
		for _, p := range patterns {
			if matchGlob(relPath, p) {
				return true
			}
		}
		return false
	}

	return matchGlob(relPath, pattern)
}

func expandBraces(pattern string) []string {
	// Find brace group
	start := strings.Index(pattern, "{")
	if start == -1 {
		return []string{pattern}
	}

	end := strings.Index(pattern[start:], "}")
	if end == -1 {
		return []string{pattern}
	}
	end += start

	prefix := pattern[:start]
	suffix := pattern[end+1:]
	options := strings.Split(pattern[start+1:end], ",")

	var results []string
	for _, opt := range options {
		results = append(results, expandBraces(prefix+opt+suffix)...)
	}

	return results
}

func matchGlob(path, pattern string) bool {
	// Convert glob to regex
	pattern = strings.ReplaceAll(pattern, ".", "\\.")
	pattern = strings.ReplaceAll(pattern, "**/", "(.*/)?")
	pattern = strings.ReplaceAll(pattern, "*", "[^/]*")
	pattern = strings.ReplaceAll(pattern, "?", ".")
	pattern = "^" + pattern + "$"

	matched, _ := regexp.MatchString(pattern, path)
	return matched
}

func (t *CodeSearchTool) searchFile(path string, pattern *regexp.Regexp, searchType string, maxResults int) ([]CodeSearchResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var results []CodeSearchResult

	for lineNum, line := range lines {
		if len(results) >= maxResults {
			break
		}

		matches := pattern.FindAllStringIndex(line, -1)
		for _, match := range matches {
			if len(results) >= maxResults {
				break
			}

			// For definition search, verify it's actually a definition
			if searchType == "definition" && !t.isDefinition(lines, lineNum, match[0]) {
				continue
			}

			// For reference search, skip if it's a definition
			if searchType == "reference" && t.isDefinition(lines, lineNum, match[0]) {
				continue
			}

			symbolType := ""
			if searchType == "definition" || searchType == "symbol" {
				symbolType = t.detectSymbolType(lines, lineNum, match[0])
			}

			results = append(results, CodeSearchResult{
				File:       path,
				Line:       lineNum + 1,
				Column:     match[0] + 1,
				Match:      line[match[0]:match[1]],
				Context:    line,
				SymbolType: symbolType,
			})
		}
	}

	return results, nil
}

func (t *CodeSearchTool) buildDefinitionPattern(query string) (*regexp.Regexp, error) {
	// Build pattern that matches common definition patterns
	escaped := regexp.QuoteMeta(query)
	patterns := []string{
		// Go
		`func\s+` + escaped + `\s*\(`,
		`func\s+\([^)]+\)\s+` + escaped + `\s*\(`,
		`type\s+` + escaped + `\s+`,
		`var\s+` + escaped + `\s*=`,
		`const\s+` + escaped + `\s*=`,
		// JavaScript/TypeScript
		`function\s+` + escaped + `\s*\(`,
		`const\s+` + escaped + `\s*=`,
		`let\s+` + escaped + `\s*=`,
		`class\s+` + escaped + `\s*`,
		`interface\s+` + escaped + `\s*`,
		// Python
		`def\s+` + escaped + `\s*\(`,
		`class\s+` + escaped + `\s*[\(:]`,
		// Java/C/C++
		`\b\w+\s+` + escaped + `\s*\(`,
		`#define\s+` + escaped + `\s`,
		`struct\s+` + escaped + `\s*\{`,
		`class\s+` + escaped + `\s*[\{:]`,
	}

	combined := "(" + strings.Join(patterns, "|") + ")"
	return regexp.Compile(combined)
}

func (t *CodeSearchTool) isDefinition(lines []string, lineNum, col int) bool {
	if lineNum >= len(lines) {
		return false
	}

	line := lines[lineNum]
	prefix := ""
	if col > 0 && col <= len(line) {
		prefix = strings.TrimSpace(line[:col])
	}

	// Check for definition keywords
	defKeywords := []string{
		"func ", "type ", "var ", "const ", "package ",
		"function ", "class ", "interface ", "enum ", "let ", "const ",
		"def ", "import ", "from ",
		"struct ", "typedef ", "#define ", "using ",
		"pub fn ", "fn ", "impl ", "trait ", "mod ",
	}

	for _, kw := range defKeywords {
		if strings.HasSuffix(prefix, strings.TrimSpace(kw)) || strings.HasPrefix(line, kw) {
			return true
		}
	}

	return false
}

func (t *CodeSearchTool) detectSymbolType(lines []string, lineNum, col int) string {
	if lineNum >= len(lines) {
		return ""
	}

	line := lines[lineNum]

	// Check for various patterns
	patterns := map[string][]string{
		"function": {`\bfunc\b`, `\bfunction\b`, `\bdef\b`, `\bfn\b`},
		"class":    {`\bclass\b`, `\bstruct\b`},
		"type":     {`\btype\b`, `\btypedef\b`, `\binterface\b`, `\btrait\b`},
		"variable": {`\bvar\b`, `\blet\b`, `\bconst\b`},
		"method":   {`\bfunc\s*\([^)]+\)`},
		"import":   {`\bimport\b`, `\buse\b`, `\brequire\b`},
	}

	for symbolType, regexps := range patterns {
		for _, pat := range regexps {
			if matched, _ := regexp.MatchString(pat, line); matched {
				return symbolType
			}
		}
	}

	return ""
}
