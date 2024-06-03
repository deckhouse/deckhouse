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

package converge

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/stretchr/testify/require"
)

func TestDestructiveChangeID(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		statistics *Statistics
		expected   string
	}{
		"nil statistics": {
			statistics: nil,
			expected:   ``,
		},
		"empty statistics": {
			statistics: &Statistics{},
			expected:   ``,
		},
		"empty destruction changes": {
			statistics: &Statistics{
				Node: []NodeCheckResult{
					{
						Group:  "master",
						Name:   "kube-master-0",
						Status: OKStatus,
					},
				},
				Cluster: ClusterCheckResult{
					Status: OKStatus,
				},
			},
			expected: ``,
		},
		"deleted node destruction change": {
			statistics: &Statistics{
				Node: []NodeCheckResult{
					{
						Group:  "master",
						Name:   "kube-master-0",
						Status: DestructiveStatus,
						DestructiveChanges: &terraform.PlanDestructiveChanges{
							ResourcesDeleted: []terraform.ValueChange{
								{
									CurrentValue: map[string]any{
										"type": "some_type",
										"name": "some_name",
										"key":  "value",
									},
								},
							},
						},
					},
				},
				Cluster: ClusterCheckResult{
					Status: OKStatus,
				},
			},
			expected: `{"node:kube-master-0:resource_deleted:0:current":{"name":"some_name","type":"some_type"}}`,
		},
		"recreated node destruction change": {
			statistics: &Statistics{
				Node: []NodeCheckResult{
					{
						Group:  "master",
						Name:   "kube-master-0",
						Status: DestructiveStatus,
						DestructiveChanges: &terraform.PlanDestructiveChanges{
							ResourcesRecreated: []terraform.ValueChange{
								{
									NextValue: map[string]any{
										"type": "some_type",
										"name": "some_name",
										"key":  "value",
									},
								},
							},
						},
					},
				},
				Cluster: ClusterCheckResult{
					Status: OKStatus,
				},
			},
			expected: `{"node:kube-master-0:resource_recreated:0:next":{"key":"value","name":"some_name","type":"some_type"}}`,
		},
		"cluster output_broken_reason": {
			statistics: &Statistics{
				Node: []NodeCheckResult{},
				Cluster: ClusterCheckResult{
					Status: DestructiveStatus,
					DestructiveChanges: &terraform.BaseInfrastructureDestructiveChanges{
						OutputBrokenReason: "some_reason",
					},
				},
			},
			expected: `{"cluster:output_broken_reason":"some_reason"}`,
		},
		"cluster output_zones_changed": {
			statistics: &Statistics{
				Node: []NodeCheckResult{},
				Cluster: ClusterCheckResult{
					Status: DestructiveStatus,
					DestructiveChanges: &terraform.BaseInfrastructureDestructiveChanges{
						OutputZonesChanged: terraform.ValueChange{
							NextValue: map[string]any{
								"type": "some_type",
								"name": "some_name",
							},
						},
					},
				},
			},
			expected: `{"cluster:output_zones_changed:next":{"name":"some_name","type":"some_type"}}`,
		},
		"deleted cluster destruction change": {
			statistics: &Statistics{
				Node: []NodeCheckResult{},
				Cluster: ClusterCheckResult{
					Status: DestructiveStatus,
					DestructiveChanges: &terraform.BaseInfrastructureDestructiveChanges{
						PlanDestructiveChanges: terraform.PlanDestructiveChanges{
							ResourcesDeleted: []terraform.ValueChange{
								{
									CurrentValue: map[string]any{
										"type": "some_type",
										"name": "some_name",
									},
								},
							},
						},
					},
				},
			},
			expected: `{"cluster:resource_deleted:0:current":{"name":"some_name","type":"some_type"}}`,
		},
		"recreated cluster destruction change": {
			statistics: &Statistics{
				Node: []NodeCheckResult{},
				Cluster: ClusterCheckResult{
					Status: DestructiveStatus,
					DestructiveChanges: &terraform.BaseInfrastructureDestructiveChanges{
						PlanDestructiveChanges: terraform.PlanDestructiveChanges{
							ResourcesRecreated: []terraform.ValueChange{
								{
									NextValue: map[string]any{
										"type": "some_type",
										"name": "some_name",
										"key":  "value",
									},
								},
							},
						},
					},
				}},
			expected: `{"cluster:resource_recreated:0:next":{"key":"value","name":"some_name","type":"some_type"}}`,
		},
		"multiple changes": {
			statistics: &Statistics{
				Node: []NodeCheckResult{
					{
						Group:  "master",
						Name:   "kube-master-0",
						Status: DestructiveStatus,
						DestructiveChanges: &terraform.PlanDestructiveChanges{
							ResourcesDeleted: []terraform.ValueChange{
								{
									CurrentValue: map[string]any{
										"type": "some_type",
										"name": "some_name1",
										"key":  "value",
									},
								},
								{
									CurrentValue: map[string]any{
										"type": "some_type",
										"name": "some_name2",
										"key":  "value",
									},
								},
							},
							ResourcesRecreated: []terraform.ValueChange{
								{
									NextValue: map[string]any{
										"type": "some_type",
										"name": "some_name1",
										"key":  "value",
									},
								},
								{
									NextValue: map[string]any{
										"type": "some_type",
										"name": "some_name2",
										"key":  "value",
									},
								},
							},
						},
					},
				},
				Cluster: ClusterCheckResult{
					Status: DestructiveStatus,
					DestructiveChanges: &terraform.BaseInfrastructureDestructiveChanges{
						OutputZonesChanged: terraform.ValueChange{
							NextValue: map[string]any{
								"type": "some_type",
								"name": "some_name",
							},
						},
						PlanDestructiveChanges: terraform.PlanDestructiveChanges{
							ResourcesDeleted: []terraform.ValueChange{
								{
									CurrentValue: map[string]any{
										"type": "some_type",
										"name": "some_name1",
									},
								},
								{
									CurrentValue: map[string]any{
										"type": "some_type",
										"name": "some_name2",
									},
								},
							},
							ResourcesRecreated: []terraform.ValueChange{
								{
									NextValue: map[string]any{
										"type": "some_type",
										"name": "some_name1",
										"key":  "value",
									},
								},
								{
									NextValue: map[string]any{
										"type": "some_type",
										"name": "some_name2",
										"key":  "value",
									},
								},
							},
						},
					},
				},
			},
			expected: `{"cluster:output_zones_changed:next":{"name":"some_name","type":"some_type"},"cluster:resource_deleted:0:current":{"name":"some_name1","type":"some_type"},"cluster:resource_deleted:1:current":{"name":"some_name2","type":"some_type"},"cluster:resource_recreated:0:next":{"key":"value","name":"some_name1","type":"some_type"},"cluster:resource_recreated:1:next":{"key":"value","name":"some_name2","type":"some_type"},"node:kube-master-0:resource_deleted:0:current":{"name":"some_name1","type":"some_type"},"node:kube-master-0:resource_deleted:1:current":{"name":"some_name2","type":"some_type"},"node:kube-master-0:resource_recreated:0:next":{"key":"value","name":"some_name1","type":"some_type"},"node:kube-master-0:resource_recreated:1:next":{"key":"value","name":"some_name2","type":"some_type"}}`,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			id, err := destructiveChangeID(tt.statistics)
			require.NoError(t, err)

			require.Equal(t, tt.expected, string(id))

			idSha, err := DestructiveChangeID(tt.statistics)
			require.NoError(t, err)

			h := sha256.New()
			h.Write([]byte(tt.expected))
			expectedSha := fmt.Sprintf("%x", h.Sum(nil))
			require.Equal(t, expectedSha, idSha)
		})
	}
}
