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

// rolloutBudget is how many more nodes of one group may be handed a new spec.
// A group is updated a few nodes at a time — the same guarantee bashible nodes
// get from the update-approval annotations, except that the desired state is
// written by this controller, so it simply withholds the change instead of
// asking the node to wait for permission.
type rolloutBudget struct {
	concurrency int
	// updating holds the nodes of the group that are mid-update: those that had
	// not applied their config when the group was read, plus the ones this pass
	// has handed a new spec to since.
	updating map[string]struct{}
}

// hasSlot answers whether a node may be handed a new spec now.
//
// A node already carrying an unapplied spec keeps its slot while the group is
// within its budget: that node is mid-update either way, and withholding the
// next change would strand it on a config nobody is going to finish. Only while
// the group is within budget, though — "already updating" is also what a node
// looks like when its agent is absent, too old to publish the condition, or
// broken, and passing those unconditionally opened the gate widest exactly when
// the group was least healthy: with as many silent nodes as the group allows
// updates, every one of them took every subsequent change, and they all
// disrupted together the moment their agents came back.
func (b *rolloutBudget) hasSlot(nodeName string) bool {
	if _, mid := b.updating[nodeName]; mid {
		return len(b.updating) <= b.concurrency
	}
	return len(b.updating) < b.concurrency
}

// spend records that a node was handed a new spec, so the nodes rendered after
// it in the same pass are judged against a group where it is updating.
func (b *rolloutBudget) spend(nodeName string) {
	b.updating[nodeName] = struct{}{}
}

// rolloutBudget reads a group's remaining budget, once per group per pass.
//
// The listing goes straight to the API server, for the reason findApproval does:
// this is a decision followed by a write. A cached list does not yet carry the
// config the previous node was just handed, so every node in turn would be
// judged against a group where nothing is updating, and the whole group would
// take the change at once — the one thing this gate exists to prevent.
//
// Reading it once per pass rather than once per node is what keeps that
// affordable: a NodeGroup edit drifts every node of the group at the same time,
// and a NodeConfig is several kilobytes, so listing per node made a group of N
// cost N listings of N of them on every pass of a rollout. Counting this pass's
// own writes (see spend) is what makes the single read as accurate as N reads.
func (r *Reconciler) rolloutBudget(ctx context.Context, ng *v1.NodeGroup, p *pass) (*rolloutBudget, error) {
	if budget, ok := p.rollouts[ng.Name]; ok {
		return budget, nil
	}

	configs := &internalv1alpha1.NodeConfigList{}
	if err := r.sources.Reader.List(ctx, configs, client.MatchingLabels{nodeGroupNameLabel: ng.Name}); err != nil {
		return nil, fmt.Errorf("list NodeConfigs of %s: %w", ng.Name, err)
	}

	budget := &rolloutBudget{
		concurrency: ua.CalculateConcurrency(maxConcurrent(ng), len(configs.Items)),
		updating:    make(map[string]struct{}),
	}
	for i := range configs.Items {
		if !applied(&configs.Items[i]) {
			budget.updating[configs.Items[i].Name] = struct{}{}
		}
	}
	// Nodes this pass gave a first config to are updating too, whether or not
	// the listing above was taken before or after they were created.
	for nodeName := range p.created[ng.Name] {
		budget.updating[nodeName] = struct{}{}
	}
	p.rollouts[ng.Name] = budget
	return budget, nil
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
