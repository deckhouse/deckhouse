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

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
)

// In a cloud the provider carries the document in as the machine's cloud config. Creating the
// first master without one leaves it waiting in the installer with no cluster to report that to,
// and the maintenance port - the static cluster's transport - would then have to be reached at an
// address nothing publishes. Checked on the source: what the provider was handed is not observable
// from what the phase returns.
func TestTheFirstMasterIsCreatedWithItsDocument(t *testing.T) {
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
		require.True(t, namesIdent(fn.Body, "NodeCloudConfig"),
			"%s creates the first master with no cloud config: an immutable machine boots bare and never joins", file)
		require.True(t, namesIdent(fn.Body, "buildImmutableMasterPayload"),
			"%s creates the first master without rendering its document", file)
		return false
	})

	require.True(t, checked, "bootstrapFirstMaster not found in %s", file)
}

// A cloud master of a classic cluster must be seeded with nothing: the payload builder answers ""
// off an immutable-less bootstrap, and a stray document there would be a second source of truth
// beside the group's own cloud config.
func TestAClassicCloudMasterIsSeededWithNothing(t *testing.T) {
	t.Parallel()

	b := &ClusterBootstrapper{}
	bctx := &bootstrapContext{}

	cloudConfig, nodeConfig, err := b.buildImmutableMasterPayload(t.Context(), bctx, "master-0", nil)
	require.NoError(t, err)
	require.Empty(t, cloudConfig)
	require.Empty(t, nodeConfig)
}

// The channel to an immutable cloud master is opened at whatever this phase records. With no
// --ssh-bastion-host nothing rewrites that address, so the private NodeInternalIP is unroutable
// from outside the cloud network and the run burns its whole API-server budget before dying.
func TestAnImmutableCloudMasterIsReachedAtTheReportedAddress(t *testing.T) {
	t.Parallel()

	address, err := immutableCloudMasterAddress("master-0", &infrastructure.PipelineOutputs{
		MasterIPForSSH: "203.0.113.7",
		NodeInternalIP: "10.241.32.7",
	})
	require.NoError(t, err)
	require.Equal(t, "203.0.113.7", address)
}

// An empty address means talking to whatever answers on the machine dhctl runs on.
func TestAnImmutableCloudMasterWithoutAnAddressIsRefused(t *testing.T) {
	t.Parallel()

	_, err := immutableCloudMasterAddress("master-0", &infrastructure.PipelineOutputs{
		NodeInternalIP: "10.241.32.7",
	})
	require.ErrorContains(t, err, "master-0")
}
