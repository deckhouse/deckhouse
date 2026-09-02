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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	statecache "github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
	utilcache "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

const eksConfigFixture = "../../../../pkg/config/testdata/eks_without_cluster_configuration.yaml"

// TestInstallDeckhousePhase_RunsAgainstAnExhaustedCache pins the one thing every managed-cluster
// installation depends on: `bootstrap-phase install-deckhouse` reaches the cluster named by its
// flags without consulting the bootstrap state cache. The cache identity of a config with no
// ClusterConfiguration collapses to "--terraform-state-cache" (MetaConfig.CachePath, with an empty
// prefix and an empty provider), which is also the identity of every static cluster, and a bootstrap
// or a destroy that ran to the end leaves that directory tombstoned. Opening it there refuses the
// command - before it ever contacts the API server - for want of state it does not use, and the
// twelve EKS jobs of .github/workflows/e2e-eks.yml and tools/kind-d8.sh would meet that refusal on
// any host where a bootstrap had finished first.
func TestInstallDeckhousePhase_RunsAgainstAnExhaustedCache(t *testing.T) {
	candiDir := tests.RequireCandiDir(t)
	deckhouseDir := filepath.Dir(candiDir)

	cacheDir := t.TempDir()
	exhausted := filepath.Join(cacheDir, stringsutil.Sha256Encode("--terraform-state-cache"))
	require.NoError(t, os.MkdirAll(exhausted, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(exhausted, state.TombstoneKey), nil, 0o600))

	opts := options.New()
	opts.Cache.Dir = cacheDir
	opts.Global.CandiDir = candiDir
	opts.Global.ModulesDir = filepath.Join(deckhouseDir, "modules")
	opts.Global.GlobalHooksModule = filepath.Join(deckhouseDir, "global-hooks")
	opts.Global.VersionMap = filepath.Join(candiDir, "version_map.yml")

	kpApp := kingpin.New("dhctl", "")
	kpApp.Terminate(func(int) {})

	cmd := kpApp.Command("install-deckhouse", "")
	cmd.PreAction(kpcontext.SetContextToAction(t.Context()))
	DefineBootstrapInstallDeckhouseCommand(cmd, opts)

	// The connection flags are parsed out of os.Args by lib-connection, several frames below the
	// command action, and the arguments of the test binary are not flags it knows.
	args := os.Args
	os.Args = []string{"dhctl"}

	defer func() { os.Args = args }()

	_, err := kpApp.Parse([]string{"install-deckhouse", "--config", eksConfigFixture})

	require.Error(t, err, "no cluster is reachable from a test, so the command has to fail somewhere")
	require.NotContains(t, err.Error(), "marked as exhausted",
		"the command must fail on the cluster it cannot reach, not on a cache it does not need",
	)
	require.IsType(t, &utilcache.DummyCache{}, statecache.Global(),
		"install-deckhouse reads its cluster through the flags, so it must leave the bootstrap state cache alone",
	)
}
