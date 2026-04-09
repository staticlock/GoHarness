package lsp

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type SymbolResult struct {
	Path      string
	Name      string
	Kind      string
	Line      int
	Character int
	Signature string
	Docstring string
}

type ReferenceResult struct {
	Path string
	Line int
	Text string
}

func ListDocumentSymbols(filePath string) ([]SymbolResult, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var results []SymbolResult
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name != nil {
				sig := signatureString(x.Type)
				results = append(results, SymbolResult{
					Path:      filePath,
					Name:      x.Name.Name,
					Kind:      "function",
					Line:      fset.Position(x.Pos()).Line,
					Character: fset.Position(x.Pos()).Column,
					Signature: sig,
					Docstring: getDocComment(x.Doc),
				})
			}
		case *ast.TypeSpec:
			if x.Name != nil {
				typeName := x.Name.Name
				kind := "type"
				if x.Type != nil {
					switch x.Type.(type) {
					case *ast.StructType:
						kind = "struct"
					case *ast.InterfaceType:
						kind = "interface"
					}
				}
				results = append(results, SymbolResult{
					Path:      filePath,
					Name:      typeName,
					Kind:      kind,
					Line:      fset.Position(x.Pos()).Line,
					Character: fset.Position(x.Pos()).Column,
					Docstring: getDocComment(x.Doc),
				})
			}
		}
		return true
	})

	return results, nil
}

func WorkspaceSymbolSearch(root, query string) ([]SymbolResult, error) {
	var results []SymbolResult
	query = strings.ToLower(query)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		symbols, err := ListDocumentSymbols(path)
		if err != nil {
			return nil
		}
		for _, s := range symbols {
			if strings.Contains(strings.ToLower(s.Name), query) {
				results = append(results, s)
			}
		}
		return nil
	})

	return results, err
}

func GoToDefinition(filePath, symbolName string, line, character *int) ([]SymbolResult, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var targetPos token.Pos
	if line != nil && character != nil {
		targetPos = token.NoPos
	} else if symbolName != "" {
		targetPos = findSymbolPos(node, fset, symbolName)
	}

	if !targetPos.IsValid() {
		return nil, fmt.Errorf("symbol not found: %s", symbolName)
	}

	pos := fset.Position(targetPos)
	return []SymbolResult{
		{
			Path:      filePath,
			Name:      symbolName,
			Kind:      "definition",
			Line:      pos.Line,
			Character: pos.Column,
		},
	}, nil
}

func FindReferences(root, filePath, symbolName string, line, character *int) ([]ReferenceResult, error) {
	var results []ReferenceResult

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var targetName string
	if symbolName != "" {
		targetName = symbolName
	} else if line != nil {
		targetName = findIdentifierAtLine(node, fset, *line)
	}

	if targetName == "" {
		return nil, fmt.Errorf("no symbol to find references for")
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		fileResults, err := findReferencesInFile(path, targetName)
		if err != nil {
			return nil
		}
		results = append(results, fileResults...)
		return nil
	})

	return results, err
}

func Hover(filePath, symbolName string, line, character *int) (*SymbolResult, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var targetName string
	if symbolName != "" {
		targetName = symbolName
	} else if line != nil {
		targetName = findIdentifierAtLine(node, fset, *line)
	}

	if targetName == "" {
		return nil, nil
	}

	var result *SymbolResult
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name != nil && x.Name.Name == targetName {
				sig := signatureString(x.Type)
				result = &SymbolResult{
					Path:      filePath,
					Name:      x.Name.Name,
					Kind:      "function",
					Line:      fset.Position(x.Pos()).Line,
					Character: fset.Position(x.Pos()).Column,
					Signature: sig,
					Docstring: getDocComment(x.Doc),
				}
				return false
			}
		case *ast.TypeSpec:
			if x.Name != nil && x.Name.Name == targetName {
				result = &SymbolResult{
					Path:      filePath,
					Name:      x.Name.Name,
					Kind:      "type",
					Line:      fset.Position(x.Pos()).Line,
					Character: fset.Position(x.Pos()).Column,
					Docstring: getDocComment(x.Doc),
				}
				return false
			}
		}
		return true
	})

	return result, nil
}

func signatureString(ft *ast.FuncType) string {
	if ft == nil {
		return "()"
	}
	return "()"
}

func getDocComment(c *ast.CommentGroup) string {
	if c == nil {
		return ""
	}
	text := c.Text()
	lines := strings.Split(text, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

func findSymbolPos(node *ast.File, fset *token.FileSet, name string) token.Pos {
	var pos token.Pos
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name != nil && x.Name.Name == name {
				pos = x.Name.Pos()
				return false
			}
		case *ast.TypeSpec:
			if x.Name != nil && x.Name.Name == name {
				pos = x.Name.Pos()
				return false
			}
		}
		return true
	})
	return pos
}

func findIdentifierAtLine(node *ast.File, fset *token.FileSet, line int) string {
	var identifier string
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			pos := fset.Position(x.Pos())
			if pos.Line == line {
				identifier = x.Name
				return false
			}
		}
		return true
	})
	return identifier
}

func findReferencesInFile(filePath, targetName string) ([]ReferenceResult, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var results []ReferenceResult
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if x.Name == targetName {
				pos := fset.Position(x.Pos())
				results = append(results, ReferenceResult{
					Path: filePath,
					Line: pos.Line,
					Text: targetName,
				})
			}
		}
		return true
	})

	return results, nil
}

func formatNode(node ast.Node, fset *token.FileSet) string {
	if node == nil {
		return ""
	}
	return ""
}
