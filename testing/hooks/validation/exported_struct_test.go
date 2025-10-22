//go:build validation
// +build validation

/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gopkg.in/yaml.v2"
)

// Check that structures which are used by FilterFunc don't have unexported fields
func TestValidationHookStructExportedFields(t *testing.T) {
	gohooks := collectGoHooks()
	for _, hookPath := range gohooks {
		fset := token.NewFileSet()

		node, err := parser.ParseFile(fset, hookPath, nil, parser.AllErrors)
		require.NoError(t, err)

		funcReturnStructs, structsDeclaration := inspectNodes(node)
		result := checkStructFields(fset, structsDeclaration, funcReturnStructs)

		for _, res := range result {
			assert.Equal(t, res.TotalFields, res.ExportedFields, "File '%s' has struct (%s:%d) with unexported fields.\n", res.FileName, res.Name, res.Line)
		}
	}
}

type structCheckResult struct {
	Name           string
	TotalFields    int
	ExportedFields int

	FileName string
	Line     int
}

type edition struct {
	Name       string `yaml:"name,omitempty"`
	ModulesDir string `yaml:"modulesDir,omitempty"`
}

type editions struct {
	Editions []edition `yaml:"editions,omitempty"`
}

func getPossiblePathToModules() []string {
	content, err := os.ReadFile("/deckhouse/editions.yaml")
	if err != nil {
		panic(fmt.Sprintf("cannot read editions file: %v", err))
	}

	e := editions{}
	err = yaml.Unmarshal(content, &e)
	if err != nil {
		panic(fmt.Errorf("cannot unmarshal editions file: %v", err))
	}

	modulesDir := make([]string, 0)
	for i, ed := range e.Editions {
		if ed.Name == "" {
			panic(fmt.Sprintf("name for %d index is empty", i))
		}
		modulesDir = append(modulesDir, fmt.Sprintf("/deckhouse/%s/*/hooks", ed.ModulesDir))
	}

	return modulesDir
}

func collectGoHooks() []string {
	var hookDirs []string
	for _, possibleDir := range getPossiblePathToModules() {
		result, err := filepath.Glob(possibleDir)
		if err != nil {

		}

		hookDirs = append(hookDirs, result...)
	}

	hookDirs = append(hookDirs, "/deckhouse/global-hooks")

	gohooks := make([]string, 0)

	for _, dir := range hookDirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			switch {
			case err != nil:
				return err

			case strings.HasSuffix(path, "test.go"): // ignore tests
				return nil

			case strings.HasSuffix(path, ".go"):
				gohooks = append(gohooks, path)

			default:
				return nil
			}

			return nil
		})
	}

	return gohooks
}

// checkStructFields parses structures which are marked as FilterFunc return values and return the check result
// with TotalFields and ExportedFields
func checkStructFields(fset *token.FileSet, structs map[string]*ast.StructType, checkMap map[string]struct{}) []structCheckResult {
	result := make([]structCheckResult, 0)

	for structName, structSpec := range structs {
		if _, ok := checkMap[structName]; !ok {
			continue
		}

		var totalField int

		walker := structWalker(&totalField, false)
		ast.Inspect(structSpec, walker)

		sc := structCheckResult{
			Name:           structName,
			TotalFields:    totalField,
			ExportedFields: 0,
			Line:           fset.Position(structSpec.Pos()).Line,
			FileName:       fset.File(structSpec.Pos()).Name(),
		}

		walker = structWalker(&sc.ExportedFields, true)
		ast.Inspect(structSpec, walker)

		result = append(result, sc)
	}

	return result
}

// this function inspects source .go file, walks through ast and returns
// 1. structs, that are return value by filter functions as go_hook.FilterResult
// 2. struct declarations from source file
func inspectNodes(node ast.Node) (map[string]struct{}, map[string]*ast.StructType) {
	structFromFuncs := make(map[string]struct{})
	structDeclarations := make(map[string]*ast.StructType)

	ast.Inspect(node, func(n ast.Node) bool {
		// Find Type Specs
		typeSpec, ok := n.(*ast.TypeSpec)
		if ok {
			structSpec, ok := typeSpec.Type.(*ast.StructType)
			if ok {
				structDeclarations[typeSpec.Name.Name] = structSpec
				return false
			}
		}

		// Find Functions
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
			structNames := parseFilterFuncDeclaration(fn)
			if len(structNames) == 0 {
				return false
			}
			for _, structName := range structNames {
				structFromFuncs[structName] = struct{}{}
			}
		}

		return true
	})

	return structFromFuncs, structDeclarations
}

