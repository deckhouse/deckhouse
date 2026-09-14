/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudprovider

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/node-controller/internal/machinetemplate"
)

func clusterTemplateTestData() RenderData {
	return RenderData{
		Provider: map[string]any{"region": "region-one"},
		Cluster: machinetemplate.ClusterFacts{
			Name: "openstack", Namespace: capiNamespace, UUID: "uuid", PodSubnet: "10.111.0.0/16",
		},
		Prefix:        "prod",
		ControlPlane:  []ControlPlaneEndpoint{{Host: "192.0.2.10", Port: 6443}},
		InstanceClass: map[string]any{"flavor": "m1.large"},
		Zone:          "zone-a",
		NodeGroupName: "worker",
	}
}

func TestClusterTemplateRender(t *testing.T) {
	registration := Registration{
		CAPIClusterName: "openstack", CAPIClusterKind: "OpenStackCluster",
		CAPIClusterAPIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
	}
	template, err := newClusterTemplate([]byte(`version: v1
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: OpenStackCluster
  metadata:
    name: {{ .cluster.name }}
  spec:
    prefix: {{ .prefix }}
    endpoint: {{ (first .controlPlane.endpoints).host }}
`), registration)
	require.NoError(t, err)

	object, err := template.Render(clusterTemplateTestData())
	require.NoError(t, err)
	assert.Equal(t, capiNamespace, object.GetNamespace())
	assert.Equal(t, "prod", object.Object["spec"].(map[string]any)["prefix"])

	for _, testCase := range []struct {
		name     string
		manifest string
		error    string
	}{
		{
			name:     "wrong kind",
			manifest: "apiVersion: infrastructure.cluster.x-k8s.io/v1beta1\nkind: Other\nmetadata: {name: openstack}\n",
			error:    "registration declares",
		},
		{
			name:     "wrong name",
			manifest: "apiVersion: infrastructure.cluster.x-k8s.io/v1beta1\nkind: OpenStackCluster\nmetadata: {name: other}\n",
			error:    "registration declares",
		},
		{
			name:     "wrong namespace",
			manifest: "apiVersion: infrastructure.cluster.x-k8s.io/v1beta1\nkind: OpenStackCluster\nmetadata: {name: openstack, namespace: other}\n",
			error:    "must be in namespace",
		},
		{
			name:     "multiple objects",
			manifest: "apiVersion: infrastructure.cluster.x-k8s.io/v1beta1\nkind: OpenStackCluster\nmetadata: {name: openstack}\n---\napiVersion: v1\nkind: Secret\nmetadata: {name: credentials}\n",
			error:    "more than one object",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			candidate, err := newClusterTemplate([]byte("version: v1\ntemplate: |\n  "+indentTemplateTest(testCase.manifest)), registration)
			require.NoError(t, err)
			_, err = candidate.Render(clusterTemplateTestData())
			require.ErrorContains(t, err, testCase.error)
		})
	}
}

func TestTemplateContextsExposeOnlyContractRoots(t *testing.T) {
	data := clusterTemplateTestData()
	machine := (&MachineTemplate{}).renderContext(data)
	machineContext, err := machine.ToMap()
	require.NoError(t, err)
	assert.Equal(t, []string{"cluster", "instanceClass", "nodeGroup", "provider", "zone"}, sortedKeys(machineContext))

	cluster := &ClusterTemplate{}
	assert.Equal(t, []string{"cluster", "controlPlane", "prefix", "provider"}, sortedKeys(cluster.context(data)))

	data.ControlPlane = nil
	assert.Equal(t, []string{"cluster", "prefix", "provider"}, sortedKeys(cluster.context(data)))
}

func TestCredentialsTemplateHasRestrictedContext(t *testing.T) {
	template, err := newCredentialsTemplate([]byte(`version: v1
template: |
  apiVersion: v1
  kind: Secret
  metadata:
    name: credentials
  stringData:
    region: {{ .provider.region }}
    cluster: {{ .cluster.name }}
`))
	require.NoError(t, err)
	secret, err := template.Render(clusterTemplateTestData())
	require.NoError(t, err)
	assert.Equal(t, "region-one", secret.StringData["region"])
	assert.Equal(t, "openstack", secret.StringData["cluster"])

	template, err = newCredentialsTemplate([]byte(`version: v1
template: |
  apiVersion: v1
  kind: Secret
  metadata: {name: credentials}
  stringData:
    prefix: {{ .prefix }}
`))
	require.NoError(t, err)
	_, err = template.Render(clusterTemplateTestData())
	require.ErrorContains(t, err, "prefix")
}

func TestCredentialsTemplateValidatesOutput(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		error    string
	}{
		{name: "wrong kind", manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: credentials}\n", error: "want v1/Secret"},
		{name: "wrong namespace", manifest: "apiVersion: v1\nkind: Secret\nmetadata: {name: credentials, namespace: other}\n", error: "must be in namespace"},
		{name: "multiple objects", manifest: "apiVersion: v1\nkind: Secret\nmetadata: {name: credentials}\n---\napiVersion: v1\nkind: Secret\nmetadata: {name: other}\n", error: "more than one object"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			candidate, err := newCredentialsTemplate([]byte("version: v1\ntemplate: |\n  " + indentTemplateTest(testCase.manifest)))
			require.NoError(t, err)
			_, err = candidate.Render(clusterTemplateTestData())
			require.ErrorContains(t, err, testCase.error)
		})
	}
}

func TestTemplateEnvelopeIsStrict(t *testing.T) {
	_, err := parseTemplateEnvelope("cluster.yaml", []byte("version: v1\nunknown: true\ntemplate: x\n"))
	require.ErrorContains(t, err, "unknown")
}

func sortedKeys(values map[string]any) []string {
	keys := slices.Collect(maps.Keys(values))
	slices.Sort(keys)
	return keys
}

func indentTemplateTest(value string) string {
	result := ""
	for index, character := range value {
		if index > 0 && value[index-1] == '\n' && character != '\n' {
			result += "  "
		}
		result += string(character)
	}
	return result
}
