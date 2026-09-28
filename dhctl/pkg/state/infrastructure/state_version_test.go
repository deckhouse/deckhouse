// Copyright 2025 Flant JSC
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

package infrastructure

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestCheckTerraformVersion(t *testing.T) {
	fakeStateYAML := `{
		"version": 4,
		"terraform_version": "0.14.8",
		"serial": 13,
		"lineage": "6e5d9457-50da-ea2c-4e78-a800a2f57a5c",
		"outputs": {},
		"resources": [
			{
				"module": "module.vpc_components",
				"mode": "managed",
				"type": "yandex_vpc_gateway",
				"name": "kube",
				"provider": "provider[\"registry.terraform.org/yandex-cloud/yandex\"]",
				"instances": [
					{
						"index_key": 0,
						"schema_version": 0,
						"attributes": {
							"created_at": "2025-03-27T12:24:04Z",
							"description": "",
							"folder_id": "2345xcf34cf5345f",
							"id": "x34f34cf3c4",
							"labels": {},
							"name": "super-tofu",
							"shared_egress_gateway": [
								{}
							],
							"timeouts": null
						},
						"sensitive_attributes": [],
						"private": "wf34rt3c4f3"
					}
				]
			}
		],
		"check_results": []
	}`

	result, err := IsTerraformState([]byte(fakeStateYAML))
	require.NoError(t, err)
	require.True(t, result)

	err = CheckCanIConvergeTerraformStateWhenWeUseTofu([]byte(fakeStateYAML))
	require.Error(t, err)
}

func TestHasTerraformStateInCluster(t *testing.T) {
	const (
		tofuState      = `{"version":4,"terraform_version":"1.9.4"}`
		terraformState = `{"version":4,"terraform_version":"0.14.8"}`
	)

	tests := []struct {
		name         string
		clusterState string
		masterStates map[string]string
		want         bool
	}{
		{
			name:         "all states are opentofu",
			clusterState: tofuState,
			masterStates: map[string]string{"master-0": tofuState, "master-1": tofuState},
			want:         false,
		},
		{
			name:         "one master state is terraform",
			clusterState: tofuState,
			masterStates: map[string]string{"master-0": tofuState, "master-1": terraformState},
			want:         true,
		},
		{
			name:         "cluster state is terraform",
			clusterState: terraformState,
			want:         true,
		},
		{
			name: "no states",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeCl := client.NewFakeKubernetesClient()
			secrets := kubeCl.CoreV1().Secrets(global.D8SystemNamespace)
			if tt.clusterState != "" {
				_, err := secrets.Create(t.Context(), manifests.SecretWithInfrastructureState([]byte(tt.clusterState)), metav1.CreateOptions{})
				require.NoError(t, err)
			}
			for node, st := range tt.masterStates {
				_, err := secrets.Create(t.Context(), manifests.SecretWithNodeInfrastructureState(node, "master", []byte(st), nil), metav1.CreateOptions{})
				require.NoError(t, err)
			}

			got, err := HasTerraformStateInCluster(t.Context(), kubeCl)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
