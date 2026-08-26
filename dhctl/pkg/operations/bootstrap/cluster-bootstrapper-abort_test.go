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

package bootstrap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAbortNeverNamesKubeAPI is the abort invariant checked instead of agreed: abort must not use
// the Kube API and must not delete cluster resources. Neither shows up in what abort returns - both
// are decided by what it is allowed to construct, so they are checked on the source. A kube provider
// is the only way into the API from this package, and destroy.NewClusterDestroyer reaches the
// cluster while it is still being built (destroy.initStateLoader) and then deletes its resources.
// destroy.GetAbortDestroyer, the one entry point left, has no field for a kube client at all.
func TestAbortNeverNamesKubeAPI(t *testing.T) {
	t.Parallel()

	const abortFile = "cluster-bootstrapper-abort.go"

	forbidden := map[string]string{
		"NewClusterDestroyer": "the cluster destroyer connects to the cluster and deletes its resources - that is what dhctl destroy is for",
		"GetKubeProvider":     "abort does not use the Kube API",
		"KubeProvider":        "abort does not use the Kube API",
	}

	file, err := parser.ParseFile(token.NewFileSet(), abortFile, nil, 0)
	require.NoError(t, err)

	ast.Inspect(file, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		why, banned := forbidden[ident.Name]
		require.Falsef(t, banned, "%s names %s: %s", abortFile, ident.Name, why)

		return true
	})
}

// The static abort suite is SSHCredential plus SudoAllowed, and an immutable machine offers no
// sshd: with no hosts GetNodeInterface hands back the installer container, so the suite would
// check the container and report it as the cluster. Checked on the source for the same reason as
// the invariant above — what abort returns does not say which suite it ran.
func TestStaticAbortSuiteIsNotRunForImmutableMachines(t *testing.T) {
	t.Parallel()

	const abortFile = "cluster-bootstrapper-abort.go"

	file, err := parser.ParseFile(token.NewFileSet(), abortFile, nil, 0)
	require.NoError(t, err)

	var guarded bool
	ast.Inspect(file, func(n ast.Node) bool {
		branch, ok := n.(*ast.IfStmt)
		if !ok || !namesIdent(branch.Body, "NewStaticAbortSuite") {
			return true
		}
		guarded = namesIdent(branch.Cond, "immutableMaster")
		return false
	})

	require.True(t, guarded,
		"%s runs the SSH-based static abort suite without excluding an immutable master", abortFile)
}

// namesIdent reports whether n mentions the given identifier anywhere inside it.
func namesIdent(n ast.Node, name string) bool {
	var found bool
	ast.Inspect(n, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && ident.Name == name {
			found = true
		}
		return !found
	})
	return found
}
