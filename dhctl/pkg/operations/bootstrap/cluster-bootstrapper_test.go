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

func TestNodesComeFromResources(t *testing.T) {
	mcFlow := &config.MetaConfig{ClusterType: config.CloudClusterType}
	require.True(t, nodesComeFromResources(mcFlow))

	legacy := &config.MetaConfig{
		ClusterType:           config.CloudClusterType,
		ProviderClusterConfig: map[string]json.RawMessage{"layout": json.RawMessage(`"Standard"`)},
	}
	require.False(t, nodesComeFromResources(legacy), "a legacy cloud cluster builds its nodes from ProviderClusterConfiguration")

	static := &config.MetaConfig{ClusterType: config.StaticClusterType}
	require.False(t, nodesComeFromResources(static), "a static cluster builds no cloud nodes")
}

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

// parseResourceDocs renders the documents the way bootstrapParseResources does, so what the split
// is handed is what the parser produces - the GVK it derives, and the kind sort it applies before
// anything downstream sees the queue.
func parseResourceDocs(t *testing.T, docs string) template.Resources {
	t.Helper()

	resources, err := template.ParseResourcesContent(context.TODO(), docs, nil)
	require.NoError(t, err)

	return resources
}

// resourceNames is what the queue assertions compare: the kind and the name are what places a
// document in a queue, and reading them back names the failure.
func resourceNames(resources template.Resources) []string {
	names := make([]string, 0, len(resources))
	for _, resource := range resources {
		names = append(names, resource.GVK.Kind+"/"+resource.Object.GetName())
	}

	return names
}

func TestSplitResources_CredentialSecretGoesToBefore(t *testing.T) {
	credSecret := newResource(t, "v1", "Secret", "d8-credentials", "d8-cloud-provider-dvp", map[string]any{
		"type": "cloud-provider.deckhouse.io/credentials",
	})
	regularResource := newResource(t, "deckhouse.io/v1alpha1", "ModuleConfig", "user-authn", "", nil)

	before, _, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{credSecret, regularResource}, true, "dvp")

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

	before, _, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{plainSecret}, true, "dvp")

	require.Empty(t, before)
	require.Len(t, after, 1)
	require.Equal(t, "my-secret", after[0].Object.GetName())
}

func TestSplitResources_BeforeAnnotationStillRespected(t *testing.T) {
	annotated := newResource(t, "v1", "ConfigMap", "annotated", "kube-system", nil)
	annotated.Object.SetAnnotations(map[string]string{
		"dhctl.deckhouse.io/bootstrap-resource-place": "before-deckhouse",
	})

	before, _, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{annotated}, true, "dvp")

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

	before, _, _ := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{credSecret, explicitNS}, true, "dvp")

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

// dhctl builds the CloudPermanent nodes from these objects, so they go to the
// cluster before the build; everything else waits for the final phase.
func TestSplitResources_ProviderNodeResourcesGoToProviderQueue(t *testing.T) {
	masterNg := newResource(t, "deckhouse.io/v1", "NodeGroup", "master", "", map[string]any{
		"spec": map[string]any{"nodeType": "CloudPermanent"},
	})
	ephemeralNg := newResource(t, "deckhouse.io/v1", "NodeGroup", "ubuntu", "", map[string]any{
		"spec": map[string]any{"nodeType": "CloudEphemeral"},
	})
	instanceClass := newResource(t, "deckhouse.io/v1alpha1", "DVPInstanceClass", "master-dvp", "", nil)
	moduleConfig := newResource(t, "deckhouse.io/v1alpha1", "ModuleConfig", "user-authn", "", nil)

	before, provider, after := splitResourcesOnPreAndPostDeckhouseInstall(
		context.TODO(), template.Resources{masterNg, ephemeralNg, instanceClass, moduleConfig}, true, "dvp")

	require.Empty(t, before)

	require.Len(t, provider, 2)
	require.Equal(t, "master", provider[0].Object.GetName())
	require.Equal(t, "master-dvp", provider[1].Object.GetName())

	require.Len(t, after, 2)
	require.Equal(t, "ubuntu", after[0].Object.GetName(), "CloudEphemeral node groups must not provision before the cluster is built")
	require.Equal(t, "user-authn", after[1].Object.GetName())
}

