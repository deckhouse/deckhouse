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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/node-controller/internal/machinetemplate"
)

const repositoryRoot = "../../../../../../.."

type helmClusterFixture struct {
	name       string
	path       string
	apiVersion string
	kind       string
	provider   map[string]any
	prefix     string
}

// TestClusterTemplatesMatchHelm compares provider-owned contracts with output frozen before
// InfrastructureCluster rendering moved out of the node-manager Helm chart. The fixture data is
// normalized only by decoding Secret.data, the same transformation the Kubernetes API performs.
func TestClusterTemplatesMatchHelm(t *testing.T) {
	fixtures := []helmClusterFixture{
		{name: "dvp", path: "modules/030-cloud-provider-dvp", apiVersion: "infrastructure.cluster.x-k8s.io/v1alpha1", kind: "DeckhouseCluster", provider: map[string]any{}},
		{name: "yandex", path: "modules/030-cloud-provider-yandex", apiVersion: "infrastructure.cluster.x-k8s.io/v1alpha1", kind: "YandexCluster", provider: map[string]any{
			"folderID": "folder-id", "zoneToSubnetIdMap": map[string]any{"ru-central1-a": "subnet-a", "ru-central1-b": "subnet-b"},
		}},
		{name: "openstack", path: "ee/modules/030-cloud-provider-openstack", apiVersion: "infrastructure.cluster.x-k8s.io/v1beta1", kind: "OpenStackCluster", provider: map[string]any{
			"connection": map[string]any{
				"authURL": "https://keystone.example/v3", "username": "cloud-user", "password": "cloud-password",
				"tenantName": "project-name", "domainName": "Default", "region": "RegionOne", "caCert": "test-ca",
			},
			"apiServerFloatingIP": false, "internalNetworkNames": []any{"internal"}, "externalNetworkNames": []any{"public"},
		}},
		{name: "vcd", path: "ee/modules/030-cloud-provider-vcd", apiVersion: "infrastructure.cluster.x-k8s.io/v1beta2", kind: "VCDCluster", provider: map[string]any{
			"server": "https://vcd.example/", "organization": "test-org", "virtualDataCenter": "test-ovdc",
			"mainNetwork": "test-network", "username": "user", "password": "pass", "apiToken": "token",
		}},
		{name: "dynamix", path: "ee/modules/030-cloud-provider-dynamix", apiVersion: "infrastructure.cluster.x-k8s.io/v1alpha1", kind: "DynamixCluster", provider: map[string]any{}, prefix: "test-prefix"},
		{name: "huaweicloud", path: "ee/modules/030-cloud-provider-huaweicloud", apiVersion: "infrastructure.cluster.x-k8s.io/v1alpha1", kind: "HuaweiCloudCluster", provider: map[string]any{}},
		{name: "zvirt", path: "ee/se-plus/modules/030-cloud-provider-zvirt", apiVersion: "infrastructure.cluster.x-k8s.io/v1", kind: "ZvirtCluster", provider: map[string]any{"clusterID": "cluster-id"}},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			clusterName := fixture.name
			if fixture.name == "vcd" {
				clusterName = "test-vapp"
			}
			registration := Registration{
				CAPIClusterName: clusterName, CAPIClusterKind: fixture.kind,
				CAPIClusterAPIVersion: fixture.apiVersion,
			}
			data := RenderData{
				Provider:     fixture.provider,
				Cluster:      machinetemplate.ClusterFacts{Name: clusterName, Namespace: capiNamespace},
				Prefix:       fixture.prefix,
				ControlPlane: []ControlPlaneEndpoint{{Host: "192.0.2.10", Port: 6443}},
			}

			contract, err := newClusterTemplate(readFixtureFile(t, fixture.path, "capi/cluster.yaml"), registration)
			require.NoError(t, err)
			actual, err := contract.Render(data)
			require.NoError(t, err)
			addLegacyModuleLabels(actual.GetLabels(), actual.SetLabels)

			expected := readGoldenObject(t, fixture.name, "cluster.yaml")
			require.Equal(t, expected.Object, actual.Object)

			credentialsPath := filepath.Join(repositoryRoot, fixture.path, "capi/credentials.yaml")
			if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
				return
			}
			require.NoError(t, err)
			credentials, err := newCredentialsTemplate(readFixtureFile(t, fixture.path, "capi/credentials.yaml"))
			require.NoError(t, err)
			actualSecret, err := credentials.Render(data)
			require.NoError(t, err)
			actualSecret.Labels["heritage"] = "deckhouse"
			actualSecret.Labels["module"] = "node-manager"

			expectedSecret := readGoldenObject(t, fixture.name, "credentials.yaml")
			expectedData, _, err := unstructured.NestedStringMap(expectedSecret.Object, "data")
			require.NoError(t, err)
			actualData := make(map[string]string, len(actualSecret.Data))
			for key, value := range actualSecret.Data {
				actualData[key] = string(value)
			}
			require.Equal(t, expectedData, actualData)

			expectedSecret.Object["data"] = nil
			actualObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(actualSecret)
			require.NoError(t, err)
			actualObject["data"] = nil
			require.Equal(t, expectedSecret.Object, actualObject)
		})
	}
}

func TestOpenStackClusterTemplateNeedsEndpointWhenFloatingIPIsDisabled(t *testing.T) {
	registration := Registration{
		CAPIClusterName:       "openstack",
		CAPIClusterKind:       "OpenStackCluster",
		CAPIClusterAPIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
	}
	template, err := newClusterTemplate(
		readFixtureFile(t, "ee/modules/030-cloud-provider-openstack", "capi/cluster.yaml"),
		registration,
	)
	require.NoError(t, err)

	data := RenderData{
		Provider: map[string]any{
			"apiServerFloatingIP":  false,
			"internalNetworkNames": []any{"internal"},
		},
		Cluster: machinetemplate.ClusterFacts{Name: "openstack", Namespace: capiNamespace},
	}

	_, err = template.Render(data)
	require.ErrorContains(t, err, "controlPlane")

	data.Provider["apiServerFloatingIP"] = true
	object, err := template.Render(data)
	require.NoError(t, err)
	_, found, err := unstructured.NestedMap(object.Object, "spec", "controlPlaneEndpoint")
	require.NoError(t, err)
	require.False(t, found)
}

func readFixtureFile(t *testing.T, providerPath, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repositoryRoot, providerPath, name))
	require.NoError(t, err)
	return data
}

func readGoldenObject(t *testing.T, provider, name string) *unstructured.Unstructured {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "helm", provider, name))
	require.NoError(t, err)
	object, err := decodeExactlyOne(name, data)
	require.NoError(t, err)
	return object
}

func addLegacyModuleLabels(labels map[string]string, set func(map[string]string)) {
	if labels == nil {
		labels = map[string]string{}
	}
	labels["heritage"] = "deckhouse"
	labels["module"] = "node-manager"
	set(labels)
}
