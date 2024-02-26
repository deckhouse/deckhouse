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
	"reflect"
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
			structName := parseFilterFuncDeclaration(fn)
			if structName == "" {
				return false
			}
			structFromFuncs[structName] = struct{}{}
		}

		return true
	})

	return structFromFuncs, structDeclarations
}

// parseFilterFuncDeclaration parses function signature and returns the name of return-value structure
func parseFilterFuncDeclaration(fn *ast.FuncDecl) string {
	selector, ok := fn.Type.Results.List[0].Type.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	if selector.Sel.Name != "FilterResult" {
		// not our function
		return ""
	}

	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return ""
	}

	// possible bug if packet imported with alias name
	if ident.Name != "go_hook" {
		return ""
	}

	for _, l := range fn.Body.List {
		ret, ok := l.(*ast.ReturnStmt)
		if !ok {
			continue
		}

		// parse only return statement
		firstStatement := ret.Results[0]
		switch lit := firstStatement.(type) {
		case *ast.CompositeLit:
			ident, ok := lit.Type.(*ast.Ident)
			if !ok {
				return ""
			}
			return ident.Name

		case *ast.Ident:
			if lit.Obj == nil {
				return ""
			}
			assign, ok := lit.Obj.Decl.(*ast.AssignStmt)
			if !ok {
				return ""
			}
			if len(assign.Rhs) > 0 {
				switch rhs0 := assign.Rhs[0].(type) {
				// pointer
				case *ast.UnaryExpr:
					comp, ok := rhs0.X.(*ast.CompositeLit)
					if !ok {
						return ""
					}
					ident, ok := comp.Type.(*ast.Ident)
					if !ok {
						return ""
					}
					return ident.Name

					// struct
				case *ast.CompositeLit:
					ident, ok := rhs0.Type.(*ast.Ident)
					if !ok {
						return ""
					}
					return ident.Name

					// called with new(Struct)
				case *ast.CallExpr:
					if len(rhs0.Args) == 0 {
						return ""
					}
					ident, ok := rhs0.Args[0].(*ast.Ident)
					if !ok {
						return ""
					}

					return ident.Name

				case *ast.IndexExpr:
				// it's some built in types, like getting value := map[string][]byte
				// pass

				case *ast.BinaryExpr:
				// it's a boolean type: true/false
				// pass

				default:
					fmt.Println("Unknown type", reflect.TypeOf(assign.Rhs[0]))
				}
			}

		default:
			return ""
		}
	}

	return ""
}

// Test validator logic
func TestExportStructValidatorLogic(t *testing.T) {
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