// A Static cluster builds no cloud nodes, so nothing may be diverted out of the
// queue that actually gets applied for it.
func TestSplitResources_StaticClusterKeepsProviderResourcesInAfter(t *testing.T) {
	instanceClass := newResource(t, "deckhouse.io/v1alpha1", "DVPInstanceClass", "worker-dvp", "", nil)
	ephemeralNg := newResource(t, "deckhouse.io/v1", "NodeGroup", "ubuntu", "", map[string]any{
		"spec": map[string]any{"nodeType": "CloudEphemeral"},
	})

	before, provider, after := splitResourcesOnPreAndPostDeckhouseInstall(
		context.TODO(), template.Resources{instanceClass, ephemeralNg}, false, "dvp")

	require.Empty(t, before)
	require.Empty(t, provider, "a Static cluster never applies the provider queue")
	require.Len(t, after, 2)
	require.Equal(t, "worker-dvp", after[0].Object.GetName())
	require.Equal(t, "ubuntu", after[1].Object.GetName())
}

// node-manager's create_master_node_group hook supplies these defaults with
// CreateIfNotExists, so it never corrects an object dhctl wrote first.
func TestApplyMasterNodeGroupDefaults_FillsWhatTheUserOmitted(t *testing.T) {
	masterNg := newResource(t, "deckhouse.io/v1", "NodeGroup", "master", "", map[string]any{
		"spec": map[string]any{
			"nodeType": "CloudPermanent",
			"nodeTemplate": map[string]any{
				"labels": map[string]any{"team": "infra"},
			},
		},
	})

	applyMasterNodeGroupDefaults(template.Resources{masterNg})

	approvalMode, found, err := unstructured.NestedString(masterNg.Object.Object, "spec", "disruptions", "approvalMode")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "Manual", approvalMode)

	labels, found, err := unstructured.NestedStringMap(masterNg.Object.Object, "spec", "nodeTemplate", "labels")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, map[string]string{
		"team":                                  "infra",
		"node-role.kubernetes.io/control-plane": "",
		"node-role.kubernetes.io/master":        "",
	}, labels)

	taints, found, err := unstructured.NestedSlice(masterNg.Object.Object, "spec", "nodeTemplate", "taints")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []any{map[string]any{
		"key":    "node-role.kubernetes.io/control-plane",
		"effect": "NoSchedule",
	}}, taints)
}

func TestApplyMasterNodeGroupDefaults_KeepsWhatTheUserChose(t *testing.T) {
	masterNg := newResource(t, "deckhouse.io/v1", "NodeGroup", "master", "", map[string]any{
		"spec": map[string]any{
			"nodeType":    "CloudPermanent",
			"disruptions": map[string]any{"approvalMode": "Automatic"},
			"nodeTemplate": map[string]any{
				"labels": map[string]any{"node-role.kubernetes.io/master": "custom"},
				"taints": []any{},
			},
		},
	})

	applyMasterNodeGroupDefaults(template.Resources{masterNg})

	approvalMode, _, _ := unstructured.NestedString(masterNg.Object.Object, "spec", "disruptions", "approvalMode")
	require.Equal(t, "Automatic", approvalMode)

	labels, _, _ := unstructured.NestedStringMap(masterNg.Object.Object, "spec", "nodeTemplate", "labels")
	require.Equal(t, "custom", labels["node-role.kubernetes.io/master"])
	require.Equal(t, "", labels["node-role.kubernetes.io/control-plane"])

	taints, found, _ := unstructured.NestedSlice(masterNg.Object.Object, "spec", "nodeTemplate", "taints")
	require.True(t, found)
	require.Empty(t, taints, "an explicitly empty taint list is the user's choice")
}

