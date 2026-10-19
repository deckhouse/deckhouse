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
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// reconcileNSPRStatuses writes each NodeStaticPodRequest's status (the checks
// this controller made, the matched groups, the Ready condition and what the
// nodes report). Mirrors reconcileNERStatuses (nerstatus.go). It runs off the
// same pass that re-renders the nodes, so editing an object refreshes both the
// nodes it targets and its status.
func (r *Reconciler) reconcileNSPRStatuses(ctx context.Context, logger logr.Logger, p *pass) error {
	nsprs := &deckhousev1alpha1.NodeStaticPodRequestList{}
	if err := r.Client.List(ctx, nsprs); err != nil {
		return fmt.Errorf("list NodeStaticPodRequests for status: %w", err)
	}
	if len(nsprs.Items) == 0 {
		return nil
	}

	ordered := orderedNSPRs(nsprs.Items)
	rejected, onMutable := rejectedNSPRs(ordered)
	immutable, mutable, err := r.nodeGroupNames(ctx)
	if err != nil {
		return err
	}

	// What the fleet made of these pods, from every node's NodeConfig. Nothing is published
	// without it: absent counts read as zero, so one failed read would turn "every node
	// refused it" into a clean Ready and lose the reason.
	f, err := r.readFleet(ctx, p)
	if err != nil {
		return fmt.Errorf("read what the nodes report about NodeStaticPodRequests: %w", err)
	}
	manifests := make(map[string]string, len(nsprs.Items))
	for i := range nsprs.Items {
		manifests[nsprs.Items[i].Name] = nsprs.Items[i].Spec.Manifest
	}
	outcomes := nsprOutcomes(f, manifests)

	// The denominator: read once for every object, since it is the same listing.
	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes); err != nil {
		return fmt.Errorf("list Nodes for the NodeStaticPodRequest counts: %w", err)
	}

	// A bashible group is capped the way its nodes are rendered: without the
	// requests a bashible step already serves there.
	capped := cappedNSPRs(ordered, rejected, immutable)
	for name, groups := range cappedNSPRs(ordered, onMutable, mutable) {
		capped[name] = append(capped[name], groups...)
	}
	// Iterated in contest order so a reader of the log sees the winner before the
	// objects that lost to it.
	for _, nspr := range ordered {
		// Refused on a Mutable node and nowhere else: a bashible step serves it there.
		_, refused := rejected[nspr.Name]
		_, refusedOnMutable := onMutable[nspr.Name]
		reach := reachOf(nspr, immutable, mutable, refusedOnMutable && !refused)
		if err := r.updateNSPRStatus(ctx, nspr, rejected, capped[nspr.Name], reach, nodes.Items, f, outcomes[nspr.Name]); err != nil {
			logger.Error(err, "cannot update NodeStaticPodRequest status", "staticPodRequest", nspr.Name)
		}
	}
	return nil
}

// reasonServedByBashible is a Ready reason of an accepted request that reaches no node:
// registry-agent on the groups bashible manages, where a bashible step writes it and the
// agent is never asked for it. Not a fault, so Ready stays True, as with NoNodesSelected.
const reasonServedByBashible = "ServedByBashible"

// nsprReach is where one request's static pod goes, by the groups it selects.
type nsprReach struct {
	// targets are the groups whose nodes the agent writes the pod to.
	targets []string
	// servedByBashible are the bashible groups where a bashible step writes it.
	servedByBashible []string
	// missing are the selected names no NodeGroup carries.
	missing []string
}

// reachOf is where a request's static pod goes.
func reachOf(nspr *deckhousev1alpha1.NodeStaticPodRequest, immutable, mutable []string, servedByBashible bool) nsprReach {
	selected := nspr.Spec.NodeGroupSelector.MatchNames
	reach := nsprReach{targets: matchedNodeGroups(selected, immutable), missing: missingNodeGroups(selected, immutable, mutable)}
	bashible := matchedNodeGroups(selected, mutable)
	if servedByBashible {
		reach.servedByBashible = bashible
		return reach
	}
	reach.targets = append(reach.targets, bashible...)
	return reach
}

// missingNodeGroups are the names a selector lists that no NodeGroup carries.
func missingNodeGroups(selected, immutable, mutable []string) []string {
	var missing []string
	for _, name := range selected {
		if !slices.Contains(immutable, name) && !slices.Contains(mutable, name) {
			missing = append(missing, name)
		}
	}
	return missing
}

// reachedNoNode says why a resolved request is on no node: a bashible step
// serves it there, or nothing it selects has a node the agent writes to.
func reachedNoNode(reach nsprReach) (string, string) {
	if len(reach.servedByBashible) > 0 {
		return reasonServedByBashible, fmt.Sprintf("a bashible step writes this static pod on the nodes of NodeGroup(s) %s. The node agent does not write it there",
			strings.Join(reach.servedByBashible, ", "))
	}
	return reasonNoNodesSelected, noNodeMessage(reach.missing, reach.targets)
}

// noNodeMessage says why the groups a selector names give no node: the names no NodeGroup
// carries, the selected groups that have no nodes, and the further causes given.
func noNodeMessage(missing, empty []string, more ...string) string {
	var causes []string
	if len(missing) > 0 {
		causes = append(causes, fmt.Sprintf("NodeGroup(s) %s do not exist", strings.Join(missing, ", ")))
	}
	if len(empty) > 0 {
		causes = append(causes, fmt.Sprintf("NodeGroup(s) %s have no nodes", strings.Join(empty, ", ")))
	}
	causes = append(causes, more...)
	if len(causes) == 0 {
		causes = append(causes, "there is no NodeGroup to select")
	}
	return "the request selects no node: " + strings.Join(causes, "; ")
}

