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
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/local"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	utilcache "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// stubKubeProvider is a provider that exists and nothing more - which is all the input checks ask
// about. The embedded interface is nil, so a check that started calling methods on the provider
// would panic here instead of passing quietly.
type stubKubeProvider struct{ libcon.KubeProvider }

// hostlessInitializer is the connection state of a bootstrap that has no host to talk to: no
// --ssh-host, and nothing in the state cache either, so GetSSHProvider fails with
// ErrHostsFromCacheNotFound and CheckHosts is false.
func hostlessInitializer() *providerinitializer.SSHProviderInitializer {
	return providerinitializer.NewSSHProviderInitializer(
		&settings.BaseProviders{},
		&sshconfig.ConnectionConfig{Config: &sshconfig.Config{}},
	)
}

// satisfiedInputs is a cloud bootstrap whose every declared input is present, so that each case
// below removes exactly one and nothing else can be what the check reports.
func satisfiedInputs() (*ClusterBootstrapper, *bootstrapContext) {
	b := &ClusterBootstrapper{Params: &Params{
		KubeProvider:           &stubKubeProvider{},
		SSHProviderInitializer: hostlessInitializer(),
	}}

	bctx := &bootstrapContext{
		metaConfig: &config.MetaConfig{
			ClusterType:   config.CloudClusterType,
			ResourcesYAML: "kind: ModuleConfig",
		},
		stateCache:              utilcache.NewTestCache(),
		deckhouseInstallConfig:  &config.DeckhouseInstaller{CloudDiscovery: []byte("{}")},
		preflightRunner:         preflight.New(),
		nodeIP:                  "10.0.0.1",
		resourcesToCreateBefore: template.Resources{{}},
		resourcesToCreateAfter:  template.Resources{{}},
		installDeckhouseResult:  &InstallDeckhouseResult{},
	}

	return b, bctx
}

// TestPhaseInputs_MissingInputNamesItsProducer is what the input checks are for. Every one of
// these inputs fails silently otherwise - a nil dereference, a Secret installed empty, a queue of
// resources that is never created - and the message has to name the phase that was supposed to
// produce it, because the phase that trips over the absence is not the phase at fault.
func TestPhaseInputs_MissingInputNamesItsProducer(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		input    string
		phase    phases.OperationPhase
		remove   func(b *ClusterBootstrapper, bctx *bootstrapContext)
		producer string
	}{
		{
			input:    "kube provider",
			phase:    phases.WaitForControlPlaneManagerReadinessPhase,
			remove:   func(b *ClusterBootstrapper, _ *bootstrapContext) { b.KubeProvider = nil },
			producer: `"FirstMaster"`,
		},
		{
			input:    "preflight runner",
			phase:    phases.PostInfraPreflightsPhase,
			remove:   func(_ *ClusterBootstrapper, bctx *bootstrapContext) { bctx.preflightRunner = nil },
			producer: `"PreInfraPreflights"`,
		},
		{
			input:    "master node ip",
			phase:    phases.InstallKubernetesPhase,
			remove:   func(_ *ClusterBootstrapper, bctx *bootstrapContext) { bctx.nodeIP = "" },
			producer: `"FirstMaster"`,
		},
		{
			input:    "cloud discovery data",
			phase:    phases.InstallDeckhousePhase,
			remove:   func(_ *ClusterBootstrapper, bctx *bootstrapContext) { bctx.deckhouseInstallConfig.CloudDiscovery = nil },
			producer: `"BaseInfra"`,
		},
		{
			input: "split resource queues",
			phase: phases.CreateResourcesPhase,
			remove: func(_ *ClusterBootstrapper, bctx *bootstrapContext) {
				bctx.resourcesToCreateBefore = nil
				bctx.resourcesToCreateAfter = nil
			},
			producer: `"ParseResources"`,
		},
		{
			input:    "deckhouse installation result",
			phase:    phases.FinalizationPhase,
			remove:   func(_ *ClusterBootstrapper, bctx *bootstrapContext) { bctx.installDeckhouseResult = nil },
			producer: `"InstallDeckhouse"`,
		},
	} {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			b, bctx := satisfiedInputs()
			validate := b.bootstrapPhaseFuncs()[tt.phase].validate
			require.NotNil(t, validate, "phase %q declares no input check", tt.phase)
			require.NoError(t, validate(context.Background(), bctx),
				"the fixture must satisfy every input of the phase before one of them is removed",
			)

			tt.remove(b, bctx)

			err := validate(context.Background(), bctx)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.producer)
		})
	}
}

// TestPhaseInputs_NodesCarryingAnInputCheck pins the wiring itself, which nothing else does: an
// input check is only ever reached through the map, so a node that loses its check goes back to
// failing the way it used to and no other test notices. The nodes absent from this list consume
// nothing beyond what the walk already checks for all of them - the state cache and the config
// Preparation loads.
func TestPhaseInputs_NodesCarryingAnInputCheck(t *testing.T) {
	t.Parallel()

	checked := make([]phases.OperationPhase, 0)
	for name, phase := range (&ClusterBootstrapper{}).bootstrapPhaseFuncs() {
		if phase.validate != nil {
			checked = append(checked, name)
		}
	}

	require.ElementsMatch(t, []phases.OperationPhase{
		phases.PostInfraPreflightsPhase,
		phases.InstallKubernetesPhase,
		phases.InstallDeckhousePhase,
		phases.InstallAdditionalMastersAndStaticNodes,
		phases.WaitForControlPlaneManagerReadinessPhase,
		phases.CreateResourcesPhase,
		phases.FinalizationPhase,
	}, checked)
}

