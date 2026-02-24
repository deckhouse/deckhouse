//go:build validation
// +build validation

/*
Copyright 2025 Flant JSC

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
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Linter: ensure filter functions (returning go_hook.FilterResult) return only custom, local structs
// and these structs do not contain any types from other packages (no embedded or field types from pkg.Type).

// Validate that Snapshots are unmarshaled only into custom local structs.
// We check usages of:
//   - UnmarshalToStruct[T](...)
//   - SnapshotIter[T](...)
//   - <snapshot>.UnmarshalTo(&T{...})
func TestNoImportedStructInFilterResult(t *testing.T) {
	gohooks := collectGoHooks()
	var allErrors []string
	for _, hookPath := range gohooks {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, hookPath, nil, parser.AllErrors)
		require.NoError(t, err)

		// unmarshalling targets must be local/custom
		errs := validateNoExternalUnmarshalTargets(fset, node)
		if len(errs) > 0 {
			allErrors = append(allErrors, errs...)
		}

		// filter functions must return local struct without external types inside
		ferrs := validateFilterReturnsNoExternalTypes(fset, node)
		if len(ferrs) > 0 {
			allErrors = append(allErrors, ferrs...)
		}
	}
	if len(allErrors) > 0 {
		// Print full list of all errors across files
		t.Fatalf(strings.Join(allErrors, "\n"))
	}
}

// Examples for new rule: ensure unmarshal targets are local custom structs
func TestNoExternalUnmarshal_Examples(t *testing.T) {
	t.Run("incorrect: UnmarshalToStruct external", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
  v1 "k8s.io/api/core/v1"
  sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

func handler(snaps pkg.Snapshots) error {
  _, _ = sdkobjectpatch.UnmarshalToStruct[v1.Secret](snaps, "s")
  return nil
}`
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		errs := validateNoExternalUnmarshalTargets(fset, node)
		require.NotEmpty(t, errs)
	})

	t.Run("incorrect: SnapshotIter external", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
  v1 "k8s.io/api/core/v1"
  sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

func handler(snaps pkg.Snapshots) error {
  for secret, err := range sdkobjectpatch.SnapshotIter[v1.Secret](snaps.Get("s")) {
    _ = secret
    _ = err
  }
  return nil
}`
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		errs := validateNoExternalUnmarshalTargets(fset, node)
		require.NotEmpty(t, errs)
	})

	t.Run("incorrect: Snapshot.UnmarshalTo external", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
  v1 "k8s.io/api/core/v1"
)

func handler(snap Snapshot) error {
  var _ = snap.UnmarshalTo(&v1.Secret{})
  return nil
}`
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		errs := validateNoExternalUnmarshalTargets(fset, node)
		require.NotEmpty(t, errs)
	})

	t.Run("correct: local struct", func(t *testing.T) {
		t.Parallel()
		src := `package foo

import (
  sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type localInfo struct { Name string }

func handler(snaps pkg.Snapshots, snap Snapshot) error {
  _, _ = sdkobjectpatch.UnmarshalToStruct[localInfo](snaps, "s")
  for item, err := range sdkobjectpatch.SnapshotIter[localInfo](snaps.Get("s")) {
    _ = item
    _ = err
  }
  _ = snap.UnmarshalTo(&localInfo{})
  return nil
}`
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", src, parser.AllErrors)
		require.NoError(t, err)

		errs := validateNoExternalUnmarshalTargets(fset, node)
		require.Empty(t, errs)
	})
}

// Core validation
func validateNoExternalUnmarshalTargets(fset *token.FileSet, node *ast.File) []string {
	// Collect struct declarations from this file to detect local types quickly.
	localTypes := map[string]struct{}{}
	ast.Inspect(node, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if _, ok := ts.Type.(*ast.StructType); ok {
			localTypes[ts.Name.Name] = struct{}{}
		}
		return false
	})

	var errors []string

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			// Also check for old-style type assertions on snapshots: input.Snapshots["key"][i].(Type)
			if ta, ok := n.(*ast.TypeAssertExpr); ok {
				if containsSnapshotAccess(ta.X) {
					if isExternalTypeExpr(ta.Type, localTypes) {
						pos := fset.Position(ta.Lparen)
						errors = append(errors, fmt.Sprintf("%s:%d:%d: snapshot type assertion to external type %s is prohibited", normalizePath(pos.Filename), pos.Line, pos.Column, renderTypeExpr(ta.Type)))
					} else if isForbiddenTypeExpr(ta.Type) {
						pos := fset.Position(ta.Lparen)
						errors = append(errors, fmt.Sprintf("%s:%d:%d: snapshot type assertion to forbidden type %s is prohibited", normalizePath(pos.Filename), pos.Line, pos.Column, renderTypeExpr(ta.Type)))
					}
				}
			}
			return true
		}

		// Handle generics: UnmarshalToStruct[T], SnapshotIter[T]
		switch fun := call.Fun.(type) {
		case *ast.IndexExpr:
			if name := baseFuncName(fun.X); name == "UnmarshalToStruct" || name == "SnapshotIter" {
				texpr := fun.Index
				if isExternalTypeExpr(texpr, localTypes) {
					pos := fset.Position(call.Lparen)
					errors = append(errors, fmt.Sprintf("%s:%d:%d: unmarshal to external type %s is prohibited", normalizePath(pos.Filename), pos.Line, pos.Column, renderTypeExpr(texpr)))
				} else if isForbiddenTypeExpr(texpr) {
					pos := fset.Position(call.Lparen)
					errors = append(errors, fmt.Sprintf("%s:%d:%d: unmarshal to forbidden type %s is prohibited", normalizePath(pos.Filename), pos.Line, pos.Column, renderTypeExpr(texpr)))
				}
			}
		case *ast.IndexListExpr:
			if name := baseFuncName(fun.X); name == "UnmarshalToStruct" || name == "SnapshotIter" {
				if len(fun.Indices) > 0 {
					texpr := fun.Indices[0]
					if isExternalTypeExpr(texpr, localTypes) {
						pos := fset.Position(call.Lparen)
						errors = append(errors, fmt.Sprintf("%s:%d:%d: unmarshal to external type %s is prohibited", normalizePath(pos.Filename), pos.Line, pos.Column, renderTypeExpr(texpr)))
					} else if isForbiddenTypeExpr(texpr) {
						pos := fset.Position(call.Lparen)
						errors = append(errors, fmt.Sprintf("%s:%d:%d: unmarshal to forbidden type %s is prohibited", normalizePath(pos.Filename), pos.Line, pos.Column, renderTypeExpr(texpr)))
					}
				}
			}
		case *ast.SelectorExpr:
			// Method call like <snap>.UnmarshalTo(&T{})
			if fun.Sel != nil && fun.Sel.Name == "UnmarshalTo" {
				if len(call.Args) > 0 {
					if ue, ok := call.Args[0].(*ast.UnaryExpr); ok && ue.Op == token.AND {
						switch x := ue.X.(type) {
						case *ast.CompositeLit:
							texpr := x.Type
							if isExternalTypeExpr(texpr, localTypes) {
								pos := fset.Position(call.Lparen)
								errors = append(errors, fmt.Sprintf("%s:%d:%d: unmarshal to external type %s is prohibited", normalizePath(pos.Filename), pos.Line, pos.Column, renderTypeExpr(texpr)))
							} else if isForbiddenTypeExpr(texpr) {
								pos := fset.Position(call.Lparen)
								errors = append(errors, fmt.Sprintf("%s:%d:%d: unmarshal to forbidden type %s is prohibited", normalizePath(pos.Filename), pos.Line, pos.Column, renderTypeExpr(texpr)))
							}
						}
					}
				}
			}
		}

		return true
	})

	return errors
}

// Helpers for new rule
func baseFuncName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	default:
		return ""
	}
}

// collectAssignedCompositeTypes collects variable -> composite type expression across arbitrary statements
// isExternalTypeExpr returns true if texpr clearly references a type from another package (pkg.Type).
// Ident types are considered local (may be from same package), selectors are external.
func isExternalTypeExpr(texpr ast.Expr, localTypes map[string]struct{}) bool {
	switch tt := texpr.(type) {
	case *ast.SelectorExpr:
		return true
	case *ast.Ident:
		// If declared in this file, treat as local; otherwise still treat as local to reduce false positives.
		_, ok := localTypes[tt.Name]
		return false || ok && false // always false; keep logic explicit
	case *ast.StarExpr:
		return isExternalTypeExpr(tt.X, localTypes)
	case *ast.ArrayType:
		return isExternalTypeExpr(tt.Elt, localTypes)
	case *ast.MapType:
		return isExternalTypeExpr(tt.Key, localTypes) || isExternalTypeExpr(tt.Value, localTypes)
	default:
		return false
	}
}

// isForbiddenTypeExpr returns true for interface{}, any, []interface{}, []any, map[any]any, map[string]any, map[string]interface{}, etc.
func isForbiddenTypeExpr(texpr ast.Expr) bool {
	switch t := texpr.(type) {
	case *ast.Ident:
		// any
		return t.Name == "any"
	case *ast.InterfaceType:
		// interface{}
		return true
	case *ast.StarExpr:
		return isForbiddenTypeExpr(t.X)
	case *ast.ArrayType:
		return isForbiddenTypeExpr(t.Elt)
	case *ast.MapType:
		return isForbiddenTypeExpr(t.Key) || isForbiddenTypeExpr(t.Value)
	default:
		return false
	}
}

// walkReturns executes cb for each return expression (first result) within stmts recursively
// renderTypeExpr converts a type expression into a human-friendly string
func renderTypeExpr(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.SelectorExpr:
		return renderSelector(t)
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + renderTypeExpr(t.X)
	case *ast.ArrayType:
		return "[]" + renderTypeExpr(t.Elt)
	case *ast.MapType:
		return "map[" + renderTypeExpr(t.Key) + "]" + renderTypeExpr(t.Value)
	default:
		return "<type>"
	}
}

func renderSelector(se *ast.SelectorExpr) string {
	pkgIdent, _ := se.X.(*ast.Ident)
	if pkgIdent == nil {
		return se.Sel.Name
	}
	return pkgIdent.Name + "." + se.Sel.Name
}

// normalizePath replaces absolute local path prefix to '/deckhouse' to match requested output style.
func normalizePath(p string) string {
	if p == "" {
		return p
	}
	return strings.TrimPrefix(filepath.Clean(p), "/deckhouse/")
}

// containsSnapshotAccess detects patterns like input.Snapshots["key"][i]
func containsSnapshotAccess(expr ast.Expr) bool {
	// Walk up the expression chain: IndexExpr/IndexListExpr over a SelectorExpr with Sel "Snapshots"
	for {
		switch e := expr.(type) {
		case *ast.IndexExpr:
			expr = e.X
			continue
		case *ast.IndexListExpr:
			expr = e.X
			continue
		case *ast.SelectorExpr:
			if e.Sel != nil && e.Sel.Name == "Snapshots" {
				return true
			}
			// keep going up (could be input.Snapshots)
			expr = e.X
			continue
		default:
			return false
		}
	}
}

// Old rule logic below (adapted):
// - find functions returning go_hook.FilterResult
// - ensure return value is a local struct, and that struct doesn't contain external package types
func validateFilterReturnsNoExternalTypes(fset *token.FileSet, node *ast.File) []string {
	// Collect struct declarations in the file
	structDecls := map[string]*ast.StructType{}
	ast.Inspect(node, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if st, ok := ts.Type.(*ast.StructType); ok {
			structDecls[ts.Name.Name] = st
		}
		return false
	})

	var errors []string

	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Type == nil || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
			return true
		}
		if !isFilterResultFunc(fn) {
			return true
		}

		// Gather local returned struct names
		localReturned := map[string]struct{}{}

		walkReturns(fn.Body.List, func(ret ast.Expr) {
			switch expr := ret.(type) {
			case *ast.CompositeLit:
				if id, ok := expr.Type.(*ast.Ident); ok {
					localReturned[id.Name] = struct{}{}
				} else if se, ok := expr.Type.(*ast.SelectorExpr); ok {
					pos := fset.Position(expr.Pos())
					errors = append(errors, fmt.Sprintf("%s:%d:%d: return uses external type %s", normalizePath(pos.Filename), pos.Line, pos.Column, renderSelector(se)))
				}
			case *ast.UnaryExpr:
				if expr.Op == token.AND {
					if cl, ok := expr.X.(*ast.CompositeLit); ok {
						if id, ok := cl.Type.(*ast.Ident); ok {
							localReturned[id.Name] = struct{}{}
						} else if se, ok := cl.Type.(*ast.SelectorExpr); ok {
							pos := fset.Position(expr.Pos())
							errors = append(errors, fmt.Sprintf("%s:%d:%d: return uses external type %s", normalizePath(pos.Filename), pos.Line, pos.Column, renderSelector(se)))
						}
					}
				}
			}
		})

		// validate local structs recursively to ensure no external fields/embeds
		visited := map[string]bool{}
		for name := range localReturned {
			if st, ok := structDecls[name]; ok {
				checkStructForExternalFields(fset, name, st, structDecls, &visited, &errors)
			}
		}

		return true
	})

	return errors
}

func isFilterResultFunc(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return false
	}
	sel, ok := fn.Type.Results.List[0].Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name != "FilterResult" {
		return false
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "go_hook" {
		return false
	}
	return true
}

func walkReturns(stmts []ast.Stmt, cb func(ast.Expr)) {
	for _, s := range stmts {
		switch st := s.(type) {
		case *ast.ReturnStmt:
			if len(st.Results) > 0 {
				cb(st.Results[0])
			}
		case *ast.BlockStmt:
			walkReturns(st.List, cb)
		case *ast.IfStmt:
			walkReturns([]ast.Stmt{st.Body}, cb)
			if st.Else != nil {
				switch e := st.Else.(type) {
				case *ast.BlockStmt:
					walkReturns(e.List, cb)
				case *ast.IfStmt:
					walkReturns([]ast.Stmt{e}, cb)
				}
			}
		case *ast.ForStmt:
			walkReturns(st.Body.List, cb)
		case *ast.RangeStmt:
			walkReturns(st.Body.List, cb)
		case *ast.SwitchStmt:
			for _, cc := range st.Body.List {
				if c, ok := cc.(*ast.CaseClause); ok {
					walkReturns(c.Body, cb)
				}
			}
		case *ast.TypeSwitchStmt:
			for _, cc := range st.Body.List {
				if c, ok := cc.(*ast.CaseClause); ok {
					walkReturns(c.Body, cb)
				}
			}
		}
	}
}

func checkStructForExternalFields(fset *token.FileSet, name string, st *ast.StructType, decls map[string]*ast.StructType, visited *map[string]bool, errors *[]string) {
	if (*visited)[name] {
		return
	}
	(*visited)[name] = true

	for _, field := range st.Fields.List {
		// Embedded field
		if len(field.Names) == 0 {
			if isExternalTypeExpr(field.Type, map[string]struct{}{}) {
				pos := fset.Position(field.Pos())
				*errors = append(*errors, fmt.Sprintf("%s:%d:%d: struct '%s' has embedded external type", normalizePath(pos.Filename), pos.Line, pos.Column, name))
				continue
			}
			// Embedded local type: recurse
			if id, ok := field.Type.(*ast.Ident); ok {
				if nxt, ok := decls[id.Name]; ok {
					checkStructForExternalFields(fset, id.Name, nxt, decls, visited, errors)
				}
			}
			continue
		}

		// Named field(s)
		if isExternalTypeExpr(field.Type, map[string]struct{}{}) {
			fieldName := field.Names[0].Name
			pos := fset.Position(field.Pos())
			*errors = append(*errors, fmt.Sprintf("%s:%d:%d: struct '%s' field '%s' uses external type", normalizePath(pos.Filename), pos.Line, pos.Column, name, fieldName))
			continue
		}

		// Recurse for local named struct types
		if id, ok := field.Type.(*ast.Ident); ok {
			if nxt, ok := decls[id.Name]; ok {
				checkStructForExternalFields(fset, id.Name, nxt, decls, visited, errors)
			}
		}
	}
}