// updateNSPRStatus computes and patches one object's status, skipping the write
// when nothing changed. Every group the selector matches is listed; the nodes of
// those that are served by bashible are not counted.
func (r *Reconciler) updateNSPRStatus(ctx context.Context, nspr *deckhousev1alpha1.NodeStaticPodRequest, rejected map[string]nsprRefusal, cappedIn []string, reach nsprReach, nodes []corev1.Node, f fleet, outcome requestOutcome) error {
	matched := matchedNodes(nodes, reach.targets)
	desired := nspr.Status.DeepCopy()
	desired.ObservedGeneration = nspr.Generation
	desired.MatchedNodeGroups = slices.Concat(reach.targets, reach.servedByBashible)
	slices.Sort(desired.MatchedNodeGroups)
	desired.MatchedNodes = int32(len(matched))
	desired.AppliedNodes = int32(len(outcome.applied))
	desired.FailedNodes = int32(len(outcome.failed))
	desired.PendingNodes = pendingNodeCount(desired.MatchedNodes, desired.AppliedNodes, desired.FailedNodes)

	accepted := nsprAccepted(nspr, rejected, cappedIn)
	pending := pendingNodes(f, matched, outcome, func(config *internalv1alpha1.NodeConfig) string {
		manifest, ok := specManifest(config, nspr.Name)
		return pendingReason(f, config, ok && manifest == nspr.Spec.Manifest, "")
	})
	// A refusal here reached no node, so nobody is late with an answer.
	if accepted.Status != metav1.ConditionTrue {
		desired.PendingNodes = 0
		pending = nil
	}
	desired.Nodes = statusNodes(outcome.failed, pending)
	ready := nsprReady(nspr, accepted, desired, reach, refusalMessage(outcome.failed), pendingMessage(pending))
	meta.SetStatusCondition(&desired.Conditions, accepted)
	meta.SetStatusCondition(&desired.Conditions, ready)
	desired.Phase = requestPhase(accepted, ready)

	if apiequality.Semantic.DeepEqual(&nspr.Status, desired) {
		return nil
	}
	return patchRequestStatus(ctx, r.Client, nspr, desired)
}

// nsprAccepted is this controller's verdict on an object: the refusals of the contest
// first, then the per-group cap.
func nsprAccepted(nspr *deckhousev1alpha1.NodeStaticPodRequest, rejected map[string]nsprRefusal, cappedIn []string) metav1.Condition {
	cond := metav1.Condition{
		Type:               acceptedConditionType,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: nspr.Generation,
		Reason:             reasonAccepted,
		Message:            "the static pod is accepted",
	}
	if refusal, refused := rejected[nspr.Name]; refused {
		cond.Status, cond.Reason, cond.Message = metav1.ConditionFalse, refusal.reason, nodecommon.Truncate(refusal.message, maxConditionMessage)
		return cond
	}
	if len(cappedIn) > 0 {
		cond.Status, cond.Reason = metav1.ConditionFalse, reasonLimitExceeded
		cond.Message = fmt.Sprintf("more than %d static pods select NodeGroup(s) %s. This request is among the newest and is not added to the NodeConfigs of their nodes",
			maxStaticPods, strings.Join(cappedIn, ", "))
	}
	return cond
}

// nsprReady says how far the selected nodes have got with writing the manifest: refusal names
// the nodes that refused it, pending the ones still owing an answer.
func nsprReady(nspr *deckhousev1alpha1.NodeStaticPodRequest, accepted metav1.Condition, status *deckhousev1alpha1.NodeStaticPodRequestStatus, reach nsprReach, refusal, pending string) metav1.Condition {
	cond := metav1.Condition{Type: readyConditionType, ObservedGeneration: nspr.Generation, Status: metav1.ConditionFalse}
	switch {
	case accepted.Status != metav1.ConditionTrue:
		cond.Reason, cond.Message = reasonNotAccepted, "the static pod is not accepted: "+accepted.Reason
	case status.FailedNodes > 0:
		cond.Reason = reasonRefusedByNodes
		cond.Message = fmt.Sprintf("%d of %d node(s) refused the static pod, %d wrote it: %s",
			status.FailedNodes, status.MatchedNodes, status.AppliedNodes, refusal)
	// A node that reports it written was reached, whatever the node list says yet.
	case status.MatchedNodes == 0 && status.AppliedNodes == 0:
		cond.Status = metav1.ConditionTrue
		cond.Reason, cond.Message = reachedNoNode(reach)
	case status.PendingNodes > 0:
		cond.Reason = reasonRolloutInProgress
		cond.Message = fmt.Sprintf("%d of %d node(s) wrote the current manifest; %d pending",
			status.AppliedNodes, status.MatchedNodes, status.PendingNodes)
		if pending != "" {
			cond.Message += ": " + pending
		}
	default:
		cond.Status = metav1.ConditionTrue
		cond.Reason = reasonAllNodesApplied
		cond.Message = fmt.Sprintf("%d of %d node(s) wrote the current manifest", status.AppliedNodes, status.MatchedNodes)
	}
	cond.Message = nodecommon.Truncate(cond.Message, maxConditionMessage)
	return cond
}

// matchedNodes names, sorted, the nodes that belong to the groups a request selects.
func matchedNodes(nodes []corev1.Node, groups []string) []string {
	var matched []string
	for i := range nodes {
		if slices.Contains(groups, nodes[i].Labels[nodecommon.NodeGroupLabel]) {
			matched = append(matched, nodes[i].Name)
		}
	}
	slices.Sort(matched)
	return matched
}

// pendingNodeCount never goes below zero: a node can report before the node
// list this pass read has caught up with it.
func pendingNodeCount(matched, applied, failed int32) int32 {
	return max(matched-applied-failed, 0)
}
