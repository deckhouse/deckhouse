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
			assert.Equal(t, res.ExportedFields, res.TotalFields, "File '%s' has struct (%s:%d) with unexported fields.\n", res.FileName, res.Name, res.Line)
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

func collectGoHooks() []string {
	var hookDirs []string
	for _, possibleDir := range []string{
		"/deckhouse/modules/*/hooks",
		"/deckhouse/ee/modules/*/hooks",
		"/deckhouse/ee/fe/modules/*/hooks",
	} {
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

		sc := structCheckResult{
			Name:           structName,
			TotalFields:    structSpec.Fields.NumFields(),
			ExportedFields: 0,
			Line:           fset.Position(structSpec.Pos()).Line,
			FileName:       fset.File(structSpec.Pos()).Name(),
		}

		for _, fields := range structSpec.Fields.List {
			switch f := fields.Type.(type) {
			case *ast.StarExpr:
				switch f.X.(type) {
				case *ast.SelectorExpr:
					if len(fields.Names) == 0 {
						// for embedded fields, like
						// type MyStruct struct { *corev1.Node }
						if inField, ok := f.X.(*ast.SelectorExpr); ok {
							if inField.Sel.IsExported() {
								sc.ExportedFields++
							}
						}
					} else {
						for _, field := range fields.Names {
							if field.IsExported() {
								sc.ExportedFields++
							}
						}
					}

				default:
					for _, field := range fields.Names {
						if field.IsExported() {
							sc.ExportedFields++
						}
					}
				}

			case *ast.SelectorExpr:
				// embedded fields
				if f.Sel.IsExported() {
					sc.ExportedFields++
				}

			default:
				// direct fields
				for _, field := range fields.Names {
					if field.IsExported() {
						sc.ExportedFields++
					}
				}
			}
		}

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
					ident, ok := rhs0.Args[0].(*ast.Ident)
					if !ok {
						return ""
					}

					return ident.Name

				case *ast.IndexExpr:
					// it's some built in types, like getting value := map[string][]byte
					// pass

				default:
					fmt.Println("Unknown type", rhs0)
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
}
