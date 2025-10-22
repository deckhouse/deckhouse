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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNoEmptyCrontabInGoHooks checks that no go_hook.HookConfig has a ScheduleConfig with an empty Crontab.
func TestNoEmptyCrontabInGoHooks(t *testing.T) {
	gohooks := collectGoHooks()

	for _, hookPath := range gohooks {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, hookPath, nil, parser.AllErrors)
		require.NoError(t, err)

		errs := parseCrontabSchedule(node, hookPath, fset)
		for _, err := range errs {
			t.Error(err)
		}
	}
}

func parseCrontabSchedule(node ast.Node, hookPath string, fset *token.FileSet) []error {
	errors := []error{}
	ast.Inspect(node, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		// Look for go_hook.HookConfig struct literals
		if se, ok := cl.Type.(*ast.SelectorExpr); ok && se.Sel.Name == "HookConfig" {
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == "Schedule" {
					// kv.Value should be a CompositeLit (slice of ScheduleConfig)
					if list, ok := kv.Value.(*ast.CompositeLit); ok {
						for _, entry := range list.Elts {
							if entryCl, ok := entry.(*ast.CompositeLit); ok {
								hasCrontab := false
								for _, entryElt := range entryCl.Elts {
									if entryKv, ok := entryElt.(*ast.KeyValueExpr); ok {
										if entryKey, ok := entryKv.Key.(*ast.Ident); ok && entryKey.Name == "Crontab" {
											hasCrontab = true
											if bl, ok := entryKv.Value.(*ast.BasicLit); ok && bl.Value == `""` {
												errors = append(errors,
													fmt.Errorf("Empty crontab in %s at line %d", hookPath, fset.Position(bl.Pos()).Line))
											}
										}
									}
								}
								if !hasCrontab {
									errors = append(errors,
										fmt.Errorf("Missing crontab in ScheduleConfig in %s at line %d", hookPath, fset.Position(entryCl.Pos()).Line))
								}
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

func TestParseCrontabSchedule(t *testing.T) {
	testCases := []struct {
		name      string
		src       string
		errCount  int
		errSubstr []string
	}{
		{
			name: "valid crontab",
			src: `package test
			import "mod/go_hook"
			var _ = go_hook.HookConfig{
				Schedule: []go_hook.ScheduleConfig{{Crontab: "* * * * *"}},
			}`,
			errCount: 0,
		},
		{
			name: "empty crontab",
			src: `package test
			import "mod/go_hook"
			var _ = go_hook.HookConfig{
				Schedule: []go_hook.ScheduleConfig{{Crontab: ""}},
			}`,
			errCount:  1,
			errSubstr: []string{"Empty crontab"},
		},
		{
			name: "missing crontab",
			src: `package test
			import "mod/go_hook"
			var _ = go_hook.HookConfig{
				Schedule: []go_hook.ScheduleConfig{{}},
			}`,
			errCount:  1,
			errSubstr: []string{"Missing crontab"},
		},
		{
			name: "missing crontab pointer",
			src: `package test
			import "mod/go_hook"
			var _ = *go_hook.HookConfig{
				Schedule: []go_hook.ScheduleConfig{{}},
			}`,
			errCount:  1,
			errSubstr: []string{"Missing crontab"},
		},
		{
			name: "no schedule field",
			src: `package test
			import "mod/go_hook"
			var _ = go_hook.HookConfig{}`,
			errCount: 0,
		},
		{
			name: "empty schedule slice",
			src: `package test
			import "mod/go_hook"
			var _ = go_hook.HookConfig{
				Schedule: []go_hook.ScheduleConfig{},
			}`,
			errCount: 0,
		},
		{
			name: "schedule config with unrelated field",
			src: `package test
			import "mod/go_hook"
			var _ = go_hook.HookConfig{
				Schedule: []go_hook.ScheduleConfig{{Crontab: "* * * * *", Foo: "bar"}},
			}`,
			errCount: 0,
		},
		{
			name: "multiple schedule configs mixed",
			src: `package test
			import "mod/go_hook"
			var _ = go_hook.HookConfig{
				Schedule: []go_hook.ScheduleConfig{{Crontab: "* * * * *"}, {Crontab: ""}, {}},
			}`,
			errCount:  2,
			errSubstr: []string{"Empty crontab", "Missing crontab"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, tc.name+".go", tc.src, parser.AllErrors)
			require.NoError(t, err)
			errs := parseCrontabSchedule(node, tc.name+".go", fset)
			require.Len(t, errs, tc.errCount)
			for _, substr := range tc.errSubstr {
				found := false
				for _, e := range errs {
					if e != nil && (substr == "" || (e.Error() != "" && strings.Contains(e.Error(), substr))) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, but not found in errors: %v", substr, errs)
				}
			}
		})
	}
}
