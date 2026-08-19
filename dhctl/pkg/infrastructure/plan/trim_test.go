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

package plan_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
)

func TestPlan_Trim(t *testing.T) {
	tests := map[string]struct {
		plan     plan.Plan
		expected plan.Plan
	}{
		"trims resource_changes to identity fields and prior_state to name/type/tags.Name": {
			plan: plan.Plan{
				"format_version":    "1.0",
				"terraform_version": "1.6.0",
				"variables":         map[string]any{"foo": "bar"},
				"planned_values":    map[string]any{"root_module": map[string]any{}},
				"output_changes":    map[string]any{},
				"configuration":     map[string]any{"root_module": map[string]any{}},
				"resource_changes": []any{
					map[string]any{
						"address":       "yandex_compute_instance.master",
						"mode":          "managed",
						"type":          "yandex_compute_instance",
						"name":          "master",
						"provider_name": "registry.opentofu.org/yandex-cloud/yandex",
						"change": map[string]any{
							"actions": []any{"delete", "create"},
							"before": map[string]any{
								"name":      "master",
								"zone":      "ru-central1-a",
								"boot_disk": []any{map[string]any{"disk_id": "abc"}},
							},
							"after": map[string]any{
								"name": "master",
								"zone": "ru-central1-a",
							},
							"after_unknown": map[string]any{"id": true},
						},
					},
				},
				"prior_state": map[string]any{
					"format_version":    "1.0",
					"terraform_version": "1.6.0",
					"values": map[string]any{
						"outputs": map[string]any{
							"master_ip_address_for_ssh": map[string]any{"value": "1.2.3.4"},
						},
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{
									"name": "master",
									"type": "yandex_compute_instance",
									"values": map[string]any{
										"tags":        map[string]any{"Name": "master-0"},
										"boot_disk":   []any{map[string]any{"disk_id": "abc"}},
										"description": "a large unrelated blob of attributes",
									},
								},
							},
						},
					},
				},
			},
			expected: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "yandex_compute_instance",
						"name": "master",
						"change": map[string]any{
							"actions": []any{"delete", "create"},
							"before":  map[string]any{"name": "master"},
							"after":   map[string]any{"name": "master"},
						},
					},
				},
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{
									"name":   "master",
									"type":   "yandex_compute_instance",
									"values": map[string]any{"tags": map[string]any{"Name": "master-0"}},
								},
							},
						},
					},
				},
			},
		},
		"before is omitted for a create-only change, not turned into an empty object": {
			plan: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "yandex_compute_instance",
						"name": "worker-1",
						"change": map[string]any{
							"actions": []any{"create"},
						},
					},
				},
			},
			expected: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type":   "yandex_compute_instance",
						"name":   "worker-1",
						"change": map[string]any{"actions": []any{"create"}},
					},
				},
			},
		},
		"after is omitted for a delete-only change, not turned into an empty object": {
			plan: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "yandex_compute_instance",
						"name": "worker-1",
						"change": map[string]any{
							"actions": []any{"delete"},
							"before":  map[string]any{"name": "worker-1-name"},
						},
					},
				},
			},
			expected: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "yandex_compute_instance",
						"name": "worker-1",
						"change": map[string]any{
							"actions": []any{"delete"},
							"before":  map[string]any{"name": "worker-1-name"},
						},
					},
				},
			},
		},
		"before without a name attribute (e.g. aws_instance) omits the name key rather than an empty string": {
			// aws_instance names instances via a tag, not a "name" attribute -
			// the key must be absent here, not present-and-empty.
			plan: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "aws_instance",
						"name": "master",
						"change": map[string]any{
							"actions": []any{"delete", "create"},
							"before": map[string]any{
								"tags":          map[string]any{"Name": "master-0"},
								"instance_type": "m5.large",
							},
						},
					},
				},
			},
			expected: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "aws_instance",
						"name": "master",
						"change": map[string]any{
							"actions": []any{"delete", "create"},
							"before":  map[string]any{},
						},
					},
				},
			},
		},
		"before name falls back through manifest.metadata.name and metadata[].name shapes": {
			plan: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "kubernetes_manifest",
						"name": "some_manifest",
						"change": map[string]any{
							"actions": []any{"delete"},
							"before": map[string]any{
								"manifest": map[string]any{
									"kind":     "Deployment",
									"metadata": map[string]any{"name": "my-deployment", "namespace": "default"},
									"spec":     map[string]any{"replicas": 3},
								},
								"metadata": []any{
									map[string]any{"name": "meta-entry", "uid": "abc-123"},
								},
							},
						},
					},
				},
			},
			expected: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "kubernetes_manifest",
						"name": "some_manifest",
						"change": map[string]any{
							"actions": []any{"delete"},
							"before": map[string]any{
								"manifest": map[string]any{
									"kind":     "Deployment",
									"metadata": map[string]any{"name": "my-deployment"},
								},
								"metadata": []any{
									map[string]any{"name": "meta-entry"},
								},
							},
						},
					},
				},
			},
		},
		"one resource_change with a malformed before shape does not drop the rest of the array": {
			plan: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "weird_provider_resource",
						"name": "quirky",
						"change": map[string]any{
							"actions": []any{"update"},
							"before": map[string]any{
								"name":     "quirky-actual-name",
								"metadata": map[string]any{"unexpected": "object-instead-of-array"},
							},
						},
					},
					map[string]any{
						"type": "yandex_compute_instance",
						"name": "worker-0",
						"change": map[string]any{
							"actions": []any{"update"},
							"before":  map[string]any{"name": "worker-0-name"},
						},
					},
				},
			},
			expected: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "weird_provider_resource",
						"name": "quirky",
						"change": map[string]any{
							"actions": []any{"update"},
							"before":  map[string]any{"name": "quirky-actual-name"},
						},
					},
					map[string]any{
						"type": "yandex_compute_instance",
						"name": "worker-0",
						"change": map[string]any{
							"actions": []any{"update"},
							"before":  map[string]any{"name": "worker-0-name"},
						},
					},
				},
			},
		},
		"missing resource_changes is omitted, prior_state is still trimmed": {
			plan: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{"name": "master", "type": "yandex_compute_instance", "values": map[string]any{}},
							},
						},
					},
				},
			},
			expected: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{"name": "master", "type": "yandex_compute_instance", "values": map[string]any{"tags": map[string]any{}}},
							},
						},
					},
				},
			},
		},
		"resource without tags keeps an empty values object": {
			plan: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{
									"name":   "worker",
									"type":   "aws_ebs_volume",
									"values": map[string]any{"size": 100},
								},
							},
						},
					},
				},
			},
			expected: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{
									"name":   "worker",
									"type":   "aws_ebs_volume",
									"values": map[string]any{"tags": map[string]any{}},
								},
							},
						},
					},
				},
			},
		},
		"resource with tags but no Name keeps an empty tags object": {
			plan: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{
									"name":   "master",
									"type":   "aws_instance",
									"values": map[string]any{"tags": map[string]any{"Environment": "prod"}},
								},
							},
						},
					},
				},
			},
			expected: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{
									"name":   "master",
									"type":   "aws_instance",
									"values": map[string]any{"tags": map[string]any{}},
								},
							},
						},
					},
				},
			},
		},
		"resource with tags explicitly null keeps an empty tags object": {
			plan: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{
									"name":   "worker",
									"type":   "aws_ebs_volume",
									"values": map[string]any{"tags": nil},
								},
							},
						},
					},
				},
			},
			expected: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{
									"name":   "worker",
									"type":   "aws_ebs_volume",
									"values": map[string]any{"tags": map[string]any{}},
								},
							},
						},
					},
				},
			},
		},
		"missing prior_state is omitted; resource_changes is still trimmed": {
			plan: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "yandex_compute_instance",
						"name": "worker-0",
						"change": map[string]any{
							"actions": []any{"no-op"},
							"before":  map[string]any{"name": "worker-0-name"},
							"after":   map[string]any{"name": "worker-0-name"},
						},
					},
				},
			},
			expected: plan.Plan{
				"resource_changes": []any{
					map[string]any{
						"type": "yandex_compute_instance",
						"name": "worker-0",
						"change": map[string]any{
							"actions": []any{"no-op"},
							"before":  map[string]any{"name": "worker-0-name"},
							"after":   map[string]any{"name": "worker-0-name"},
						},
					},
				},
			},
		},
		"malformed prior_state shape yields an empty scaffold, not an error": {
			plan: plan.Plan{
				"prior_state": "not-an-object",
			},
			expected: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": nil,
						},
					},
				},
			},
		},
		"one resource with a malformed field does not drop the rest of the array": {
			plan: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{
									"name": "master",
									"type": "google_compute_instance",
									"values": map[string]any{
										"tags": []any{"master", "kube"},
									},
								},
								map[string]any{
									"name": "worker-0",
									"type": "yandex_compute_instance",
									"values": map[string]any{
										"tags": map[string]any{"Name": "worker-0-name"},
									},
								},
							},
						},
					},
				},
			},
			expected: plan.Plan{
				"prior_state": map[string]any{
					"values": map[string]any{
						"root_module": map[string]any{
							"resources": []any{
								map[string]any{
									"name":   "master",
									"type":   "google_compute_instance",
									"values": map[string]any{"tags": map[string]any{}},
								},
								map[string]any{
									"name":   "worker-0",
									"type":   "yandex_compute_instance",
									"values": map[string]any{"tags": map[string]any{"Name": "worker-0-name"}},
								},
							},
						},
					},
				},
			},
		},
		"empty plan returns empty": {
			plan:     plan.Plan{},
			expected: plan.Plan{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual, err := json.Marshal(tt.plan.Trim())
			require.NoError(t, err)

			expected, err := json.Marshal(tt.expected)
			require.NoError(t, err)

			assert.JSONEq(t, string(expected), string(actual))
		})
	}
}
