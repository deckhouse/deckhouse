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

package capi

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type providerClusterGoldenFixture struct {
	name       string
	contract   string
	cluster    string
	provider   map[string]any
	goldenFile string
}

func TestProviderClusterTemplatesMatchHelmGoldens(t *testing.T) {
	fixtures := []providerClusterGoldenFixture{
		{
			name:       "dvp",
			contract:   "../../../../../../../030-cloud-provider-dvp/capi/cluster.yaml",
			cluster:    "dvp",
			provider:   map[string]any{},
			goldenFile: "dvp.yaml",
		},
		{
			name:     "yandex",
			contract: "../../../../../../../030-cloud-provider-yandex/capi/cluster.yaml",
			cluster:  "yandex",
			provider: map[string]any{
				"folderID": "folder-id",
				"zoneToSubnetIdMap": map[string]any{
					"ru-central1-a": "subnet-a",
					"ru-central1-b": "subnet-b",
				},
			},
			goldenFile: "yandex.yaml",
		},
		{
			name:     "openstack",
			contract: "../../../../../../../../ee/modules/030-cloud-provider-openstack/capi/cluster.yaml",
			cluster:  "openstack",
			provider: map[string]any{
				"connection": map[string]any{
					"authURL":    "https://keystone.example/v3",
					"username":   "cloud-user",
					"password":   "cloud-password",
					"tenantName": "project-name",
					"domainName": "Default",
					"region":     "RegionOne",
					"caCert":     "test-ca",
				},
				"apiServerFloatingIP":  false,
				"internalNetworkNames": []any{"internal"},
				"externalNetworkNames": []any{"public"},
			},
			goldenFile: "openstack.yaml",
		},
		{
			name:     "vcd",
			contract: "../../../../../../../../ee/modules/030-cloud-provider-vcd/capi/cluster.yaml",
			cluster:  "test-vapp",
			provider: map[string]any{
				"server":            "https://vcd.example/",
				"organization":      "test-org",
				"virtualDataCenter": "test-ovdc",
				"mainNetwork":       "test-network",
				"username":          "user",
				"password":          "pass",
				"apiToken":          "token",
			},
			goldenFile: "vcd.yaml",
		},
		{
			name:       "dynamix",
			contract:   "../../../../../../../../ee/modules/030-cloud-provider-dynamix/capi/cluster.yaml",
			cluster:    "dynamix",
			provider:   map[string]any{},
			goldenFile: "dynamix.yaml",
		},
		{
			name:       "huaweicloud",
			contract:   "../../../../../../../../ee/modules/030-cloud-provider-huaweicloud/capi/cluster.yaml",
			cluster:    "huaweicloud",
			provider:   map[string]any{},
			goldenFile: "huaweicloud.yaml",
		},
		{
			name:       "zvirt",
			contract:   "../../../../../../../../ee/se-plus/modules/030-cloud-provider-zvirt/capi/cluster.yaml",
			cluster:    "zvirt",
			provider:   map[string]any{"clusterID": "cluster-id"},
			goldenFile: "zvirt.yaml",
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			data, err := os.ReadFile(fixture.contract)
			require.NoError(t, err)
			contract, err := parseClusterTemplateContract(data)
			require.NoError(t, err)

			objects, err := renderClusterTemplate(contract, clusterTemplateContext{
				Provider: fixture.provider,
				Cluster: clusterTemplateClusterContext{
					Name:          fixture.cluster,
					Namespace:     capiNamespace,
					PodSubnet:     "10.111.0.0/16",
					ServiceSubnet: "10.222.0.0/16",
					Domain:        "cluster.local",
					Prefix:        "test-prefix",
					MasterEndpoints: []map[string]interface{}{
						{"address": "192.0.2.10", "kubeApiPort": 6443},
					},
					MasterAddresses: []string{"192.0.2.10:6443"},
				},
			})
			require.NoError(t, err)
			normalizeClusterGoldenObjects(t, objects)

			golden, err := os.ReadFile(filepath.Join("testdata", "cluster", fixture.goldenFile))
			require.NoError(t, err)
			expected, err := decodeClusterTemplateObjects(golden)
			require.NoError(t, err)

			assert.Equal(t, expected, objects)
		})
	}
}

func normalizeClusterGoldenObjects(t *testing.T, objects []*unstructured.Unstructured) {
	t.Helper()
	for _, object := range objects {
		prepareClusterTemplateObject(object)
		annotations := object.GetAnnotations()
		delete(annotations, "helm.sh/resource-policy")
		if len(annotations) == 0 {
			object.SetAnnotations(nil)
		} else {
			object.SetAnnotations(annotations)
		}

		if object.GetAPIVersion() != "v1" || object.GetKind() != "Secret" {
			continue
		}
		data, found, err := unstructured.NestedStringMap(object.Object, "data")
		require.NoError(t, err)
		if !found {
			continue
		}
		decoded := make(map[string]interface{}, len(data))
		for key, value := range data {
			plain, err := base64.StdEncoding.DecodeString(value)
			require.NoError(t, err, "decode Secret data %s", key)
			decoded[key] = string(plain)
		}
		object.Object["data"] = decoded
	}
}

func TestOpenStackClusterTemplateEndpointModes(t *testing.T) {
	data, err := os.ReadFile("../../../../../../../../ee/modules/030-cloud-provider-openstack/capi/cluster.yaml")
	require.NoError(t, err)
	contract, err := parseClusterTemplateContract(data)
	require.NoError(t, err)

	baseProvider := map[string]any{
		"connection": map[string]any{
			"authURL":    "https://keystone.example/v3",
			"username":   "cloud-user",
			"password":   "cloud-password",
			"domainName": "Default",
			"region":     "RegionOne",
		},
		"internalNetworkNames": []any{"internal"},
	}

	t.Run("floating IP enabled by default", func(t *testing.T) {
		objects, err := renderClusterTemplate(contract, clusterTemplateContext{
			Provider: baseProvider,
			Cluster: clusterTemplateClusterContext{
				Name:            "openstack",
				Namespace:       capiNamespace,
				MasterAddresses: []string{"10.0.0.1:6443"},
			},
		})
		require.NoError(t, err)
		infra := objects[len(objects)-1]
		_, found, err := unstructured.NestedFieldNoCopy(infra.Object, "spec", "disableAPIServerFloatingIP")
		require.NoError(t, err)
		assert.False(t, found)
		_, found, err = unstructured.NestedFieldNoCopy(infra.Object, "spec", "controlPlaneEndpoint")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("master address fallback without floating IP", func(t *testing.T) {
		provider := make(map[string]any, len(baseProvider)+1)
		for key, value := range baseProvider {
			provider[key] = value
		}
		provider["apiServerFloatingIP"] = false

		objects, err := renderClusterTemplate(contract, clusterTemplateContext{
			Provider: provider,
			Cluster: clusterTemplateClusterContext{
				Name:            "openstack",
				Namespace:       capiNamespace,
				MasterAddresses: []string{"10.0.0.1:6443"},
			},
		})
		require.NoError(t, err)
		infra := objects[len(objects)-1]
		host, found, err := unstructured.NestedString(infra.Object, "spec", "controlPlaneEndpoint", "host")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "10.0.0.1", host)
		port, found, err := unstructured.NestedFloat64(infra.Object, "spec", "controlPlaneEndpoint", "port")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, float64(6443), port)
	})
}
