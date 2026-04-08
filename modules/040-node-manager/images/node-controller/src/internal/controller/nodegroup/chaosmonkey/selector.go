/*
Copyright 2025 Flant JSC

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

package chaosmonkey

import (
	"context"
	"math/rand"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
)

const (
	chaosMonkeyVictimLabel = "node.deckhouse.io/chaos-monkey-victim"
	nodeGroupLabel         = "node.deckhouse.io/group"
	machineNamespace       = "d8-cloud-instance-manager"
)

// victimSelector picks a random Machine to delete for a given NodeGroup.
type victimSelector struct {
	client client.Client
}

// hasExistingVictim returns true if any Machine in the namespace is already marked as a chaos monkey victim.
func (s *victimSelector) hasExistingVictim(ctx context.Context) (bool, error) {
	machineList := &mcmv1alpha1.MachineList{}
	if err := s.client.List(ctx, machineList,
		client.InNamespace(machineNamespace),
		client.MatchingLabels{chaosMonkeyVictimLabel: ""},
	); err != nil {
		return false, err
	}
	return len(machineList.Items) > 0, nil
}

// selectVictim picks a random Machine belonging to a node in the given NodeGroup.
// Returns nil if no suitable victim is found.
func (s *victimSelector) selectVictim(ctx context.Context, rng *rand.Rand, ngName string) (*mcmv1alpha1.Machine, error) {
	// List nodes in the NodeGroup.
	nodeList := &corev1.NodeList{}
	if err := s.client.List(ctx, nodeList,
		client.MatchingLabels{nodeGroupLabel: ngName},
	); err != nil {
		return nil, err
	}
	if len(nodeList.Items) == 0 {
		return nil, nil
	}

	// Pick a random node.
	victimNode := nodeList.Items[rng.Intn(len(nodeList.Items))]

	// List all Machines and find the one matching the victim node.
	machineList := &mcmv1alpha1.MachineList{}
	if err := s.client.List(ctx, machineList,
		client.InNamespace(machineNamespace),
	); err != nil {
		return nil, err
	}

	for i := range machineList.Items {
		m := &machineList.Items[i]
		if m.Labels["node"] == victimNode.Name {
			return m, nil
		}
	}

	return nil, nil
}
