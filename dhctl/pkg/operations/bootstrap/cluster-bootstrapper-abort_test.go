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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
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
		if !ok || !namesIdent(branch.Body, "NewNodeAccessSuite") {
			return true
		}
		guarded = namesIdent(branch.Cond, "immutableMaster")
		return false
	})

	require.True(t, guarded,
		"%s runs the SSH-based static abort suite without excluding an immutable master", abortFile)
}

// Abort removes what a bootstrap has created, and a configuration that bootstrap refuses must not keep
// it from that: the infrastructure may come from an older dhctl or from the configuration before it was
// edited. The registry refusal is the one bootstrap gives a configuration the module leaves Unmanaged
// with no registry named. Abort parses it as the destroy it is, which does not refuse, and goes on to
// the state cache.
func TestAbort_DoesNotRefuseWhatOnlyAnInstallationRefuses(t *testing.T) {
	t.Parallel()

	const unmanagedWithoutRegistry = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.33"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  version: 1
  settings:
    mode: Unmanaged
`
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(unmanagedWithoutRegistry), 0o600))

	globalOptions := eksGlobalOptions(t)
	globalOptions.ConfigPaths = []string{configPath}

	// The configuration really is one an installation refuses.
	_, err := config.LoadConfigFromFile(t.Context(), globalOptions.ConfigPaths, config.DummyValidatorProvider(), globalOptions)
	require.ErrorIs(t, err, config.ErrNoRegistryToPullFrom)

	opts := &options.Options{Global: *globalOptions}
	opts.Cache.Dir = t.TempDir()
	b := &ClusterBootstrapper{Params: &Params{Options: opts, SSHProviderInitializer: hostlessInitializer()}}

	err = b.doRunBootstrapAbort(t.Context())
	require.NotErrorIs(t, err, config.ErrNoRegistryToPullFrom)
	require.ErrorContains(t, err, "No UUID found in the cache", "abort has to get past the parse to its state cache")
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
