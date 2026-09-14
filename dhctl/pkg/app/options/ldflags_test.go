/*
Copyright 2026 Flant JSC

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

package options

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	dhctlModulePath = "github.com/deckhouse/deckhouse/dhctl/"
	dhctlModuleRoot = "../../.."
)

var ldflagRegexp = regexp.MustCompile(`-X\s+(\S+)\.(\w+)=`)

// TestBuildVariablesPointAtRealSymbols guards the whole -X ldflag block in
// dhctl/Makefile: the linker silently ignores a -X for a symbol that does not
// exist, so a package rename turns build metadata into its zero default with
// no build error anywhere.
func TestBuildVariablesPointAtRealSymbols(t *testing.T) {
	makefile, err := os.ReadFile(filepath.Join(dhctlModuleRoot, "Makefile"))
	if err != nil {
		t.Fatalf("read dhctl Makefile: %v", err)
	}

	block := buildVariablesBlock(t, string(makefile))
	matches := ldflagRegexp.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		t.Fatal("no -X ldflags found in BUILD_VARIABLES")
	}

	for _, m := range matches {
		importPath, symbol := m[1], m[2]
		if !strings.HasPrefix(importPath, dhctlModulePath) {
			t.Errorf("-X %s.%s: import path is outside the dhctl module", importPath, symbol)
			continue
		}

		dir := filepath.Join(dhctlModuleRoot, strings.TrimPrefix(importPath, dhctlModulePath))
		if !packageHasVar(t, dir, symbol) {
			t.Errorf("-X %s.%s: no package-level var %q in %s, the linker will ignore this flag", importPath, symbol, symbol, dir)
		}
	}
}

func buildVariablesBlock(t *testing.T, makefile string) string {
	t.Helper()

	_, rest, found := strings.Cut(makefile, "define BUILD_VARIABLES")
	if !found {
		t.Fatal("BUILD_VARIABLES is not defined in dhctl/Makefile")
	}

	block, _, found := strings.Cut(rest, "endef")
	if !found {
		t.Fatal("BUILD_VARIABLES is not terminated by endef")
	}

	return block
}

func packageHasVar(t *testing.T, dir, name string) bool {
	t.Helper()

	pkgs, err := parser.ParseDir(token.NewFileSet(), dir, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.VAR {
					continue
				}
				if declaresVar(genDecl, name) {
					return true
				}
			}
		}
	}

	return false
}

func declaresVar(decl *ast.GenDecl, name string) bool {
	for _, spec := range decl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, ident := range valueSpec.Names {
			if ident.Name == name {
				return true
			}
		}
	}

	return false
}
