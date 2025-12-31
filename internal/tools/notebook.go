package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NotebookEditInput defines input for the NotebookEdit tool
type NotebookEditInput struct {
	NotebookPath string `json:"notebook_path"`
	CellID       string `json:"cell_id,omitempty"`
	CellNumber   int    `json:"cell_number,omitempty"`
	NewSource    string `json:"new_source"`
	CellType     string `json:"cell_type,omitempty"` // code, markdown
	EditMode     string `json:"edit_mode,omitempty"` // replace, insert, delete
}

// Notebook represents a Jupyter notebook structure
type Notebook struct {
	Cells    []NotebookCell         `json:"cells"`
	Metadata map[string]interface{} `json:"metadata"`
	NBFormat int                    `json:"nbformat"`
	NBFormatMinor int               `json:"nbformat_minor"`
}

// NotebookCell represents a single cell in a notebook
type NotebookCell struct {
	CellType       string                 `json:"cell_type"`
	Source         interface{}            `json:"source"` // Can be string or []string
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	ExecutionCount *int                   `json:"execution_count,omitempty"`
	Outputs        []interface{}          `json:"outputs,omitempty"`
	ID             string                 `json:"id,omitempty"`
}

// NotebookEditTool edits Jupyter notebook cells
type NotebookEditTool struct {
	BaseTool
	workDir string
}

// NewNotebookEditTool creates a new NotebookEdit tool
func NewNotebookEditTool(workDir string) *NotebookEditTool {
	return &NotebookEditTool{
		BaseTool: NewBaseTool(
			"NotebookEdit",
			`Edits cells in Jupyter notebooks (.ipynb files).

Operations:
- replace: Replace the contents of an existing cell
- insert: Insert a new cell at the specified position
- delete: Delete the cell at the specified position

Cell can be identified by cell_id or cell_number (0-indexed).`,
			BuildSchema(map[string]interface{}{
				"notebook_path": StringProperty("Absolute path to the Jupyter notebook", true),
				"cell_id":       StringProperty("ID of the cell to edit (use this or cell_number)", false),
				"cell_number":   IntProperty("0-indexed cell number to edit"),
				"new_source":    StringProperty("New source code/markdown for the cell", true),
				"cell_type":     StringProperty("Cell type: 'code' or 'markdown' (required for insert)", false),
				"edit_mode":     StringProperty("Edit mode: 'replace' (default), 'insert', or 'delete'", false),
			}, []string{"notebook_path", "new_source"}),
			true, // Requires permission
			CategoryFile,
		),
		workDir: workDir,
	}
}

