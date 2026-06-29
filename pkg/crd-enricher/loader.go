// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crdenricher

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// packageInfo holds everything the enricher needs to know about a single Go
// API package: the markers found on its types and fields, and the lookup table
// of CRD root types it declares.
type packageInfo struct {
	// path is the import path of the package, used to map a *types.Named back
	// to its packageInfo while walking field types across packages.
	path string
	// version is the Kubernetes API version the package implements, derived
	// from the package name (for example "v1alpha1").
	version string

	// typeMarkers maps a Go type name to the markers declared on the type.
	typeMarkers map[string][]marker
	// fieldMarkers maps a Go type name and a Go field name to the markers
	// declared on that field.
	fieldMarkers map[string]map[string][]marker

	// roots maps a CRD kind (the Go root type name) to its named type.
	roots map[string]*types.Named
}

// loadPackages loads the Go packages matched by the given patterns and returns
// a packageInfo for every one of them, indexed by import path.
func loadPackages(dir string, patterns ...string) (map[string]*packageInfo, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax,
		Dir: dir,
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("packages contain load errors")
	}

	infos := make(map[string]*packageInfo, len(pkgs))
	for _, pkg := range pkgs {
		if pkg.Types == nil {
			continue
		}
		infos[pkg.PkgPath] = newPackageInfo(pkg)
	}

	return infos, nil
}

// newPackageInfo scans the syntax trees of a package and collects all markers
// together with the CRD root types it declares.
func newPackageInfo(pkg *packages.Package) *packageInfo {
	info := &packageInfo{
		path:         pkg.PkgPath,
		version:      pkg.Name,
		typeMarkers:  make(map[string][]marker),
		fieldMarkers: make(map[string]map[string][]marker),
		roots:        make(map[string]*types.Named),
	}

	for _, file := range pkg.Syntax {
		// Track the end of the previous declaration so that the comment
		// groups belonging to the next one can be gathered from the gap in
		// between. This mirrors how controller-gen associates markers: every
		// comment above a type declaration counts, even when blank lines split
		// it into several groups (as is the case for the kubebuilder markers
		// placed above a "// Foo is ..." doc comment).
		prevEnd := file.Name.End()
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				prevEnd = decl.End()
				continue
			}
			// The gap comments belong to the declaration only when it holds a
			// single type; inside a "type ( ... )" block each spec carries its
			// own Doc instead.
			var doc []*ast.CommentGroup
			if len(genDecl.Specs) == 1 {
				doc = commentsInRange(file, prevEnd, genDecl.Pos())
			}
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					info.collectType(pkg, typeSpec, doc)
				}
			}
			prevEnd = genDecl.End()
		}
	}

	return info
}

// commentsInRange returns the comment groups of a file that lie strictly
// between two positions.
func commentsInRange(file *ast.File, from, to token.Pos) []*ast.CommentGroup {
	var groups []*ast.CommentGroup
	for _, group := range file.Comments {
		if group.Pos() > from && group.End() <= to {
			groups = append(groups, group)
		}
	}
	return groups
}

// collectType records the markers of a single type declaration and, when the
// type is a CRD root, registers it in the roots table. doc holds the comment
// groups gathered from the gap above the type declaration.
func (info *packageInfo) collectType(pkg *packages.Package, typeSpec *ast.TypeSpec, doc []*ast.CommentGroup) {
	name := typeSpec.Name.Name

	// For a single-type declaration doc already covers every comment group in
	// the gap (including the one go/ast would expose as Doc); for multi-type
	// blocks doc is nil and the per-spec Doc is the only source.
	groups := doc
	if groups == nil && typeSpec.Doc != nil {
		groups = []*ast.CommentGroup{typeSpec.Doc}
	}

	markers := parseCommentGroups(groups...)
	if len(markers) > 0 {
		info.typeMarkers[name] = markers
	}

	if structType, ok := typeSpec.Type.(*ast.StructType); ok {
		info.collectFields(name, structType)
	}

	if hasMarker(markers, rootMarker) {
		if named := lookupNamed(pkg.Types, name); named != nil {
			info.roots[name] = named
		}
	}
}

// collectFields records the markers attached to every field of a struct,
// keyed by the Go field name (or the embedded type name for anonymous fields).
func (info *packageInfo) collectFields(typeName string, structType *ast.StructType) {
	for _, field := range structType.Fields.List {
		markers := parseCommentGroups(field.Doc, field.Comment)
		if len(markers) == 0 {
			continue
		}

		names := fieldNames(field)
		for _, name := range names {
			if info.fieldMarkers[typeName] == nil {
				info.fieldMarkers[typeName] = make(map[string][]marker)
			}
			info.fieldMarkers[typeName][name] = append(info.fieldMarkers[typeName][name], markers...)
		}
	}
}

// fieldNames returns the Go names a struct field is addressed by. Named fields
// return their identifiers; anonymous (embedded) fields return the embedded
// type name, which matches the name reported by go/types.
func fieldNames(field *ast.Field) []string {
	if len(field.Names) > 0 {
		names := make([]string, 0, len(field.Names))
		for _, ident := range field.Names {
			names = append(names, ident.Name)
		}
		return names
	}

	if name := embeddedName(field.Type); name != "" {
		return []string{name}
	}

	return nil
}

// embeddedName extracts the type name of an embedded field expression, peeling
// off pointers and package qualifiers.
func embeddedName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.StarExpr:
		return embeddedName(t.X)
	default:
		return ""
	}
}

// lookupNamed resolves a named type by name within a package scope.
func lookupNamed(pkg *types.Package, name string) *types.Named {
	obj := pkg.Scope().Lookup(name)
	if obj == nil {
		return nil
	}
	typeName, ok := obj.(*types.TypeName)
	if !ok {
		return nil
	}
	named, ok := typeName.Type().(*types.Named)
	if !ok {
		return nil
	}
	return named
}
