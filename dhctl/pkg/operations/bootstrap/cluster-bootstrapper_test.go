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
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	utilcache "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func newResource(t *testing.T, apiVersion, kind, name, namespace string, fields map[string]any) *template.Resource {
	t.Helper()
	obj := unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetName(name)
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	for k, v := range fields {
		obj.Object[k] = v
	}
	return &template.Resource{
		GVK:    schema.FromAPIVersionAndKind(apiVersion, kind),
		Object: obj,
	}
}

func TestSplitResources_CredentialSecretGoesToBefore(t *testing.T) {
	credSecret := newResource(t, "v1", "Secret", "d8-credentials", "d8-cloud-provider-dvp", map[string]any{
		"type": "cloud-provider.deckhouse.io/credentials",
	})
	regularResource := newResource(t, "deckhouse.io/v1alpha1", "ModuleConfig", "user-authn", "", nil)

	before, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{credSecret, regularResource})

	// before queue must contain the credential Secret AND a namespace stub for d8-cloud-provider-dvp.
	require.Len(t, before, 2)
	require.Equal(t, "Namespace", before[0].Object.GetKind())
	require.Equal(t, "d8-cloud-provider-dvp", before[0].Object.GetName())
	require.Equal(t, "Secret", before[1].Object.GetKind())
	require.Equal(t, "d8-credentials", before[1].Object.GetName())

	require.Len(t, after, 1)
	require.Equal(t, "ModuleConfig", after[0].Object.GetKind())
}

func TestSplitResources_NonCredentialSecretGoesToAfter(t *testing.T) {
	plainSecret := newResource(t, "v1", "Secret", "my-secret", "default", map[string]any{
		"type": "Opaque",
	})

	before, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{plainSecret})

	require.Empty(t, before)
	require.Len(t, after, 1)
	require.Equal(t, "my-secret", after[0].Object.GetName())
}

func TestSplitResources_BeforeAnnotationStillRespected(t *testing.T) {
	annotated := newResource(t, "v1", "ConfigMap", "annotated", "kube-system", nil)
	annotated.Object.SetAnnotations(map[string]string{
		"dhctl.deckhouse.io/bootstrap-resource-place": "before-deckhouse",
	})

	before, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{annotated})

	require.Empty(t, after)
	// Namespace stub for kube-system is added even though kube-system always exists; harmless.
	require.Len(t, before, 2)
	require.Equal(t, "Namespace", before[0].Object.GetKind())
	require.Equal(t, "ConfigMap", before[1].Object.GetKind())
}

func TestSplitResources_ExplicitNamespaceNotDuplicated(t *testing.T) {
	credSecret := newResource(t, "v1", "Secret", "d8-credentials", "my-ns", map[string]any{
		"type": "cloud-provider.deckhouse.io/credentials",
	})
	explicitNS := newResource(t, "v1", "Namespace", "my-ns", "", nil)
	explicitNS.Object.SetAnnotations(map[string]string{
		"dhctl.deckhouse.io/bootstrap-resource-place": "before-deckhouse",
	})

	before, _ := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{credSecret, explicitNS})

	// Only one Namespace entry — the user-provided one, no auto-stub.
	nsCount := 0
	for _, r := range before {
		if r.Object.GetKind() == "Namespace" {
			nsCount++
		}
	}
	require.Equal(t, 1, nsCount)
}

// TestBootstrapPhaseFuncsMatchTree is the declared-equals-executed invariant, checked instead of
// agreed: the walker resolves the tree and looks every node up by name, so a node declared with
// nothing behind it is a phase Commander is told about that never runs, and a function with no
// node is work that never happens. The tree here is the ungated one - gates decide which nodes
// run in a given cluster, not which ones have to be implemented.
func TestBootstrapPhaseFuncsMatchTree(t *testing.T) {
	t.Parallel()

	declared := make([]phases.OperationPhase, 0)
	for _, phase := range phases.BootstrapPhases() {
		declared = append(declared, phase.Phase)
	}

	implemented := make([]phases.OperationPhase, 0)
	for phase := range (&ClusterBootstrapper{}).bootstrapPhaseFuncs() {
		implemented = append(implemented, phase)
	}

	require.ElementsMatch(t, declared, implemented)
}

// TestSaveStaticConnectionToCache_RecordsEveryHost pins the only record a static bootstrap leaves of
// the hosts it touched. Recording just the first one leaves an abort of a multi-master cluster
// unable to reach the rest, and they keep a half-installed control plane running.
func TestSaveStaticConnectionToCache_RecordsEveryHost(t *testing.T) {
	t.Parallel()

	stateCache := utilcache.NewTestCache()
	connectionConfig := &sshconfig.ConnectionConfig{
		Config: &sshconfig.Config{},
		Hosts: []sshconfig.Host{
			{Host: "10.0.0.1"},
			{Host: "10.0.0.2"},
			{Host: "10.0.0.3"},
		},
	}

	require.NoError(t, saveStaticConnectionToCache(t.Context(), stateCache, connectionConfig))

	hosts, err := state.GetMasterHostsIPs(t.Context(), stateCache)
	require.NoError(t, err)

	addresses := make([]string, 0, len(hosts))
	for _, host := range hosts {
		addresses = append(addresses, host.Host)
	}

	require.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, addresses)
}

func TestSaveStaticConnectionToCache_WithoutHostsRecordsNothing(t *testing.T) {
	t.Parallel()

	stateCache := utilcache.NewTestCache()

	require.NoError(t, saveStaticConnectionToCache(t.Context(), stateCache, &sshconfig.ConnectionConfig{Config: &sshconfig.Config{}}))

	inCache, err := stateCache.InCache(t.Context(), state.MasterHostsCacheKey)
	require.NoError(t, err)
	require.False(t, inCache)
}

// bootstrapperAskingProvider records whether the cleanup asked for a cloud provider at all. The
// getter fails the way the real one does on a cluster carrying no UUID, so a run that asks is a
// run that dies in Preparation.
func bootstrapperAskingProvider(asked *bool) *ClusterBootstrapper {
	return &ClusterBootstrapper{
		Params: &Params{
			InfrastructureContext: infrastructure.NewContextWithProvider(
				func(context.Context, *config.MetaConfig) (infrastructure.CloudProvider, error) {
					*asked = true

					return nil, errors.New("Unable to get full UUID for provider '/'. It is empty")
				},
			),
		},
	}
}

func TestGetCleanupFunc_WithoutClusterConfigurationNoProviderIsAsked(t *testing.T) {
	t.Parallel()

	asked := false

	cleanup, err := bootstrapperAskingProvider(&asked).getCleanupFunc(t.Context(), &config.MetaConfig{})

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	require.False(t, asked, "a cluster whose control plane dhctl did not build unpacks no cloud provider")
}

func TestGetCleanupFunc_WithClusterConfigurationTheProviderIsStillReleased(t *testing.T) {
	t.Parallel()

	asked := false
	metaConfig := &config.MetaConfig{ClusterConfig: map[string]json.RawMessage{"clusterType": json.RawMessage(`"Static"`)}}

	_, err := bootstrapperAskingProvider(&asked).getCleanupFunc(t.Context(), metaConfig)

	require.Error(t, err)
	require.True(t, asked, "the guard must not swallow the cleanup of a cluster dhctl does build")
}