func (t *NotebookEditTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params NotebookEditInput
	if err := json.Unmarshal(input, &params); err != nil {
		return NewErrorResult(fmt.Errorf("invalid input: %w", err)), nil
	}

	// Validate notebook path
	path := params.NotebookPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workDir, path)
	}

	if !strings.HasSuffix(strings.ToLower(path), ".ipynb") {
		return NewErrorResultString("File must be a Jupyter notebook (.ipynb)"), nil
	}

	// Default edit mode
	editMode := params.EditMode
	if editMode == "" {
		editMode = "replace"
	}

	// Read notebook
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && editMode == "insert" {
			// Create new notebook for insert
			return t.createNewNotebook(path, params)
		}
		return NewErrorResult(fmt.Errorf("failed to read notebook: %w", err)), nil
	}

	var notebook Notebook
	if err := json.Unmarshal(data, &notebook); err != nil {
		return NewErrorResult(fmt.Errorf("failed to parse notebook: %w", err)), nil
	}

	// Find cell index
	cellIndex := -1
	if params.CellID != "" {
		for i, cell := range notebook.Cells {
			if cell.ID == params.CellID {
				cellIndex = i
				break
			}
		}
		if cellIndex == -1 && editMode != "insert" {
			return NewErrorResultString(fmt.Sprintf("Cell with ID '%s' not found", params.CellID)), nil
		}
	} else {
		cellIndex = params.CellNumber
	}

	// Perform operation
	switch editMode {
	case "replace":
		if cellIndex < 0 || cellIndex >= len(notebook.Cells) {
			return NewErrorResultString(fmt.Sprintf("Cell index %d out of range (0-%d)", cellIndex, len(notebook.Cells)-1)), nil
		}
		notebook.Cells[cellIndex].Source = t.formatSource(params.NewSource)
		if params.CellType != "" {
			notebook.Cells[cellIndex].CellType = params.CellType
		}

	case "insert":
		cellType := params.CellType
		if cellType == "" {
			cellType = "code"
		}
		newCell := NotebookCell{
			CellType: cellType,
			Source:   t.formatSource(params.NewSource),
			Metadata: make(map[string]interface{}),
		}
		if cellType == "code" {
			newCell.Outputs = []interface{}{}
		}

		if cellIndex < 0 {
			// Insert at end
			notebook.Cells = append(notebook.Cells, newCell)
		} else if cellIndex >= len(notebook.Cells) {
			notebook.Cells = append(notebook.Cells, newCell)
		} else {
			// Insert at position
			notebook.Cells = append(notebook.Cells[:cellIndex+1], notebook.Cells[cellIndex:]...)
			notebook.Cells[cellIndex] = newCell
		}

	case "delete":
		if cellIndex < 0 || cellIndex >= len(notebook.Cells) {
			return NewErrorResultString(fmt.Sprintf("Cell index %d out of range", cellIndex)), nil
		}
		notebook.Cells = append(notebook.Cells[:cellIndex], notebook.Cells[cellIndex+1:]...)

	default:
		return NewErrorResultString(fmt.Sprintf("Unknown edit mode: %s", editMode)), nil
	}

	// Write notebook back
	output, err := json.MarshalIndent(notebook, "", " ")
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to serialize notebook: %w", err)), nil
	}

	if err := os.WriteFile(path, output, 0644); err != nil {
		return NewErrorResult(fmt.Errorf("failed to write notebook: %w", err)), nil
	}

	result := NewResult(fmt.Sprintf("Successfully %sed cell in notebook", editMode))
	result.WithMetadata("notebook_path", path)
	result.WithMetadata("operation", editMode)
	result.WithMetadata("cell_count", len(notebook.Cells))
	return result, nil
}

func (t *NotebookEditTool) formatSource(source string) []string {
	// Jupyter notebooks store source as array of lines
	lines := strings.Split(source, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		if i < len(lines)-1 {
			result[i] = line + "\n"
		} else {
			result[i] = line
		}
	}
	return result
}

func (t *NotebookEditTool) createNewNotebook(path string, params NotebookEditInput) (*Result, error) {
	cellType := params.CellType
	if cellType == "" {
		cellType = "code"
	}

	notebook := Notebook{
		Cells: []NotebookCell{
			{
				CellType: cellType,
				Source:   t.formatSource(params.NewSource),
				Metadata: make(map[string]interface{}),
				Outputs:  []interface{}{},
			},
		},
		Metadata: map[string]interface{}{
			"kernelspec": map[string]interface{}{
				"display_name": "Python 3",
				"language":     "python",
				"name":         "python3",
			},
			"language_info": map[string]interface{}{
				"name": "python",
			},
		},
		NBFormat:      4,
		NBFormatMinor: 5,
	}

	// Create parent directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewErrorResult(fmt.Errorf("failed to create directory: %w", err)), nil
	}

	output, err := json.MarshalIndent(notebook, "", " ")
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to serialize notebook: %w", err)), nil
	}

	if err := os.WriteFile(path, output, 0644); err != nil {
		return NewErrorResult(fmt.Errorf("failed to write notebook: %w", err)), nil
	}

	result := NewResult("Created new notebook with initial cell")
	result.WithMetadata("notebook_path", path)
	result.WithMetadata("operation", "create")
	return result, nil
}
