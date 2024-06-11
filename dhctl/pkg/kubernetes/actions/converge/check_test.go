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

package converge_test

import (
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
)

func TestStatistics_Format(t *testing.T) {
	t.Parallel()

	var statistics converge.Statistics
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

var expectedStatistics = converge.Statistics{
	Cluster: converge.ClusterCheckResult{
		Status: "ok",
		DestructiveChanges: &terraform.BaseInfrastructureDestructiveChanges{
			PlanDestructiveChanges: terraform.PlanDestructiveChanges{
				ResourcesRecreated: []terraform.ValueChange{
					{
						CurrentValue: map[string]any{"zone": "ru-central1-a"},
						NextValue:    map[string]any{"zone": "ru-central1-b"},
					},
				},
			},
		},
	},
	NodeTemplates: []converge.NodeGroupCheckResult{
		{
			Name:   "master",
			Status: "ok",
		},
		{
			Name:   "khm",
			Status: "ok",
		},
	},
	Node: []converge.NodeCheckResult{
		{
			Group:  "master",
			Name:   "akul-master-0",
			Status: "destructively_changed",
			DestructiveChanges: &terraform.PlanDestructiveChanges{
				ResourcesRecreated: []terraform.ValueChange{
					{
						CurrentValue: map[string]any{"zone": "ru-central1-a"},
						NextValue:    map[string]any{"zone": "ru-central1-b"},
					},
				},
			},
		},
	},
	TerraformPlan: []terraform.TerraformPlan{
		{
			"configuration":  map[string]any{},
			"format_version": "0.1",
		},
	},
}