// parseFilterFuncDeclaration parses function signature and returns the names of return-value structure
func parseFilterFuncDeclaration(fn *ast.FuncDecl) []string {
	selector, ok := fn.Type.Results.List[0].Type.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	if selector.Sel.Name != "FilterResult" {
		return nil
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok || ident.Name != "go_hook" {
		return nil
	}

	structNames := make(map[string]struct{})
	varAssignments := make(map[string]string)

	// Helper function to recursively walk statements and collect variable assignments.
	var collectAssignments func(stmts []ast.Stmt)
	collectAssignments = func(stmts []ast.Stmt) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *ast.AssignStmt:
				if len(s.Lhs) == 1 && len(s.Rhs) == 1 {
					lhs, ok1 := s.Lhs[0].(*ast.Ident)
					switch rhs := s.Rhs[0].(type) {
					case *ast.CompositeLit:
						if ok1 {
							if ident, ok := rhs.Type.(*ast.Ident); ok {
								varAssignments[lhs.Name] = ident.Name
							}
						}
					case *ast.UnaryExpr:
						// Handle pointer to struct: &privateStruct{...}
						if rhs.Op == token.AND {
							if comp, ok := rhs.X.(*ast.CompositeLit); ok {
								if ident, ok := comp.Type.(*ast.Ident); ok && ok1 {
									varAssignments[lhs.Name] = ident.Name
								}
							}
						}
					}
				}
			case *ast.BlockStmt:
				collectAssignments(s.List)
			case *ast.IfStmt:
				collectAssignments([]ast.Stmt{s.Body})
				if s.Else != nil {
					switch elseStmt := s.Else.(type) {
					case *ast.BlockStmt:
						collectAssignments(elseStmt.List)
					case *ast.IfStmt:
						collectAssignments([]ast.Stmt{elseStmt})
					}
				}
			case *ast.ForStmt:
				collectAssignments(s.Body.List)
			case *ast.RangeStmt:
				collectAssignments(s.Body.List)
			case *ast.SwitchStmt:
				for _, stmt := range s.Body.List {
					if caseClause, ok := stmt.(*ast.CaseClause); ok {
						collectAssignments(caseClause.Body)
					}
				}
			case *ast.TypeSwitchStmt:
				for _, stmt := range s.Body.List {
					if caseClause, ok := stmt.(*ast.CaseClause); ok {
						collectAssignments(caseClause.Body)
					}
				}
			case *ast.DeclStmt, *ast.ExprStmt, *ast.BranchStmt, *ast.LabeledStmt, *ast.ReturnStmt:
				// These statement types are not relevant for further AST traversal in this context:
				// - DeclStmt: Variable or constant declarations, already handled or not needed for struct return analysis.
				// - ExprStmt: Standalone expressions (e.g., function calls), do not affect struct return detection.
				// - BranchStmt: Control flow statements (break, continue, goto, fallthrough), do not impact struct analysis.
				// - LabeledStmt: Labels for branching, not relevant for struct or return value analysis.
				// - ReturnStmt: Return statements are processed separately; no need to traverse further here.
				// No additional processing is required for these statement types.
			default:
				fmt.Printf("Unhandled statement type: %T\n", s)
			}
		}
	}
	collectAssignments(fn.Body.List)

	// Helper function to recursively walk statements and collect struct names from return statements.
	var walkStmts func(stmts []ast.Stmt)
	walkStmts = func(stmts []ast.Stmt) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *ast.ReturnStmt:
				if len(s.Results) == 0 {
					continue
				}
				switch lit := s.Results[0].(type) {
				case *ast.CompositeLit:
					if ident, ok := lit.Type.(*ast.Ident); ok {
						structNames[ident.Name] = struct{}{}
					}
				case *ast.UnaryExpr:
					// Handle return &privateStruct{...}
					if lit.Op == token.AND {
						if comp, ok := lit.X.(*ast.CompositeLit); ok {
							if ident, ok := comp.Type.(*ast.Ident); ok {
								structNames[ident.Name] = struct{}{}
							}
						}
					}
				case *ast.Ident:
					if structType, ok := varAssignments[lit.Name]; ok {
						structNames[structType] = struct{}{}
					}
				}
			case *ast.BlockStmt:
				walkStmts(s.List)
			case *ast.IfStmt:
				walkStmts([]ast.Stmt{s.Body})
				if s.Else != nil {
					switch elseStmt := s.Else.(type) {
					case *ast.BlockStmt:
						walkStmts(elseStmt.List)
					case *ast.IfStmt:
						walkStmts([]ast.Stmt{elseStmt})
					}
				}
			case *ast.ForStmt:
				walkStmts(s.Body.List)
			case *ast.RangeStmt:
				walkStmts(s.Body.List)
			case *ast.SwitchStmt:
				for _, stmt := range s.Body.List {
					if caseClause, ok := stmt.(*ast.CaseClause); ok {
						walkStmts(caseClause.Body)
					}
				}
			case *ast.TypeSwitchStmt:
				for _, stmt := range s.Body.List {
					if caseClause, ok := stmt.(*ast.CaseClause); ok {
						walkStmts(caseClause.Body)
					}
				}
			case *ast.AssignStmt, *ast.DeclStmt, *ast.ExprStmt, *ast.BranchStmt, *ast.LabeledStmt:
				// These statement types do not affect the analysis of return values or struct usage.
				// - AssignStmt: Variable assignments, already handled.
				// - DeclStmt: Declarations (e.g., var, const), not relevant for return analysis.
				// - ExprStmt: Standalone expressions (e.g., function calls), not relevant here.
				// - BranchStmt: Control flow statements (break, continue, goto, fallthrough), do not impact struct returns.
				// - LabeledStmt: Labeled statements for goto/branching, not relevant for struct analysis.
				// TODO: Add more cases if you want to support select, etc.
			default:
				fmt.Printf("Unhandled statement type: %T\n", s)
			}
		}
	}

	walkStmts(fn.Body.List)

	var result []string
	for name := range structNames {
		result = append(result, name)
	}
	return result
}

