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
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
	utilcache "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// eksConfigFixture is testing/cloud_layouts/EKS/WithoutNAT/configuration.tpl.yaml with the
// envsubst placeholders filled in: as a template it carries `${IMAGES_REPO}` where the
// InitConfiguration schema wants a registry address, and would fail validation for that reason
// instead of proving anything about the ClusterConfiguration it does not carry.
const eksConfigFixture = "../../config/testdata/eks_without_cluster_configuration.yaml"

// eksGlobalOptions is the loader environment this config needs, and every caller below needs all of
// it. The candi schemas decide whether InitConfiguration is recognised at all: with no schema for
// it the document fails the lookup and is silently filed away as a resource. The loaders read two
// more things on their way past the refusal this test used to pin: the version map out of candi,
// and <DeckhouseDir>/version, which on a developer machine is the /deckhouse the installer image
// has and the checkout does not.
func eksGlobalOptions(t *testing.T) *options.GlobalOptions {
	t.Helper()

	candiDir := tests.RequireCandiDir(t)
	deckhouseDir := filepath.Dir(candiDir)
	modulesDir := filepath.Join(deckhouseDir, "modules")
	globalHooksModule := filepath.Join(deckhouseDir, "global-hooks")

	// DHCTL_TEST_CANDI_DIR may point at a candi tree whose parent holds neither sibling, and a
	// fixture this machine does not have is a skip, not a failure of the code under test.
	tests.RequireDir(t, modulesDir, "modules of the deckhouse tree next to candi")
	tests.RequireDir(t, globalHooksModule, "global-hooks of the deckhouse tree next to candi")

	versionDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(versionDir, "version"), []byte("dev"), 0o600))

	return &options.GlobalOptions{
		CandiDir:          candiDir,
		ModulesDir:        modulesDir,
		GlobalHooksModule: globalHooksModule,
		VersionMap:        filepath.Join(candiDir, "version_map.yml"),
		DeckhouseDir:      versionDir,
	}
}

// TestEKSConfigWithoutClusterConfiguration is the acceptance criterion of the second gate axis:
// install-deckhouse and create-resources must keep working on a config with no
// ClusterConfiguration. That config is not hypothetical - twelve jobs of .github/workflows/e2e-eks.yml
// run it today - but they run on a label, so a regression would land before any of them votes.
// Hence the preconditions of both commands are pinned here instead: the config loads, the install
// config they build their installation out of comes back, the checks that run ahead of every gate
// pass, and the phase list a full bootstrap walks keeps the two nodes that reach them.
func TestEKSConfigWithoutClusterConfiguration(t *testing.T) {
	globalOptions := eksGlobalOptions(t)

	metaConfig, err := config.ParseConfig(
		t.Context(),
		[]string{eksConfigFixture},
		infrastructureprovider.MetaConfigValidatorProvider(),
		globalOptions,
	)
	require.NoError(t, err, "ParseConfig is what InstallDeckhouse and CreateResources load the config with")
	require.False(t, metaConfig.HasClusterConfiguration())
	require.Equal(t, "registry.deckhouse.io/deckhouse/ee", metaConfig.DeckhouseConfig.ImagesRepo,
		"InitConfiguration must be parsed, not filed away as a resource for want of a schema",
	)

	// The one thing both install-deckhouse and create-resources run over the config as a whole.
	installConfig, err := config.PrepareDeckhouseInstallConfig(t.Context(), metaConfig, globalOptions)
	require.NoError(t, err)
	require.Empty(t, installConfig.UUID,
		"kube-system/d8-cluster-uuid is the only authority on such a cluster, and the adoption in install-deckhouse stands on dhctl arriving without one",
	)

	globalChecks := suites.NewGlobalSuite(suites.GlobalDeps{MetaConfig: metaConfig}).Checks()

	for _, name := range []preflight.CheckName{checks.CidrIntersectionCheckName, checks.PublicDomainTemplateCheckName} {
		at := slices.IndexFunc(globalChecks, func(c preflight.Check) bool { return c.Name == name })
		require.GreaterOrEqualf(t, at, 0, "check %q left the global suite", name)
		require.NoErrorf(t, globalChecks[at].Run(t.Context()), "check %q runs before any gate and must pass", name)
	}

	declared := make([]phases.OperationPhase, 0)
	for _, phase := range phases.PhasesFor(phases.OperationBootstrap, phaseClusterConfig(metaConfig)) {
		declared = append(declared, phase.Phase)
	}

	assert.Contains(t, declared, phases.InstallDeckhousePhase)
	assert.Contains(t, declared, phases.CreateResourcesPhase)
	assert.NotContains(t, declared, phases.InstallKubernetesPhase)

	// The other half of the decision: a full `dhctl bootstrap` has to get as far as resolving the
	// phase tree above, and it loads its config with LoadConfigFromFile and the two options no
	// other caller passes. While that loader refused every config without a ClusterConfiguration,
	// both gates in the tree were unreachable in production and a managed cluster could only be
	// installed with the two bootstrap-phase commands.
	fullBootstrapConfig, err := config.LoadConfigFromFile(
		t.Context(),
		[]string{eksConfigFixture},
		infrastructureprovider.MetaConfigValidatorProvider(),
		globalOptions,
		config.ValidateOptionValidateExtensions(true),
		config.ValidateOptionOperation(infrastructureprovider.DhctlOperationBootstrap),
	)
	require.NoError(t, err, "the loader of a full bootstrap must accept a cluster whose control plane dhctl did not create")
	require.False(t, fullBootstrapConfig.HasClusterConfiguration())
	require.Empty(t, fullBootstrapConfig.ClusterType, "with no ClusterConfiguration the cluster type is unknown, not static")
}

