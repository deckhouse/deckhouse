/*
Copyright 2024 Flant JSC

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

package scope

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
)

func TestMachineScopeLabelSelector(t *testing.T) {
	tests := []struct {
		machineScope MachineScope
		err          bool
	}{
		{
			machineScope: MachineScope{
				StaticMachine: &infrav1.StaticMachine{},
			},
		},
		{
			machineScope: MachineScope{
				StaticMachine: &infrav1.StaticMachine{
					Spec: infrav1.StaticMachineSpec{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"node-group": "worker",
							},
						},
					},
				},
			},
		},
		{
			machineScope: MachineScope{
				StaticMachine: &infrav1.StaticMachine{
					Spec: infrav1.StaticMachineSpec{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"node.deckhouse.io/allow-bootstrap": "true",
							},
						},
					},
				},
			},
			err: true,
		},
		{
			machineScope: MachineScope{
				StaticMachine: &infrav1.StaticMachine{
					Spec: infrav1.StaticMachineSpec{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "node.deckhouse.io/allow-bootstrap",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"false"},
								},
							},
						},
					},
				},
			},
			err: true,
		},
	}

	allowBootstrapRequirement, err := labels.NewRequirement("node.deckhouse.io/allow-bootstrap", selection.NotIn, []string{"false"})
	require.NoError(t, err)

	for _, test := range tests {
		labelSelector, err := test.machineScope.LabelSelector()
		if test.err {
			require.Error(t, err)
			continue
		}

		require.NoError(t, err)

		requirements, _ := labelSelector.Requirements()

		require.Equal(t, *allowBootstrapRequirement, requirements[len(requirements)-1])
	}
}