// Test validator logic
func TestExportStructValidatorLogic(t *testing.T) {
	t.Run("pointer assignment and return", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
  "github.com/flant/addon-operator/pkg/module_manager/go_hook"
  "time"
)

type fooBar struct {
  str string
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
  asVar := &fooBar{str: "a"}
  if obj == nil {
    return asVar, nil
  }
  return nil, nil
}`
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		funcResults, structs := inspectNodes(node)

		result := checkStructFields(fset, structs, funcResults)

		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].TotalFields)
		require.Equal(t, 0, result[0].ExportedFields)
	})

	t.Run("direct pointer return", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
  "github.com/flant/addon-operator/pkg/module_manager/go_hook"
  "time"
)

type fooBar struct {
  str string
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
  if obj == nil {
    return &fooBar{str: "a"}, nil
  }
  return nil, nil
}`
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		funcResults, structs := inspectNodes(node)

		result := checkStructFields(fset, structs, funcResults)

		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].TotalFields)
		require.Equal(t, 0, result[0].ExportedFields)
	})

	t.Run("pointer assignment, switch statement", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
  "github.com/flant/addon-operator/pkg/module_manager/go_hook"
  "time"
)

type fooBar struct {
  str string
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
  asVar := &fooBar{str: "a"}
  switch obj.Object["field"] {
  case nil:
    return asVar, nil
  default:
    return nil, nil
  }
}`
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		funcResults, structs := inspectNodes(node)

		result := checkStructFields(fset, structs, funcResults)

		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].TotalFields)
		require.Equal(t, 0, result[0].ExportedFields)
	})
	t.Run("var assignment", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"time"
)

type fooBar struct {
  str string
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	asVar := fooBar{str: "a"}
	if obj == nil {
		return asVar, nil
	}
	return nil, nil
}`
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		funcResults, structs := inspectNodes(node)

		result := checkStructFields(fset, structs, funcResults)

		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].TotalFields)
		require.Equal(t, 0, result[0].ExportedFields)
	})
	t.Run("early return", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"time"
)

