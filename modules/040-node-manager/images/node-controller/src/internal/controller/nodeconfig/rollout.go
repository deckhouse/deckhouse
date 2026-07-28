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

package nodeconfig

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
)

// rolloutSlot answers whether this node may be handed a new spec now. A group
// is updated a few nodes at a time — the same guarantee bashible nodes get from
// the update-approval annotations, except that the desired state is written by
// this controller, so it simply withholds the change instead of asking the node
// to wait for permission.
//
// A node already carrying an unapplied spec keeps its slot: it is mid-update
// either way, and withholding the next change would strand it on a config no
// one is going to finish.
func (r *Reconciler) rolloutSlot(ctx context.Context, ng *v1.NodeGroup, nodeName string) (bool, error) {
	configs := &internalv1alpha1.NodeConfigList{}
	if err := r.Client.List(ctx, configs, client.MatchingLabels{nodeGroupNameLabel: ng.Name}); err != nil {
		return false, fmt.Errorf("list NodeConfigs of %s: %w", ng.Name, err)
	}

	updating := 0
	for i := range configs.Items {
		if !applied(&configs.Items[i]) {
			if configs.Items[i].Name == nodeName {
				return true, nil
			}
			updating++
		}
	}

	return updating < ua.CalculateConcurrency(maxConcurrent(ng), len(configs.Items)), nil
}

// applied reports whether the node has finished reconciling the spec it was
// given. A node that has not reported back yet counts as still updating, so a
// silent agent holds the rollout rather than letting it run ahead.
func applied(nc *internalv1alpha1.NodeConfig) bool {
	// A node waiting for permission to interrupt itself has not applied
	// anything, whatever else its status says. Counting it as done would free
	// the slot and walk the change through the whole group while every node
	// sits waiting.
	if disruptionRequested(nc) {
		return false
	}
	// AppliedGeneration, not ObservedGeneration: a held node has *observed* the
	// current generation (observedGeneration == generation) but is still running
	// the previous one. What frees a rollout slot is the node actually running
	// the published config, which is what AppliedGeneration reports.
	if nc.Status.AppliedGeneration != nc.Generation {
		return false
	}
	// ConfigurationApplied, not phase == Ready. The two answer different
	// questions and only one of them is this gate's: whether the node is running
	// what it was published. A node that rolled the spec back, quarantined it or
	// applied half of it says so here and rightly holds the rollout — that is the
	// safety this gate exists for. But phase is Degraded for ANY unhealthy
	// subsystem, including one that has nothing to do with the spec being rolled
	// out and that the rollout cannot fix: an auxiliary unit of an unrelated
	// sysext failing froze the whole group's rollout for as long as it stayed
	// broken, with nothing saying which node was holding it.
	cond := meta.FindStatusCondition(nc.Status.Conditions, configurationAppliedCondition)
	return cond != nil && cond.Status == metav1.ConditionTrue && cond.ObservedGeneration == nc.Generation
}

func maxConcurrent(ng *v1.NodeGroup) *intstr.IntOrString {
	if ng.Spec.Update == nil {
		return nil
	}
	return ng.Spec.Update.MaxConcurrent
}
