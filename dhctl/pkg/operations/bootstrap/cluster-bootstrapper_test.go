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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
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

	before, _, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{credSecret, regularResource}, true)

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

	before, _, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{plainSecret}, true)

	require.Empty(t, before)
	require.Len(t, after, 1)
	require.Equal(t, "my-secret", after[0].Object.GetName())
}

func TestSplitResources_BeforeAnnotationStillRespected(t *testing.T) {
	annotated := newResource(t, "v1", "ConfigMap", "annotated", "kube-system", nil)
	annotated.Object.SetAnnotations(map[string]string{
		"dhctl.deckhouse.io/bootstrap-resource-place": "before-deckhouse",
	})

	before, _, after := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{annotated}, true)

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

	before, _, _ := splitResourcesOnPreAndPostDeckhouseInstall(context.TODO(), template.Resources{credSecret, explicitNS}, true)

	// Only one Namespace entry — the user-provided one, no auto-stub.
	nsCount := 0
	for _, r := range before {
		if r.Object.GetKind() == "Namespace" {
			nsCount++
		}
	}
	require.Equal(t, 1, nsCount)
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
		context.TODO(), template.Resources{masterNg, ephemeralNg, instanceClass, moduleConfig}, true)

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
		context.TODO(), template.Resources{instanceClass, ephemeralNg}, false)

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
