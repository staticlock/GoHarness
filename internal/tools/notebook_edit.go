package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// NotebookEditTool edits Jupyter notebook cells.
type NotebookEditTool struct{}

func (t *NotebookEditTool) Name() string        { return "notebook_edit" }
func (t *NotebookEditTool) Description() string { return "Create or edit a Jupyter notebook cell." }
func (t *NotebookEditTool) IsReadOnly() bool    { return false }

func (t *NotebookEditTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the .ipynb file",
			},
			"cell_index": map[string]interface{}{
				"type":        "integer",
				"description": "Zero-based cell index",
			},
			"new_source": map[string]interface{}{
				"type":        "string",
				"description": "Replacement or appended source for the target cell",
			},
			"cell_type": map[string]interface{}{
				"type":        "string",
				"description": "Cell type: code or markdown",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"description": "Mode: replace or append",
			},
			"create_if_missing": map[string]interface{}{
				"type":        "boolean",
				"description": "Create notebook if missing",
			},
		},
		"required": []string{"path", "cell_index", "new_source"},
	}
}

type notebookEditInput struct {
	Path            string `json:"path"`
	CellIndex       int    `json:"cell_index"`
	NewSource       string `json:"new_source"`
	CellType        string `json:"cell_type,omitempty"`
	Mode            string `json:"mode,omitempty"`
	CreateIfMissing *bool  `json:"create_if_missing,omitempty"`
}

func (t *NotebookEditTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input notebookEditInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	if input.CellType == "" {
		input.CellType = "code"
	}
	if input.Mode == "" {
		input.Mode = "replace"
	}

	createIfMissing := true
	if input.CreateIfMissing != nil {
		createIfMissing = *input.CreateIfMissing
	}

	path := resolvePath(execCtx.CWD, input.Path)
	notebook, err := loadNotebook(path, createIfMissing)
	if err != nil {
		return NewErrorResult(fmt.Errorf("notebook not found: %s", path)), nil
	}

	cells, ok := notebook["cells"].([]interface{})
	if !ok {
		cells = []interface{}{}
		notebook["cells"] = cells
	}

	cellSlice := make([]map[string]interface{}, len(cells))
	for i, c := range cells {
		if cm, ok := c.(map[string]interface{}); ok {
			cellSlice[i] = cm
		}
	}

	for len(cellSlice) <= input.CellIndex {
		cellSlice = append(cellSlice, emptyCell(input.CellType))
	}

	cell := cellSlice[input.CellIndex]
	cell["cell_type"] = input.CellType
	if _, ok := cell["metadata"]; !ok {
		cell["metadata"] = map[string]interface{}{}
	}
	if input.CellType == "code" {
		if _, ok := cell["outputs"]; !ok {
			cell["outputs"] = []interface{}{}
		}
		if _, ok := cell["execution_count"]; !ok {
			cell["execution_count"] = nil
		}
	}

	existing := normalizeSource(cell["source"])
	var updated string
	if input.Mode == "append" {
		updated = existing + input.NewSource
	} else {
		updated = input.NewSource
	}
	cell["source"] = updated

	cellSlice[input.CellIndex] = cell
	notebook["cells"] = cellSlice

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return NewErrorResult(err), nil
	}

	data, err := json.MarshalIndent(notebook, "", "  ")
	if err != nil {
		return NewErrorResult(err), nil
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return NewErrorResult(err), nil
	}

	return NewSuccessResult(fmt.Sprintf("Updated notebook cell %d in %s", input.CellIndex, path)), nil
}

func loadNotebook(path string, createIfMissing bool) (map[string]interface{}, error) {
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var notebook map[string]interface{}
		if err := json.Unmarshal(data, &notebook); err != nil {
			return nil, err
		}
		return notebook, nil
	}
	if !createIfMissing {
		return nil, fmt.Errorf("not found")
	}
	return map[string]interface{}{
		"cells":          []interface{}{},
		"metadata":       map[string]interface{}{"language_info": map[string]interface{}{"name": "python"}},
		"nbformat":       4,
		"nbformat_minor": 5,
	}, nil
}

func emptyCell(cellType string) map[string]interface{} {
	if cellType == "markdown" {
		return map[string]interface{}{
			"cell_type": "markdown",
			"metadata":  map[string]interface{}{},
			"source":    "",
		}
	}
	return map[string]interface{}{
		"cell_type":       "code",
		"metadata":        map[string]interface{}{},
		"source":          "",
		"outputs":         []interface{}{},
		"execution_count": nil,
	}
}

func normalizeSource(source interface{}) string {
	if s, ok := source.(string); ok {
		return s
	}
	if arr, ok := source.([]interface{}); ok {
		result := ""
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result += s
			}
		}
		return result
	}
	return ""
}
