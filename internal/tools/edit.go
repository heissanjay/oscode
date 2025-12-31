package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
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
			"Performs string replacements in files. Uses intelligent matching that handles whitespace and indentation variations. The old_string should match the content you want to replace. Use replace_all to replace all occurrences.",
			BuildSchema(map[string]interface{}{
				"file_path":   StringProperty("The absolute path to the file to edit", true),
				"old_string":  StringProperty("The text to replace (handles minor whitespace variations)", true),
				"new_string":  StringProperty("The text to replace it with", true),
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

// Replacer interface for different matching strategies
type Replacer interface {
	Name() string
	Find(content, oldString string, replaceAll bool) []Match
}

// Match represents a found match
type Match struct {
	Start int
	End   int
	Text  string
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

	// Check if file was read (warn but allow for flexibility)
	if !t.HasReadFile(filePath) {
		// Mark it as read now since we're about to read it
		t.MarkFileRead(filePath)
	}

	// Read current content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to read file: %w", err)), nil
	}

	contentStr := string(content)

	// Try different matching strategies (like opencode)
	replacers := []Replacer{
		&ExactReplacer{},
		&LineTrimmedReplacer{},
		&WhitespaceNormalizedReplacer{},
		&IndentationFlexibleReplacer{},
		&BlockAnchorReplacer{},
		&FuzzyLineReplacer{},
	}

	var matches []Match
	var usedReplacer string

	for _, replacer := range replacers {
		matches = replacer.Find(contentStr, params.OldString, params.ReplaceAll)
		if len(matches) > 0 {
			usedReplacer = replacer.Name()
			break
		}
	}

	if len(matches) == 0 {
		// Provide helpful error message
		return NewErrorResultString(fmt.Sprintf(
			"Could not find a match for the text in %s. Tried 6 matching strategies.\n"+
				"Tips:\n"+
				"- Make sure the text exists in the file\n"+
				"- Check for invisible characters or encoding issues\n"+
				"- Include more surrounding context\n"+
				"- Use the Read tool to verify the exact content",
			params.FilePath)), nil
	}

	// Check for uniqueness if not replacing all
	if !params.ReplaceAll && len(matches) > 1 {
		return NewErrorResultString(fmt.Sprintf(
			"Found %d occurrences in %s. Either provide more context to make it unique, or set replace_all to true.",
			len(matches), params.FilePath)), nil
	}

	// Perform replacement (in reverse order to preserve positions)
	newContent := contentStr
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		newContent = newContent[:m.Start] + params.NewString + newContent[m.End:]
	}

	// Write back
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return NewErrorResult(fmt.Errorf("failed to write file: %w", err)), nil
	}

	// Calculate diff stats
	oldLines := strings.Count(params.OldString, "\n") + 1
	newLines := strings.Count(params.NewString, "\n") + 1
	added := 0
	removed := 0
	if newLines > oldLines {
		added = newLines - oldLines
	} else if oldLines > newLines {
		removed = oldLines - newLines
	}

	var msg string
	if params.ReplaceAll {
		msg = fmt.Sprintf("✓ Replaced %d occurrence(s) in %s", len(matches), params.FilePath)
	} else {
		msg = fmt.Sprintf("✓ Replaced 1 occurrence in %s", params.FilePath)
	}

	if usedReplacer != "exact" {
		msg += fmt.Sprintf(" (matched via %s)", usedReplacer)
	}

	if added > 0 {
		msg += fmt.Sprintf(" [+%d lines]", added)
	}
	if removed > 0 {
		msg += fmt.Sprintf(" [-%d lines]", removed)
	}

	result := NewResult(msg)
	result.WithMetadata("replacements", len(matches))
	result.WithMetadata("file", params.FilePath)
	result.WithMetadata("strategy", usedReplacer)

	return result, nil
}

// ExactReplacer - direct exact match
type ExactReplacer struct{}

func (r *ExactReplacer) Name() string { return "exact" }