type fooBar struct {
  str string
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if obj == nil {
		return fooBar{str: "a"}, nil
	}
	return nil, nil
}`
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		funcResults, structs := inspectNodes(node)

		result := checkStructFields(fset, structs, funcResults)

		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].TotalFields)
		require.Equal(t, 0, result[0].ExportedFields)
	})
	t.Run("test full unexported", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"time"
)

type fooBar struct {
  str string
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return fooBar{str: "a"}, nil
}`

		fset := token.NewFileSet()

		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		funcReturnStructs, structsDeclaration := inspectNodes(node)

		result := checkStructFields(fset, structsDeclaration, funcReturnStructs)

		require.Len(t, result, 1)
		require.Equal(t, 0, result[0].ExportedFields)
		require.Equal(t, 1, result[0].TotalFields)
	})

	t.Run("test partially unexported", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"time"
)

type fooBar struct {
  Num int
  str string
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return fooBar{str: "a"}, nil
}`

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		funcResults, structs := inspectNodes(node)

		result := checkStructFields(fset, structs, funcResults)

		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].ExportedFields)
		require.Equal(t, 2, result[0].TotalFields)
	})

	t.Run("test embeded struct", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"time"
)

type barBaz struct {
  Name string
  X string
}

type fooBar struct {
  barBaz
  XXX *barBaz
  ZXC barBaz
  Num int
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
    bz := barBaz{Name: "qqqq", X: "asd"}
	return fooBar{barBaz: barBaz{Name: "lalala", X: "foor"}, XXX: &bz, ZXC: bz, Num: 3}, nil
}`

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		funcResults, structs := inspectNodes(node)

		result := checkStructFields(fset, structs, funcResults)

		require.Len(t, result, 1)
		require.Equal(t, 5, result[0].TotalFields)
		require.Equal(t, 5, result[0].ExportedFields)
	})

	t.Run("test named fields", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"time"
)

type barBaz struct {
  Name string
  X string
}

type fooBar struct {
  XXX *barBaz
  ZXC barBaz
  Num map[string]interface{}
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
    bz := barBaz{Name: "qqqq", X: "str"}
	return fooBar{XXX: &bz, ZXC: bz, Num: map[string]interface{}{"a":"b"}}, nil
}`

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		funcResults, structs := inspectNodes(node)

		result := checkStructFields(fset, structs, funcResults)

		require.Len(t, result, 1)
		require.Equal(t, 3, result[0].TotalFields)
		require.Equal(t, 3, result[0].ExportedFields)
	})

	t.Run("test named fields with private struct", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"time"
)

type barBaz struct {
  name string
  x string
}

type fooBar struct {
  XXX *barBaz
  ZXC barBaz
  Num map[string]interface{}
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
    bz := barBaz{name: "qqqq", x: "str"}
	return fooBar{XXX: &bz, ZXC: bz, Num: map[string]interface{}{"a":"b"}}, nil
}`

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		funcResults, structs := inspectNodes(node)

		result := checkStructFields(fset, structs, funcResults)

		require.Len(t, result, 1)
		require.Equal(t, 3, result[0].TotalFields)
		require.Equal(t, 3, result[0].ExportedFields)
	})
}

func structWalker(fieldCounter *int, onlyExported bool) func(n ast.Node) bool {
	return func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.Field:
			if len(t.Names) == 0 {
				// embeded types
				switch tt := t.Type.(type) {
				case *ast.Ident:
					if tt.Obj != nil {
						switch ttt := tt.Obj.Decl.(type) {
						case *ast.TypeSpec:
							switch tttt := ttt.Type.(type) {
							case *ast.StructType:
								for _, field := range tttt.Fields.List {
									for _, name := range field.Names {
										if onlyExported {
											if name.IsExported() {
												*fieldCounter++
											}
										} else {
											*fieldCounter++
										}
									}
								}
								return false
							}
						}
					}
				}
			} else {
				for _, name := range t.Names {
					if onlyExported {
						if name.IsExported() {
							*fieldCounter++
						}
					} else {
						*fieldCounter++
					}
				}
				return false
			}
		}
		return true
	}
}
