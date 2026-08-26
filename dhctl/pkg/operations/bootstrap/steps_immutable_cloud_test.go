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

// An immutable machine takes its configuration from the maintenance port, the same way a static
// cluster's machines do. Handing the provider a NodeCloudConfig puts the document back on the
// cloud-init path, where the image's init and the agent parse it separately and neither checks it
// against the hardware. Checked on the source: what the provider was handed is not observable from
// what the phase returns.
func TestTheFirstMasterIsCreatedWithNoCloudConfig(t *testing.T) {
	t.Parallel()

	const file = "cluster-bootstrapper.go"

	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	require.NoError(t, err)

	var checked bool
	ast.Inspect(parsed, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "bootstrapFirstMaster" {
			return true
		}
		checked = true
		require.False(t, namesIdent(fn.Body, "NodeCloudConfig"),
			"%s hands the provider a NodeCloudConfig; an immutable machine is pushed to, not seeded", file)
		return false
	})

	require.True(t, checked, "bootstrapFirstMaster not found in %s", file)
}

// The address comes from the infrastructure, and there is nothing to push to without it. Refused
// rather than pushed at "": an empty address dials whatever answers on the machine dhctl runs on.
func TestTheCloudMasterNeedsAnAddress(t *testing.T) {
	t.Parallel()

	b, bctx := immutableTestBootstrapper(t)

	err := b.handImmutableCloudMaster(t.Context(), bctx, "master-0", "")
	require.ErrorContains(t, err, "no address")
	require.ErrorContains(t, err, "master-0")
}
