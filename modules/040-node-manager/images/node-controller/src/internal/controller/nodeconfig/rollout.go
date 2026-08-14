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
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
)

// nodeConfigRolloutInputsChanged reports whether a NodeConfig update touched
// anything a pass reads: Generation stands in for the spec (status subresource),
// the rest is what the rollout gate reads or what decides ownership.
func nodeConfigRolloutInputsChanged(before, after client.Object) bool {
	oldConfig, okOld := before.(*internalv1alpha1.NodeConfig)
	newConfig, okNew := after.(*internalv1alpha1.NodeConfig)
	if !okOld || !okNew {
		return true
	}
	if oldConfig.Generation != newConfig.Generation ||
		oldConfig.Status.AppliedGeneration != newConfig.Status.AppliedGeneration {
		return true
	}
	if (oldConfig.DeletionTimestamp == nil) != (newConfig.DeletionTimestamp == nil) {
		return true
	}
	if !maps.Equal(oldConfig.Labels, newConfig.Labels) ||
		!slices.EqualFunc(oldConfig.OwnerReferences, newConfig.OwnerReferences, ownerRefEqual) {
		return true
	}
	// readNEROutcomes counts the extension states off this list, and a node can
	// change one without touching a condition or the applied generation: the
	// event has to come from the same object the count is read from.
	if !slices.EqualFunc(oldConfig.Status.Extensions, newConfig.Status.Extensions, extensionStatusEqual) {
		return true
	}
	return !conditionEqual(oldConfig.Status.Conditions, newConfig.Status.Conditions, configurationAppliedCondition) ||
		!conditionEqual(oldConfig.Status.Conditions, newConfig.Status.Conditions, disruptionRequiredCondition)
}

func extensionStatusEqual(a, b internalv1alpha1.ExtensionStatus) bool {
	return a.Name == b.Name && a.State == b.State && a.Message == b.Message
}

func ownerRefEqual(a, b metav1.OwnerReference) bool {
	return a.UID == b.UID && a.Name == b.Name && a.Kind == b.Kind
}

// conditionEqual compares one condition across two status lists by the two
// fields the rollout gate tests it on.
func conditionEqual(before, after []metav1.Condition, name string) bool {
	oldCondition := meta.FindStatusCondition(before, name)
	newCondition := meta.FindStatusCondition(after, name)
	if oldCondition == nil || newCondition == nil {
		return oldCondition == newCondition
	}
	return oldCondition.Status == newCondition.Status &&
		oldCondition.ObservedGeneration == newCondition.ObservedGeneration
}

// rolloutBudget is how many more nodes of one group may be handed a new spec.
// Like bashible's update-approval, it counts grants and not health: a node
// broken on a spec nobody publishes any more is not mid-rollout.
type rolloutBudget struct {
	concurrency int
	// updating holds the nodes of the group that are mid-update: those carrying
	// the published spec unproven when the group was read, plus the ones this
	// pass has handed a new spec to since.
	updating map[string]struct{}
}

// hasSlot answers whether a node may be handed a new spec now. A mid-update
// node keeps its slot, but only while the group is within budget: silent nodes
// look "already updating" too, and grandfathering them opened the gate widest.
func (b *rolloutBudget) hasSlot(nodeName string) bool {
	if _, mid := b.updating[nodeName]; mid {
		return len(b.updating) <= b.concurrency
	}
	return len(b.updating) < b.concurrency
}

// holders names the nodes occupying the slots, sorted, for the one log line an
// operator reads when a rollout is not moving.
func (b *rolloutBudget) holders() []string {
	return slices.Sorted(maps.Keys(b.updating))
}

// spend records that a node was handed a new spec, so the nodes rendered after
// it in the same pass are judged against a group where it is updating.
func (b *rolloutBudget) spend(nodeName string) {
	b.updating[nodeName] = struct{}{}
}