// A non-string label value (an unquoted YAML scalar decodes as bool/float64,
// not string) must not make the merge drop every other label.
func TestApplyMasterNodeGroupDefaults_PreservesNonStringLabelValues(t *testing.T) {
	masterNg := newResource(t, "deckhouse.io/v1", "NodeGroup", "master", "", map[string]any{
		"spec": map[string]any{
			"nodeType": "CloudPermanent",
			"nodeTemplate": map[string]any{
				"labels": map[string]any{"team": "infra", "enabled": true},
			},
		},
	})

	applyMasterNodeGroupDefaults(template.Resources{masterNg})

	labels, found, err := unstructured.NestedMap(masterNg.Object.Object, "spec", "nodeTemplate", "labels")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, true, labels["enabled"])
	require.Equal(t, "infra", labels["team"])
	require.Equal(t, "", labels["node-role.kubernetes.io/control-plane"])
	require.Equal(t, "", labels["node-role.kubernetes.io/master"])
}

// Only the master group gets these; every other NodeGroup is left alone.
func TestApplyMasterNodeGroupDefaults_IgnoresOtherNodeGroups(t *testing.T) {
	workerNg := newResource(t, "deckhouse.io/v1", "NodeGroup", "worker", "", map[string]any{
		"spec": map[string]any{"nodeType": "CloudPermanent"},
	})
	instanceClass := newResource(t, "deckhouse.io/v1alpha1", "DVPInstanceClass", "master", "", nil)

	applyMasterNodeGroupDefaults(template.Resources{workerNg, instanceClass})

	_, found, _ := unstructured.NestedMap(workerNg.Object.Object, "spec", "disruptions")
	require.False(t, found)
	_, found, _ = unstructured.NestedMap(instanceClass.Object.Object, "spec")
	require.False(t, found, "an InstanceClass named master must not be mistaken for the NodeGroup")
}