// TestPhaseInputs_KubeProviderOnStaticIsNotProducedByAPhase pins the one input with two producers:
// on a cloud cluster FirstMaster builds it around the master it has just created, on a static one
// it arrives ready-made from the command line. Naming FirstMaster there would send the reader
// looking for a phase that never runs on their cluster.
func TestPhaseInputs_KubeProviderOnStaticIsNotProducedByAPhase(t *testing.T) {
	t.Parallel()

	b, bctx := satisfiedInputs()
	b.KubeProvider = nil
	bctx.metaConfig.ClusterType = config.StaticClusterType

	err := b.validateKubeProviderInput(context.Background(), bctx)

	require.ErrorContains(t, err, "--ssh-host")
	require.NotContains(t, err.Error(), phases.FirstMasterPhase)
}

// TestPhaseInputs_StaticNamesNoCloudOnlyProducer walks every check with nothing in hand at all.
// Two things are asserted at once: no check dereferences what it is checking for the absence of,
// and none of them blames a phase the cluster does not have - BaseInfra and FirstMaster are gated
// out on a static cluster, so a message naming them describes an impossible fix.
func TestPhaseInputs_StaticNamesNoCloudOnlyProducer(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		metaConfig *config.MetaConfig
	}{
		{name: "static cluster", metaConfig: &config.MetaConfig{ClusterType: config.StaticClusterType}},
		// The cluster type is what Preparation produces, so a context that has not reached the end
		// of Preparation has no type at all - and an unset type is not a cloud one.
		{name: "no config loaded", metaConfig: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := &ClusterBootstrapper{Params: &Params{SSHProviderInitializer: hostlessInitializer()}}
			bctx := &bootstrapContext{metaConfig: tt.metaConfig}

			for name, phase := range b.bootstrapPhaseFuncs() {
				if phase.validate == nil {
					continue
				}

				err := phase.validate(context.Background(), bctx)
				if err == nil {
					continue
				}

				require.NotContains(t, err.Error(), phases.BaseInfraPhase, "phase %q", name)
				require.NotContains(t, err.Error(), phases.FirstMasterPhase, "phase %q", name)
			}
		})
	}
}

// TestPhaseInputs_EmptyResourceDocumentIsNotAMissingProducer covers the difference the resource
// queues have to express: empty queues are the normal state of a bootstrap without a resource
// document, and only a document that was never split means its producer did not run.
func TestPhaseInputs_EmptyResourceDocumentIsNotAMissingProducer(t *testing.T) {
	t.Parallel()

	b, bctx := satisfiedInputs()
	bctx.resourcesToCreateBefore = nil
	bctx.resourcesToCreateAfter = nil

	bctx.metaConfig.ResourcesYAML = ""
	require.NoError(t, b.validateDeckhouseInputs(context.Background(), bctx))
	require.NoError(t, b.validateCreateResourcesInputs(context.Background(), bctx))

	bctx.metaConfig.ResourcesYAML = "kind: ModuleConfig"
	require.Error(t, b.validateDeckhouseInputs(context.Background(), bctx))
	require.Error(t, b.validateCreateResourcesInputs(context.Background(), bctx))
}

// TestRequireStateCache_RejectsTheCacheThatKeepsNothing pins the input every node past Preparation
// shares. The process starts with a DummyCache that accepts every write and returns nothing, so a
// node run against it rediscovers an empty world at every step - infrastructure recreated rather
// than a run that fails.
func TestRequireStateCache_RejectsTheCacheThatKeepsNothing(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, requireStateCache(&bootstrapContext{}), `"Preparation"`)
	require.ErrorContains(t, requireStateCache(&bootstrapContext{stateCache: &utilcache.DummyCache{}}), `"Preparation"`)
	require.NoError(t, requireStateCache(&bootstrapContext{stateCache: utilcache.NewTestCache()}))
}

// TestBashibleNodeInterface_LocalInterfaceOnlyWhereItWasChosen covers the fallback the bundle must
// not take. helper.GetNodeInterface answers a failed SSH provider with the local node interface,
// which is right for a static cluster bootstrapped on the machine dhctl runs on and catastrophic
// anywhere else: the whole Kubernetes bundle is installed inside the installer container, and the
// bootstrap reports success.
func TestBashibleNodeInterface_LocalInterfaceOnlyWhereItWasChosen(t *testing.T) {
	t.Parallel()

	b := &ClusterBootstrapper{Params: &Params{SSHProviderInitializer: hostlessInitializer()}}

	t.Run("cloud cluster fails instead", func(t *testing.T) {
		t.Parallel()

		nodeInterface, err := b.bashibleNodeInterface(
			context.Background(),
			&bootstrapContext{metaConfig: &config.MetaConfig{ClusterType: config.CloudClusterType}},
		)

		require.ErrorIs(t, err, providerinitializer.ErrHostsFromCacheNotFound)
		require.Nil(t, nodeInterface)
	})

	t.Run("static cluster with no hosts runs locally", func(t *testing.T) {
		t.Parallel()

		nodeInterface, err := b.bashibleNodeInterface(
			context.Background(),
			&bootstrapContext{metaConfig: &config.MetaConfig{ClusterType: config.StaticClusterType}},
		)

		require.NoError(t, err)
		require.IsType(t, &local.NodeInterface{}, nodeInterface)
	})
}
