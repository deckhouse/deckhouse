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
	"slices"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// reconcileNSPRStatuses writes each NodeStaticPodRequest's status (the checks
// this controller made, the matched groups, the Ready condition and what the
// nodes report). Mirrors reconcileNERStatuses (nerstatus.go). It runs off the
// same pass that re-renders the nodes, so editing an object refreshes both the
// nodes it targets and its status.
func (r *Reconciler) reconcileNSPRStatuses(ctx context.Context, logger logr.Logger) error {
	nsprs := &deckhousev1alpha1.NodeStaticPodRequestList{}
	if err := r.Client.List(ctx, nsprs); err != nil {
		return fmt.Errorf("list NodeStaticPodRequests for status: %w", err)
	}
	if len(nsprs.Items) == 0 {
		return nil
	}

	ordered := orderedNSPRs(nsprs.Items)
	rejected := rejectedNSPRs(ordered)
	groups, err := r.immutableNodeGroupNames(ctx)
	if err != nil {
		return err
	}

	// What the fleet made of these pods, from the one place the answer lives:
	// the NodeConfig of every node in an Immutable group. Nothing is published
	// without it: counts absent read as counts zero, so a transient read failure
	// would turn "every node refused it" into a clean Ready and lose the reason.
	outcomes, err := readNodeConfigOutcomes(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("read what the nodes report about NodeStaticPodRequests: %w", err)
	}

	// The denominator: read once for every object, since it is the same listing.
	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes); err != nil {
		return fmt.Errorf("list Nodes for the NodeStaticPodRequest counts: %w", err)
	}

	// Iterated in contest order so a reader of the log sees the winner before the
	// objects that lost to it.
	for _, nspr := range ordered {
		if err := r.updateNSPRStatus(ctx, nspr, rejected, groups, nodes.Items, outcomes[nspr.Name]); err != nil {
			logger.Error(err, "cannot update NodeStaticPodRequest status", "staticPodRequest", nspr.Name)
		}
	}
	return nil
}

// updateNSPRStatus computes and patches one object's status, skipping the write
// when nothing changed.
func (r *Reconciler) updateNSPRStatus(ctx context.Context, nspr *deckhousev1alpha1.NodeStaticPodRequest, rejected map[string]nsprRefusal, nodeGroups []string, nodes []corev1.Node, outcome nsprOutcome) error {
	desired := nspr.Status.DeepCopy()
	desired.ObservedGeneration = nspr.Generation
	desired.MatchedNodeGroups = matchedNodeGroups(nspr.Spec.NodeGroupSelector.MatchNames, nodeGroups)
	desired.MatchedNodes = matchedNodeCount(nodes, desired.MatchedNodeGroups)
	desired.AppliedNodes = outcome.applied
	desired.FailedNodes = outcome.failed
	desired.FailureMessage = outcome.message

	// The default is the object this controller refused, reason and message
	// already in hand.
	desired.Phase = phaseDegraded
	status := metav1.ConditionFalse
	reason, message := "", ""
	if refusal, refused := rejected[nspr.Name]; refused {
		reason, message = refusal.reason, refusal.message
	}
	// A refusal here reached no node, so nobody is late with an answer. Only a
	// refusal by the nodes keeps the arithmetic: there they did get it.
	desired.PendingNodes = pendingNodeCount(desired.MatchedNodes, outcome.applied, outcome.failed)
	if reason != "" {
		desired.PendingNodes = 0
	}
	switch {
	case reason == "" && outcome.failed > 0:
		reason = reasonRefusedByNodes
		message = fmt.Sprintf("%d node(s) refused the static pod, %d wrote it: %s",
			outcome.failed, outcome.applied, outcome.message)
	case reason == "":
		desired.Phase = phaseReady
		status = metav1.ConditionTrue
		reason = reasonResolved
		message = fmt.Sprintf("the static pod resolved; %d of %d node(s) report it written", outcome.applied, desired.MatchedNodes)
	}
	meta.SetStatusCondition(&desired.Conditions, metav1.Condition{
		Type:               readyConditionType,
		Status:             status,
		ObservedGeneration: nspr.Generation,
		Reason:             reason,
		Message:            message,
	})

	if apiequality.Semantic.DeepEqual(&nspr.Status, desired) {
		return nil
	}
	patched := nspr.DeepCopy()
	patched.Status = *desired
	if err := r.Client.Status().Patch(ctx, patched, client.MergeFrom(nspr)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}
	return nil
}

// matchedNodeCount is how many nodes belong to the groups a request selects.
func matchedNodeCount(nodes []corev1.Node, groups []string) int32 {
	var count int32
	for i := range nodes {
		if slices.Contains(groups, nodes[i].Labels[nodecommon.NodeGroupLabel]) {
			count++
		}
	}
	return count
}

// pendingNodeCount never goes below zero: a node can report before the node
// list this pass read has caught up with it.
func pendingNodeCount(matched, applied, failed int32) int32 {
	return max(matched-applied-failed, 0)
}
