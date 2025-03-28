// Copyright 2024 Flant JSC
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

package check

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestStatistics_Format(t *testing.T) {
	t.Parallel()

	var statistics Statistics
	err := yaml.Unmarshal([]byte(statisticsYAML), &statistics)
	require.NoError(t, err)

	assert.EqualValues(t, expectedStatistics, statistics)

	formattedStatistics, err := statistics.Format("yaml")
	require.NoError(t, err)

	assert.EqualValues(t, expectedStatistics, statistics)
	assert.Equal(t, statisticsYAMLPrintable, string(formattedStatistics))
}

const (
	statisticsYAML = `cluster:
  status: ok
  destructive_changes:
    resourced_recreated:
    - current_value:
        zone: ru-central1-a
      next_value:
        zone: ru-central1-b
node_templates:
- name: master
  status: ok
- name: khm
  status: ok
nodes:
- destructive_changes:
    resourced_recreated:
    - current_value:
        zone: ru-central1-a
      next_value:
        zone: ru-central1-b
  group: master
  name: akul-master-0
  status: destructively_changed
terraform_plan:
- configuration: {}
  format_version: "0.1"`

	statisticsYAMLPrintable = `cluster:
  status: ok
node_templates:
- name: master
  status: ok
- name: khm
  status: ok
nodes:
- group: master
  name: akul-master-0
  status: destructively_changed
`
)

var expectedStatistics = Statistics{
	Cluster: ClusterCheckResult{
		Status: "ok",
		DestructiveChanges: &infrastructure.BaseInfrastructureDestructiveChanges{
			PlanDestructiveChanges: infrastructure.PlanDestructiveChanges{
				ResourcesRecreated: []infrastructure.ValueChange{
					{
						CurrentValue: map[string]any{"zone": "ru-central1-a"},
						NextValue:    map[string]any{"zone": "ru-central1-b"},
					},
				},
			},
		},
	},
	NodeTemplates: []NodeGroupCheckResult{
		{
			Name:   "master",
			Status: "ok",
		},
		{
			Name:   "khm",
			Status: "ok",
		},
	},
	Node: []NodeCheckResult{
		{
			Group:  "master",
			Name:   "akul-master-0",
			Status: "destructively_changed",
			DestructiveChanges: &infrastructure.PlanDestructiveChanges{
				ResourcesRecreated: []infrastructure.ValueChange{
					{
						CurrentValue: map[string]any{"zone": "ru-central1-a"},
						NextValue:    map[string]any{"zone": "ru-central1-b"},
					},
				},
			},
		},
	},
	InfrastructurePlan: []infrastructure.Plan{
		{
			"configuration":  map[string]any{},
			"format_version": "0.1",
		},
	},
}

func mockTerraformVersionProvider(ctx context.Context, metaConfig *config.MetaConfig) ([]byte, error) {
	return []byte(`{
		"terraform_version": "0.14.8",
		"terraform_revision": "",
		"provider_selections": {},
		"terraform_outdated": true
	}`), nil
}

func TestCheckTerraformVersion(t *testing.T) {
	ctx := context.Background()
	kubeCl := client.NewFakeKubernetesClient()

	// Подготовка фейкового состояния кластера с версией Terraform в стейте "1.9.0".
	fakeStateYAML := `{
		"version": 4,
		"terraform_version": "1.9.0",
		"serial": 13,
		"lineage": "6e5d9457-50da-ea2c-4e78-a800a2f57a5c",
		"outputs": {},
		"resources": [
			{
				"module": "module.vpc_components",
				"mode": "managed",
				"type": "yandex_vpc_gateway",
				"name": "kube",
				"provider": "provider[\"registry.opentofu.org/yandex-cloud/yandex\"]",
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

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-cluster-terraform-state",
			Namespace: "d8-system",
		},
		Data: map[string][]byte{
			"cluster-tf-state.json": []byte(fakeStateYAML),
		},
	}
	_, err := kubeCl.CoreV1().Secrets("d8-system").Create(ctx, secret, metav1.CreateOptions{})
	require.NoError(t, err)

	metaConfig := &config.MetaConfig{}
	result, err := getCurrentTerraformVersion(ctx, metaConfig, mockTerraformVersionProvider)

	require.NoError(t, err)
	require.Exactly(t, "0.14.8", result)

	result2, err := getTerraformVersionFromState(ctx, kubeCl)
	require.NoError(t, err)
	require.Exactly(t, "1.9.0", result2.TerraformVersion)
}
