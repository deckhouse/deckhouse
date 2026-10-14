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
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	rejected, onMutable := rejectedNSPRs(ordered)
	immutable, mutable, err := r.nodeGroupNames(ctx)
	if err != nil {
		return err
	}

	// What the fleet made of these pods, from every node's NodeConfig. Nothing is published
	// without it: absent counts read as zero, so one failed read would turn "every node
	// refused it" into a clean Ready and lose the reason.
	outcomes, err := readNodeConfigOutcomes(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("read what the nodes report about NodeStaticPodRequests: %w", err)
	}

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
		if err := r.updateNSPRStatus(ctx, nspr, rejected, capped[nspr.Name], reach, nodes.Items, outcomes[nspr.Name]); err != nil {
			logger.Error(err, "cannot update NodeStaticPodRequest status", "staticPodRequest", nspr.Name)
		}
	}
	return nil
}

// Ready reasons of a request that resolved and reaches no node. Neither is a
// fault, so Ready stays True and `kubectl wait --for=condition=Ready` returns.
const (
	// reasonServedByBashible is registry-agent on the groups bashible manages:
	// a bashible step writes it there, and the agent is never asked for it.
	reasonServedByBashible = "ServedByBashible"
	// reasonNoNodesSelected names what keeps the request from every node.
	reasonNoNodesSelected = "NoNodesSelected"
)

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
		return reasonServedByBashible, fmt.Sprintf("a bashible step writes this static pod on the nodes of NodeGroup(s) %s; the node agent is not asked for it",
			strings.Join(reach.servedByBashible, ", "))
	}
	var causes []string
	if len(reach.missing) > 0 {
		causes = append(causes, fmt.Sprintf("NodeGroup(s) %s do not exist", strings.Join(reach.missing, ", ")))
	}
	if len(reach.targets) > 0 {
		causes = append(causes, fmt.Sprintf("NodeGroup(s) %s have no nodes", strings.Join(reach.targets, ", ")))
	}
	if len(causes) == 0 {
		causes = append(causes, "there is no NodeGroup to select")
	}
	return reasonNoNodesSelected, "the static pod reaches no node: " + strings.Join(causes, "; ")
}

// updateNSPRStatus computes and patches one object's status, skipping the write
// when nothing changed. Every group the selector matches is listed; the nodes of
// those that are served by bashible are not counted.
func (r *Reconciler) updateNSPRStatus(ctx context.Context, nspr *deckhousev1alpha1.NodeStaticPodRequest, rejected map[string]nsprRefusal, cappedIn []string, reach nsprReach, nodes []corev1.Node, outcome nsprOutcome) error {
	desired := nspr.Status.DeepCopy()
	desired.ObservedGeneration = nspr.Generation
	desired.MatchedNodeGroups = slices.Concat(reach.targets, reach.servedByBashible)
	slices.Sort(desired.MatchedNodeGroups)
	desired.MatchedNodes = matchedNodeCount(nodes, reach.targets)
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
	if reason == "" && len(cappedIn) > 0 {
		reason = reasonLimitExceeded
		message = fmt.Sprintf("more than %d static pods select node group(s) %s; this one is among the youngest and is left out of their node configs",
			maxStaticPods, strings.Join(cappedIn, ", "))
	}
	// A refusal here reached no node, so nobody is late with an answer. Only a
	// refusal by the nodes keeps the arithmetic: there they did get it.
	desired.PendingNodes = pendingNodeCount(desired.MatchedNodes, outcome.applied, outcome.failed)
	if reason != "" {
		desired.PendingNodes = 0
	}
	// A node that reports it written was reached, whatever the node list says yet.
	reachedNone := desired.MatchedNodes == 0 && outcome.applied == 0
	switch {
	case reason == "" && outcome.failed > 0:
		reason = reasonRefusedByNodes
		message = fmt.Sprintf("%d node(s) refused the static pod, %d wrote it: %s",
			outcome.failed, outcome.applied, outcome.message)
	case reason == "" && reachedNone:
		desired.Phase = phaseReady
		status = metav1.ConditionTrue
		reason, message = reachedNoNode(reach)
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
	// The whole status in one JSON patch: a merge patch would leave out the
	// counters that stay 0 and keep the fields that went empty.
	patch, err := json.Marshal([]map[string]any{{"op": "add", "path": "/status", "value": desired}})
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	if err := r.Client.Status().Patch(ctx, nspr, client.RawPatch(types.JSONPatchType, patch)); err != nil {
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
