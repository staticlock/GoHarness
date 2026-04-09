package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/staticlock/GoHarness/internal/services/lsp"
)

type LspToolInput struct {
	Operation string `json:"operation"`
	FilePath  string `json:"file_path,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	Line      *int   `json:"line,omitempty"`
	Character *int   `json:"character,omitempty"`
	Query     string `json:"query,omitempty"`
}

type LspTool struct{}

func (t *LspTool) Name() string { return "lsp" }
func (t *LspTool) Description() string {
	return "Inspect Python code symbols, definitions, references, and hover information across the current workspace."
}
func (t *LspTool) IsReadOnly() bool { return true }

func (t *LspTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "The code intelligence operation to perform",
				"enum":        []string{"document_symbol", "workspace_symbol", "go_to_definition", "find_references", "hover"},
			},
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the source file for file-based operations",
			},
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "Explicit symbol name to look up",
			},
			"line": map[string]interface{}{
				"type":        "integer",
				"description": "1-based line number for position-based lookups",
			},
			"character": map[string]interface{}{
				"type":        "integer",
				"description": "1-based character offset for position-based lookups",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Substring query for workspace_symbol",
			},
		},
		"required": []string{"operation"},
	}
}

func (t *LspTool) Execute(ctx context.Context, args json.RawMessage, execCtx ToolExecutionContext) (ToolResult, error) {
	var input LspToolInput
	if err := json.Unmarshal(args, &input); err != nil {
		return NewErrorResult(err), nil
	}

	if input.Operation == "workspace_symbol" {
		if input.Query == "" {
			return NewErrorResult(fmt.Errorf("workspace_symbol requires query")), nil
		}
		results, err := lsp.WorkspaceSymbolSearch(execCtx.CWD, input.Query)
		if err != nil {
			return NewErrorResult(err), nil
		}
		return formatSymbolResults(results, execCtx.CWD), nil
	}

	if input.FilePath == "" {
		return NewErrorResult(fmt.Errorf("%s requires file_path", input.Operation)), nil
	}

	filePath := resolvePath(execCtx.CWD, input.FilePath)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return NewErrorResult(fmt.Errorf("file not found: %s", filePath)), nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".go" {
		return NewErrorResult(fmt.Errorf("the lsp tool currently supports Go files only.")), nil
	}

	switch input.Operation {
	case "document_symbol":
		results, err := lsp.ListDocumentSymbols(filePath)
		if err != nil {
			return NewErrorResult(err), nil
		}
		return formatSymbolResults(results, execCtx.CWD), nil

	case "go_to_definition":
		results, err := lsp.GoToDefinition(filePath, input.Symbol, input.Line, input.Character)
		if err != nil {
			return NewErrorResult(err), nil
		}
		return formatSymbolResults(results, execCtx.CWD), nil

	case "find_references":
		results, err := lsp.FindReferences(execCtx.CWD, filePath, input.Symbol, input.Line, input.Character)
		if err != nil {
			return NewErrorResult(err), nil
		}
		return formatReferenceResults(results, execCtx.CWD), nil

	case "hover":
		result, err := lsp.Hover(filePath, input.Symbol, input.Line, input.Character)
		if err != nil {
			return NewErrorResult(err), nil
		}
		if result == nil {
			return NewSuccessResult("(no hover result)"), nil
		}
		output := fmt.Sprintf("%s %s\npath: %s:%d:%d", result.Kind, result.Name, result.Path, result.Line, result.Character)
		if result.Signature != "" {
			output += "\nsignature: " + result.Signature
		}
		if result.Docstring != "" {
			output += "\ndocstring: " + result.Docstring
		}
		return NewSuccessResult(output), nil

	default:
		return NewErrorResult(fmt.Errorf("unsupported operation: %s", input.Operation)), nil
	}
}

func formatSymbolResults(results []lsp.SymbolResult, root string) ToolResult {
	if len(results) == 0 {
		return NewSuccessResult("(no results)")
	}
	var lines []string
	for _, r := range results {
		relPath, _ := filepath.Rel(root, r.Path)
		line := fmt.Sprintf("%s %s - %s:%d:%d", r.Kind, r.Name, relPath, r.Line, r.Character)
		lines = append(lines, line)
		if r.Signature != "" {
			lines = append(lines, "  signature: "+r.Signature)
		}
		if r.Docstring != "" {
			lines = append(lines, "  docstring: "+r.Docstring)
		}
	}
	return NewSuccessResult(strings.Join(lines, "\n"))
}

func formatReferenceResults(results []lsp.ReferenceResult, root string) ToolResult {
	if len(results) == 0 {
		return NewSuccessResult("(no results)")
	}
	var lines []string
	for _, r := range results {
		relPath, _ := filepath.Rel(root, r.Path)
		lines = append(lines, fmt.Sprintf("%s:%d:%s", relPath, r.Line, r.Text))
	}
	return NewSuccessResult(strings.Join(lines, "\n"))
}