// The documents an external provider module is installed from. They reach the split only because
// the module is not in the installer image: a ModuleConfig whose module is there is validated
// against its schema and parsed into MetaConfig.ModuleConfigs instead of ResourcesYAML, which is
// the only thing this split is fed.
const (
	providerModuleSourceDoc = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: deckhouse
spec:
  registry:
    repo: registry.deckhouse.io/deckhouse/ee/modules
`
	providerModuleConfigDoc = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  version: 1
`
	providerNodeDocs = `
---
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: master-dvp
spec:
  virtualMachineClassName: generic
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
`
	userModuleConfigDoc = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: true
`
)

// The provider module ships the InstanceClass CRDs the rest of the provider queue is written
// against, so the documents that install it lead that queue instead of waiting for the final
// phase. They are not left behind in the final queue as well: that queue would re-apply the
// config.yml body over what the controllers wrote into those same objects during the run, and
// nothing needs the copy - a bootstrap that skips the phase draining the provider queue is
// refused outright (runPhases), and the standalone create-resources command applies every
// document of the configuration without consulting this split at all.
func TestSplitResources_ExternalProviderModuleLeadsTheProviderQueue(t *testing.T) {
	resources := parseResourceDocs(t, providerModuleSourceDoc+providerModuleConfigDoc+providerNodeDocs+userModuleConfigDoc)

	before, provider, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), resources, true, "dvp")

	require.Empty(t, before)

	require.Equal(t, []string{
		"ModuleSource/deckhouse",
		"ModuleConfig/cloud-provider-dvp",
		"DVPInstanceClass/master-dvp",
		"NodeGroup/master",
	}, resourceNames(provider))

	require.Equal(t, []string{"ModuleConfig/user-authn"}, resourceNames(after))
}

// A ModulePullOverride is the one reason to pin which build of the module gets installed, so it
// has to be in the cluster before the module is installed at all - a phase later, and it can only
// replace what the release channel already deployed. The order inside the queue is the order the
// controllers consume it in: the source scan creates the Module the override controller looks up,
// and the override has to exist before the ModuleConfig enables the module.
func TestSplitResources_PullOverrideTravelsWithTheModule(t *testing.T) {
	overrideDoc := `
---
apiVersion: deckhouse.io/v1alpha2
kind: ModulePullOverride
metadata:
  name: cloud-provider-dvp
spec:
  imageTag: pr123
`
	resources := parseResourceDocs(t, providerModuleConfigDoc+overrideDoc+providerModuleSourceDoc+providerNodeDocs)

	_, provider, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), resources, true, "dvp")

	require.Equal(t, []string{
		"ModuleSource/deckhouse",
		"ModulePullOverride/cloud-provider-dvp",
		"ModuleConfig/cloud-provider-dvp",
		"DVPInstanceClass/master-dvp",
		"NodeGroup/master",
	}, resourceNames(provider))

	require.Empty(t, after)
}

// Everything that must leave the split exactly as it always was. The divert is for one case only:
// this cluster's provider module is not in the installer image, which is the only way its
// ModuleConfig can reach these documents.
func TestSplitResources_ModuleDocumentsThatDoNotDivert(t *testing.T) {
	tests := []struct {
		name               string
		docs               string
		nodesFromResources bool
		providerName       string
		wantProvider       []string
		wantAfter          []string
	}{
		{
			// An in-tree provider's ModuleConfig never reaches these documents, so a ModuleSource
			// for some other module is all there is - and it must not drag anything forward.
			name:               "in-tree provider, ModuleSource for another module",
			docs:               providerModuleSourceDoc + providerNodeDocs + userModuleConfigDoc,
			nodesFromResources: true,
			providerName:       "dvp",
			wantProvider:       []string{"DVPInstanceClass/master-dvp", "NodeGroup/master"},
			wantAfter:          []string{"ModuleSource/deckhouse", "ModuleConfig/user-authn"},
		},
		{
			// The cloud-provider ModuleConfig migration leaves the previous provider's entry
			// behind, and that module ships no CRD this cluster's nodes are written against.
			name:               "ModuleConfig of a provider this cluster does not run",
			docs:               providerModuleSourceDoc + providerModuleConfigDoc + providerNodeDocs,
			nodesFromResources: true,
			providerName:       "openstack",
			wantProvider:       []string{"DVPInstanceClass/master-dvp", "NodeGroup/master"},
			wantAfter:          []string{"ModuleSource/deckhouse", "ModuleConfig/cloud-provider-dvp"},
		},
		{
			// An override with no spec.imageTag names no image to pull, and the bundle resolver
			// ignores it for exactly that reason (ModuleDocs.ImageTags). The two must agree.
			name: "ModulePullOverride without an image tag",
			docs: providerModuleSourceDoc + providerNodeDocs + `
---
apiVersion: deckhouse.io/v1alpha2
kind: ModulePullOverride
metadata:
  name: cloud-provider-dvp
spec:
  scanInterval: 60s
`,
			nodesFromResources: true,
			providerName:       "dvp",
			wantProvider:       []string{"DVPInstanceClass/master-dvp", "NodeGroup/master"},
			wantAfter:          []string{"ModuleSource/deckhouse", "ModulePullOverride/cloud-provider-dvp"},
		},
		{
			// A static cluster applies the provider queue nowhere, so diverting into it would
			// drop the module documents entirely.
			name:               "static cluster",
			docs:               providerModuleSourceDoc + providerModuleConfigDoc,
			nodesFromResources: false,
			providerName:       "",
			wantProvider:       []string{},
			wantAfter:          []string{"ModuleSource/deckhouse", "ModuleConfig/cloud-provider-dvp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := parseResourceDocs(t, tt.docs)

			before, provider, after := splitResourcesOnPreAndPostDeckhouseInstall(
				context.TODO(), resources, tt.nodesFromResources, tt.providerName)

			require.Empty(t, before)
			require.Equal(t, tt.wantProvider, resourceNames(provider))
			require.Equal(t, tt.wantAfter, resourceNames(after))
		})
	}
}