// TestPreflightSuites_WithoutClusterConfigurationTheGlobalSuiteIsAll is the other half of a full
// bootstrap on a managed cluster. The selection used to ask a single question - cloud or not - and
// a cluster with no ClusterConfiguration has no type at all, so it fell into the static arm:
// sixteen checks over SSH hosts that do not exist and, worse, over the node interface
// helper.GetNodeInterface hands back when there is no SSH provider to build - the installer
// container, where sudo, python and the deckhouse user would then be checked and created.
func TestPreflightSuites_WithoutClusterConfigurationTheGlobalSuiteIsAll(t *testing.T) {
	t.Parallel()

	b := &ClusterBootstrapper{Params: &Params{
		Options:                &options.Options{},
		SSHProviderInitializer: hostlessInitializer(),
	}}

	selected, err := b.preflightSuites(t.Context(), &bootstrapContext{metaConfig: &config.MetaConfig{}})
	require.NoError(t, err)
	require.Equal(t, checkNames(suites.NewGlobalSuite(suites.GlobalDeps{})), checkNames(selected...))

	selected, err = b.preflightSuites(t.Context(), &bootstrapContext{
		metaConfig: bootstrappedMetaConfig(config.StaticClusterType),
	})
	require.NoError(t, err)
	require.Contains(t, checkNames(selected...), checks.SudoAllowedCheckName,
		"a cluster whose control plane dhctl does build keeps the checks over the node it builds on",
	)
}

// TestBootstrapPreflight_ParksTheRunnerPostInfraPreflightsReads covers the input the new selection
// path is one forgotten line away from losing. Nothing reads the runner while PostInfraPreflights
// is gated out of the tree, so a missing assignment would surface only once that node is reachable
// again - as a nil dereference, several phases in.
func TestBootstrapPreflight_ParksTheRunnerPostInfraPreflightsReads(t *testing.T) {
	t.Parallel()

	opts := &options.Options{}
	opts.Preflight.SkipAll = true

	b := &ClusterBootstrapper{Params: &Params{Options: opts, SSHProviderInitializer: hostlessInitializer()}}
	bctx := &bootstrapContext{
		metaConfig:     &config.MetaConfig{},
		bootstrapState: NewBootstrapState(utilcache.NewTestCache()),
	}

	require.NoError(t, b.bootstrapPreflight(t.Context(), bctx))
	require.NoError(t, b.validatePostInfraPreflightsInputs(t.Context(), bctx))
}

// TestAbort_RefusesWithoutClusterConfiguration keeps the other refusal the loader gave up. Abort
// has nothing to destroy on a cluster whose control plane dhctl did not create, and it does not
// find that out on its own until GetAbortDestroyer - by which point the state cache, whose name
// collapses to the one directory every prefix-less cluster shares, has been initialised and
// written to.
func TestAbort_RefusesWithoutClusterConfiguration(t *testing.T) {
	t.Parallel()

	globalOptions := eksGlobalOptions(t)
	globalOptions.ConfigPaths = []string{eksConfigFixture}

	b := &ClusterBootstrapper{Params: &Params{Options: &options.Options{Global: *globalOptions}}}

	require.ErrorContains(t, b.doRunBootstrapAbort(t.Context()), "dhctl bootstrap-phase abort requires ClusterConfiguration")
}

func checkNames(list ...preflight.Suite) []preflight.CheckName {
	names := make([]preflight.CheckName, 0)

	for _, suite := range list {
		for _, check := range suite.Checks() {
			names = append(names, check.Name)
		}
	}

	return names
}