func (r *ExactReplacer) Find(content, oldString string, replaceAll bool) []Match {
	var matches []Match
	start := 0
	for {
		idx := strings.Index(content[start:], oldString)
		if idx == -1 {
			break
		}
		absIdx := start + idx
		matches = append(matches, Match{
			Start: absIdx,
			End:   absIdx + len(oldString),
			Text:  oldString,
		})
		if !replaceAll {
			break
		}
		start = absIdx + len(oldString)
	}
	return matches
}

// LineTrimmedReplacer - matches lines after trimming whitespace
type LineTrimmedReplacer struct{}

func (r *LineTrimmedReplacer) Name() string { return "line-trimmed" }

func (r *LineTrimmedReplacer) Find(content, oldString string, replaceAll bool) []Match {
	oldLines := strings.Split(oldString, "\n")
	contentLines := strings.Split(content, "\n")

	// Trim each line for comparison
	trimmedOld := make([]string, len(oldLines))
	for i, line := range oldLines {
		trimmedOld[i] = strings.TrimSpace(line)
	}

	var matches []Match
	pos := 0

	for i := 0; i <= len(contentLines)-len(oldLines); i++ {
		match := true
		for j, trimmed := range trimmedOld {
			if strings.TrimSpace(contentLines[i+j]) != trimmed {
				match = false
				break
			}
		}

		if match {
			// Calculate actual byte positions
			start := 0
			for k := 0; k < i; k++ {
				start += len(contentLines[k]) + 1 // +1 for newline
			}
			end := start
			for k := 0; k < len(oldLines); k++ {
				end += len(contentLines[i+k])
				if i+k < len(contentLines)-1 {
					end++ // +1 for newline
				}
			}

			// Get the actual text from content
			if end > len(content) {
				end = len(content)
			}
			matches = append(matches, Match{
				Start: start,
				End:   end,
				Text:  content[start:end],
			})

			if !replaceAll {
				return matches
			}
			i += len(oldLines) - 1
		}
		pos++
	}

	return matches
}

// WhitespaceNormalizedReplacer - normalizes all whitespace to single spaces
type WhitespaceNormalizedReplacer struct{}

func (r *WhitespaceNormalizedReplacer) Name() string { return "whitespace-normalized" }

func (r *WhitespaceNormalizedReplacer) Find(content, oldString string, replaceAll bool) []Match {
	normalizeWS := func(s string) string {
		return strings.Join(strings.Fields(s), " ")
	}

	normalizedOld := normalizeWS(oldString)
	if normalizedOld == "" {
		return nil
	}

	// We need to find regions in content that normalize to the same thing
	// This is expensive, so we use a sliding window approach
	words := strings.Fields(oldString)
	if len(words) == 0 {
		return nil
	}

	var matches []Match
	contentLen := len(content)

	// Find potential start positions (where first word appears)
	firstWord := words[0]
	start := 0

	for start < contentLen {
		idx := strings.Index(content[start:], firstWord)
		if idx == -1 {
			break
		}
		absStart := start + idx

		// Try to match from here
		// Find the end of the matching region
		pos := absStart
		wordIdx := 0

		for wordIdx < len(words) && pos < contentLen {
			// Skip whitespace
			for pos < contentLen && unicode.IsSpace(rune(content[pos])) {
				pos++
			}

			// Try to match word
			word := words[wordIdx]
			if pos+len(word) <= contentLen && content[pos:pos+len(word)] == word {
				pos += len(word)
				wordIdx++
			} else {
				break
			}
		}

		if wordIdx == len(words) {
			// Found a match
			matches = append(matches, Match{
				Start: absStart,
				End:   pos,
				Text:  content[absStart:pos],
			})

			if !replaceAll {
				return matches
			}
			start = pos
		} else {
			start = absStart + 1
		}
	}

	return matches
}

// IndentationFlexibleReplacer - ignores leading indentation differences
type IndentationFlexibleReplacer struct{}

func (r *IndentationFlexibleReplacer) Name() string { return "indentation-flexible" }