// rolloutBudget reads a group's remaining budget, once per group per pass. The
// listing goes straight to the API server — a cached list would judge every node
// against "nothing updating". Counting this pass's own writes keeps it accurate.
func (r *Reconciler) rolloutBudget(ctx context.Context, ng *v1.NodeGroup, p *pass) (*rolloutBudget, error) {
	if budget, ok := p.rollouts[ng.Name]; ok {
		return budget, nil
	}

	configs := &internalv1alpha1.NodeConfigList{}
	if err := r.sources.Reader.List(ctx, configs, client.MatchingLabels{nodecommon.NodeGroupLabel: ng.Name}); err != nil {
		return nil, fmt.Errorf("list NodeConfigs of %s: %w", ng.Name, err)
	}

	nodes := &corev1.NodeList{}
	if err := r.sources.Reader.List(ctx, nodes, client.MatchingLabels{nodecommon.NodeGroupLabel: ng.Name}); err != nil {
		return nil, fmt.Errorf("list Nodes of %s: %w", ng.Name, err)
	}
	// The same render apply() gives the node, so a node that already carries what
	// the cluster publishes is not mistaken for one left behind. Read once for
	// the group: renderSpec is pure and the inputs are memoised in the pass.
	in, err := r.clusterInputs(ctx, ng, p)
	if err != nil {
		return nil, err
	}
	desired := make(map[string]internalv1alpha1.NodeSpec, len(nodes.Items))
	for i := range nodes.Items {
		desired[nodes.Items[i].Name] = renderSpec(ng, &nodes.Items[i], in)
	}

	budget := &rolloutBudget{
		concurrency: ua.CalculateConcurrency(maxConcurrent(ng), len(configs.Items)),
		updating:    membersOfUpdating(configs.Items, desired),
	}
	p.rollouts[ng.Name] = budget
	return budget, nil
}

// membersOfUpdating names the nodes that occupy a rollout slot: those carrying
// the spec the cluster publishes now, and not yet proven.
//
// A node stuck on an older spec is not mid-rollout, it is broken, and counting
// it locks the cure out of a group that is already broken. A NodeConfig with no
// desired spec has no live Node and is about to be deleted.
func membersOfUpdating(configs []internalv1alpha1.NodeConfig,
	desired map[string]internalv1alpha1.NodeSpec) map[string]struct{} {
	updating := make(map[string]struct{})
	for i := range configs {
		nc := &configs[i]
		want, ok := desired[nc.Name]
		if !ok {
			continue
		}
		if applied(nc) {
			continue
		}
		// The carry-over apply() performs, with no reported address: a node
		// keeping what it booted with is not drift, and reading it as drift
		// would free a slot the node is genuinely holding.
		keepBootstrapOnlyFields(&want, &nc.Spec, nil)
		if !apiequality.Semantic.DeepEqual(nc.Spec, want) {
			continue
		}
		updating[nc.Name] = struct{}{}
	}
	return updating
}

// applied reports whether the node has finished reconciling the spec it was
// given. A node that has not reported back yet counts as still updating, so a
// silent agent holds the rollout rather than letting it run ahead.
func applied(nc *internalv1alpha1.NodeConfig) bool {
	// A node waiting for permission to interrupt itself has not applied
	// anything; counting it as done would free the slot and walk the change
	// through the whole group while every node sits waiting.
	if disruptionRequested(nc) {
		return false
	}
	// AppliedGeneration, not ObservedGeneration: a held node has observed the
	// current generation but still runs the previous one. What frees a slot is
	// the node actually running the published config.
	if nc.Status.AppliedGeneration != nc.Generation {
		return false
	}
	// ConfigurationApplied, not phase == Ready: a rolled-back or half-applied
	// spec rightly holds the rollout, but phase is Degraded for ANY unhealthy
	// subsystem, and an unrelated fault must not freeze the group forever.
	cond := meta.FindStatusCondition(nc.Status.Conditions, configurationAppliedCondition)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		return false
	}
	return cond.ObservedGeneration == nc.Generation
}

func maxConcurrent(ng *v1.NodeGroup) *intstr.IntOrString {
	if ng.Spec.Update == nil {
		return nil
	}
	return ng.Spec.Update.MaxConcurrent
}