func (r *IndentationFlexibleReplacer) Find(content, oldString string, replaceAll bool) []Match {
	oldLines := strings.Split(oldString, "\n")
	contentLines := strings.Split(content, "\n")

	// Extract content without leading whitespace
	trimContent := func(lines []string) []string {
		result := make([]string, len(lines))
		for i, line := range lines {
			result[i] = strings.TrimLeft(line, " \t")
		}
		return result
	}

	trimmedOld := trimContent(oldLines)

	var matches []Match

	for i := 0; i <= len(contentLines)-len(oldLines); i++ {
		match := true
		for j, trimmed := range trimmedOld {
			if strings.TrimLeft(contentLines[i+j], " \t") != trimmed {
				match = false
				break
			}
		}

		if match {
			// Calculate byte positions
			start := 0
			for k := 0; k < i; k++ {
				start += len(contentLines[k]) + 1
			}
			end := start
			for k := 0; k < len(oldLines); k++ {
				end += len(contentLines[i+k])
				if i+k < len(contentLines)-1 {
					end++
				}
			}

			if end > len(content) {
				end = len(content)
			}
			matches = append(matches, Match{
				Start: start,
				End:   end,
				Text:  content[start:end],
			})

			if !replaceAll {
				return matches
			}
			i += len(oldLines) - 1
		}
	}

	return matches
}

// BlockAnchorReplacer - uses first and last lines as anchors
type BlockAnchorReplacer struct{}

func (r *BlockAnchorReplacer) Name() string { return "block-anchor" }

func (r *BlockAnchorReplacer) Find(content, oldString string, replaceAll bool) []Match {
	oldLines := strings.Split(oldString, "\n")
	if len(oldLines) < 2 {
		return nil // Need at least 2 lines for anchoring
	}

	contentLines := strings.Split(content, "\n")
	firstAnchor := strings.TrimSpace(oldLines[0])
	lastAnchor := strings.TrimSpace(oldLines[len(oldLines)-1])

	if firstAnchor == "" || lastAnchor == "" {
		return nil
	}

	var matches []Match

	for i := 0; i < len(contentLines); i++ {
		if strings.TrimSpace(contentLines[i]) != firstAnchor {
			continue
		}

		// Found first anchor, look for last anchor
		expectedEnd := i + len(oldLines) - 1
		if expectedEnd >= len(contentLines) {
			continue
		}

		if strings.TrimSpace(contentLines[expectedEnd]) != lastAnchor {
			continue
		}

		// Calculate byte positions
		start := 0
		for k := 0; k < i; k++ {
			start += len(contentLines[k]) + 1
		}
		end := start
		for k := 0; k < len(oldLines); k++ {
			end += len(contentLines[i+k])
			if i+k < len(contentLines)-1 {
				end++
			}
		}

		if end > len(content) {
			end = len(content)
		}
		matches = append(matches, Match{
			Start: start,
			End:   end,
			Text:  content[start:end],
		})

		if !replaceAll {
			return matches
		}
		i = expectedEnd
	}

	return matches
}

// FuzzyLineReplacer - uses regex patterns for flexible matching
type FuzzyLineReplacer struct{}

func (r *FuzzyLineReplacer) Name() string { return "fuzzy-line" }

func (r *FuzzyLineReplacer) Find(content, oldString string, replaceAll bool) []Match {
	// Build a regex pattern from the old string
	// Escape regex special characters but allow flexible whitespace
	oldLines := strings.Split(oldString, "\n")
	var patternParts []string

	for _, line := range oldLines {
		// Escape special regex characters
		escaped := regexp.QuoteMeta(strings.TrimSpace(line))
		// Allow flexible leading whitespace
		patternParts = append(patternParts, `[ \t]*`+escaped)
	}

	pattern := strings.Join(patternParts, `\n`)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	var matches []Match
	results := re.FindAllStringIndex(content, -1)

	for _, result := range results {
		matches = append(matches, Match{
			Start: result[0],
			End:   result[1],
			Text:  content[result[0]:result[1]],
		})
		if !replaceAll {
			break
		}
	}

	return matches
}

// CreateDiff generates a unified diff representation
func CreateDiff(oldContent, newContent, filePath string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- %s\n", filePath))
	diff.WriteString(fmt.Sprintf("+++ %s\n", filePath))

	// Simple line-by-line diff
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
