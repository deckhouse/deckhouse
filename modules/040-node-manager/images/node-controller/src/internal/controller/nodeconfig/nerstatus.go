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

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// The two conditions of a request, NodeExtensionRequest and NodeStaticPodRequest alike:
// Accepted is this controller's verdict on the object, Ready says the fleet runs it.
const (
	acceptedConditionType = "Accepted"
	readyConditionType    = "Ready"
)

// Reasons of the Ready condition. NoNodesSelected and ServedByBashible (nsprstatus.go)
// are True as well: nothing is waited for.
const (
	reasonAllNodesApplied   = "AllNodesApplied"
	reasonRolloutInProgress = "RolloutInProgress"
	// reasonRefusedByNodes is the request accepted here and rejected where it matters.
	reasonRefusedByNodes  = "RefusedByNodes"
	reasonNotAccepted     = "NotAccepted"
	reasonNoNodesSelected = "NoNodesSelected"
)

// The phase of a request, the scale NodeConfig uses too (phaseReady is shared).
const (
	phasePending  = "Pending"
	phaseDegraded = "Degraded"
)

// maxConditionMessage bounds a condition message: a node's refusal can carry a whole
// script output, and a status write must not fail on its size.
const maxConditionMessage = 4096

// reconcileNERStatuses writes each NodeExtensionRequest's status: the groups it reaches
// and the Ready condition. It settles the contest the renders settle, so the status says
// what the renders did; a failing write blocks no render.
func (r *Reconciler) reconcileNERStatuses(ctx context.Context, logger logr.Logger, p *pass) error {
	ners := &deckhousev1alpha1.NodeExtensionRequestList{}
	if err := r.Client.List(ctx, ners); err != nil {
		return fmt.Errorf("list NodeExtensionRequests for status: %w", err)
	}
	if len(ners.Items) == 0 {
		return nil
	}

	// Only a registry package contends with the platform's packages, so the release is
	// read for those alone, and a release that cannot be read holds back their status only.
	var platform []internalv1alpha1.Extension
	var platformErr error
	if slices.ContainsFunc(ners.Items, isPackageRequest) {
		platform, platformErr = r.platformPackages(ctx)
	}
	_, conflicts := settleNERs(ners.Items, platform)
	immutable, mutable, err := r.nodeGroupNames(ctx)
	if err != nil {
		return err
	}

	// What the nodes made of these requests. Nothing is published without it:
	// counts absent read as counts zero, so a transient read failure would turn
	// "50 nodes refused it" into a clean Ready and lose the reason with it.
	digests := make(map[string]string, len(ners.Items))
	for i := range ners.Items {
		digests[ners.Items[i].Name] = ners.Items[i].Spec.Extension.Digest
	}
	f, err := r.readFleet(ctx, p)
	if err != nil {
		return fmt.Errorf("read what the nodes report about NodeExtensionRequests: %w", err)
	}
	outcomes := nerOutcomes(f, digests)

	// The denominator: read once for every request, since it is the same listing.
	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes); err != nil {
		return fmt.Errorf("list Nodes for the NodeExtensionRequest counts: %w", err)
	}

	for i := range ners.Items {
		ner := &ners.Items[i]
		if platformErr != nil && isPackageRequest(*ner) {
			continue
		}
		reach := nerReachOf(ner, immutable, mutable)
		if err := r.updateNERStatus(ctx, ner, conflicts, reach, nodes.Items, f, outcomes[ner.Name]); err != nil {
			logger.Error(err, "cannot update NodeExtensionRequest status", "request", ner.Name)
		}
	}
	return platformErr
}

func isPackageRequest(ner deckhousev1alpha1.NodeExtensionRequest) bool {
	return ner.Spec.Extension.Kind == deckhousev1alpha1.ExtensionKindPackage
}

// platformPackages reads the packages the platform document of this release installs.
func (r *Reconciler) platformPackages(ctx context.Context) ([]internalv1alpha1.Extension, error) {
	images, err := r.sources.readImagesDigests(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the release images for the NodeExtensionRequest contest: %w", err)
	}
	platform, err := r.sources.readPlatformDocument(ctx, images)
	if err != nil {
		return nil, fmt.Errorf("read the platform document for the NodeExtensionRequest contest: %w", err)
	}
	return platform.Extensions, nil
}

// nodeGroupNames lists the NodeGroups a request can select: the Engine ones, then
// the ones bashible manages. A listing failure is an error, not an empty answer:
// publishing "no groups matched" from a read that did not happen misleads everyone.
func (r *Reconciler) nodeGroupNames(ctx context.Context) ([]string, []string, error) {
	ngs := &v1.NodeGroupList{}
	if err := r.Client.List(ctx, ngs); err != nil {
		return nil, nil, fmt.Errorf("list NodeGroups: %w", err)
	}
	var immutable, mutable []string
	for i := range ngs.Items {
		if IsMutable(&ngs.Items[i]) {
			mutable = append(mutable, ngs.Items[i].Name)
			continue
		}
		immutable = append(immutable, ngs.Items[i].Name)
	}
	return immutable, mutable, nil
}

// nerReach is where a request's image goes among the groups it selects.
type nerReach struct {
	// selected are the existing groups the selector picks, whatever their type.
	selected []string
	// targets are the selected groups whose nodes install the image.
	targets []string
	// refused say which named groups cannot install it, and why.
	refused []string
	// missing are the named groups no NodeGroup carries.
	missing []string
}

// nerReachOf is where a request's image goes: Engine groups take a system extension, groups
// bashible manages take a registry package. Only named groups are reported as refused: an
// empty selector means "wherever it works".
func nerReachOf(ner *deckhousev1alpha1.NodeExtensionRequest, immutable, mutable []string) nerReach {
	named := ner.Spec.NodeGroupSelector.MatchNames
	engine := matchedNodeGroups(named, immutable)
	bashible := matchedNodeGroups(named, mutable)
	reach := nerReach{selected: slices.Concat(engine, bashible), missing: missingNodeGroups(named, immutable, mutable)}
	switch ner.Spec.Extension.Kind {
	case deckhousev1alpha1.ExtensionKindSystemExtension:
		reach.targets = engine
		reach.refused = refusedGroups(named, bashible, "are managed by bashible, which installs no system extension")
	case deckhousev1alpha1.ExtensionKindPackage:
		reach.refused = refusedGroups(named, engine, "run Deckhouse Engine, which installs no registry package")
		if len(ner.Spec.Extension.KernelModules) == 0 {
			reach.targets = bashible
		}
	}
	return reach
}

// refusedGroups explains the named groups that cannot install the image, if any.
func refusedGroups(named, groups []string, why string) []string {
	if len(named) == 0 || len(groups) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("NodeGroup(s) %s %s", strings.Join(groups, ", "), why)}
}

// updateNERStatus computes and writes one request's status, skipping the write when
// nothing changed.
func (r *Reconciler) updateNERStatus(ctx context.Context, ner *deckhousev1alpha1.NodeExtensionRequest, conflicts map[string]nerConflict, reach nerReach, nodes []corev1.Node, f fleet, outcome requestOutcome) error {
	matched := nerMatchedNodes(ner, nodes, reach.targets)
	desired := ner.Status.DeepCopy()
	desired.ObservedGeneration = ner.Generation
	desired.MatchedNodeGroups = reach.targets
	desired.MatchedNodes = int32(len(matched))
	desired.AppliedNodes = int32(len(outcome.applied))
	desired.FailedNodes = int32(len(outcome.failed))
	desired.PendingNodes = pendingNodeCount(desired.MatchedNodes, desired.AppliedNodes, desired.FailedNodes)

	accepted := nerAccepted(ner, conflicts, reach)
	pending := pendingNodes(f, matched, outcome, func(config *internalv1alpha1.NodeConfig) string {
		return pendingReason(f, config, carriesNER(config, ner), nerHeld(config, ner.Spec.Extension))
	})
	// A refusal here reached no node, so nobody is late with an answer.
	if accepted.Status != metav1.ConditionTrue {
		desired.PendingNodes = 0
		pending = nil
	}
	desired.Nodes = statusNodes(outcome.failed, pending)
	ready := nerReady(ner, accepted, desired, reach, nodes, refusalMessage(outcome.failed), pendingMessage(pending))
	meta.SetStatusCondition(&desired.Conditions, accepted)
	meta.SetStatusCondition(&desired.Conditions, ready)
	desired.Phase = requestPhase(accepted, ready)

	if apiequality.Semantic.DeepEqual(&ner.Status, desired) {
		return nil
	}
	return patchRequestStatus(ctx, r.Client, ner, desired)
}

// nerAccepted is this controller's verdict on a request: an invalid extension first, then
// a lost contest, then nowhere to go.
func nerAccepted(ner *deckhousev1alpha1.NodeExtensionRequest, conflicts map[string]nerConflict, reach nerReach) metav1.Condition {
	cond := metav1.Condition{Type: acceptedConditionType, ObservedGeneration: ner.Generation, Status: metav1.ConditionFalse}
	reason, message := nerStatusReason(ner, conflicts, reach)
	if reason == "" {
		cond.Status = metav1.ConditionTrue
		reason = reasonAccepted
		message = strings.Join(append([]string{fmt.Sprintf("the %s is accepted", kindWords(ner.Spec.Extension.Kind))}, reach.refused...), ". ")
	}
	cond.Reason, cond.Message = reason, nodecommon.Truncate(message, maxConditionMessage)
	return cond
}

// nerReady says how far the selected nodes have got with the request's digest: refusal names
// the nodes that refused it, pending the ones still owing an answer.
func nerReady(ner *deckhousev1alpha1.NodeExtensionRequest, accepted metav1.Condition, status *deckhousev1alpha1.NodeExtensionRequestStatus, reach nerReach, nodes []corev1.Node, refusal, pending string) metav1.Condition {
	cond := metav1.Condition{Type: readyConditionType, ObservedGeneration: ner.Generation, Status: metav1.ConditionFalse}
	ext := ner.Spec.Extension
	switch {
	case accepted.Status != metav1.ConditionTrue:
		cond.Reason, cond.Message = reasonNotAccepted, "the request is not accepted: "+accepted.Reason
	case status.FailedNodes > 0:
		cond.Reason = reasonRefusedByNodes
		cond.Message = fmt.Sprintf("%d of %d node(s) refused the %s %s, %d run it: %s",
			status.FailedNodes, status.MatchedNodes, kindWords(ext.Kind), ext.Digest, status.AppliedNodes, refusal)
	case status.MatchedNodes == 0 && status.AppliedNodes == 0:
		cond.Status = metav1.ConditionTrue
		cond.Reason, cond.Message = reasonNoNodesSelected, nerReachesNoNode(reach, nodes)
	case status.PendingNodes > 0:
		cond.Reason = reasonRolloutInProgress
		cond.Message = fmt.Sprintf("%d of %d node(s) run %s; %d pending", status.AppliedNodes, status.MatchedNodes, ext.Digest, status.PendingNodes)
		if pending != "" {
			cond.Message += ": " + pending
		}
	default:
		cond.Status = metav1.ConditionTrue
		cond.Reason = reasonAllNodesApplied
		cond.Message = fmt.Sprintf("%d of %d node(s) run %s", status.AppliedNodes, status.MatchedNodes, ext.Digest)
	}
	cond.Message = nodecommon.Truncate(cond.Message, maxConditionMessage)
	return cond
}

// requestPhase follows the two conditions: Ready when the fleet runs the request,
// Degraded when this controller or the nodes refused it, Pending while it rolls out.
func requestPhase(accepted, ready metav1.Condition) string {
	if ready.Status == metav1.ConditionTrue {
		return phaseReady
	}
	if accepted.Status != metav1.ConditionTrue || ready.Reason == reasonRefusedByNodes {
		return phaseDegraded
	}
	return phasePending
}

// patchRequestStatus writes the whole status in one JSON patch: a merge patch leaves out
// the counters that stay 0 and keeps the fields that went empty.
func patchRequestStatus(ctx context.Context, c client.Client, obj client.Object, status any) error {
	patch, err := json.Marshal([]map[string]any{{"op": "add", "path": "/status", "value": status}})
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	if err := c.Status().Patch(ctx, obj, client.RawPatch(types.JSONPatchType, patch)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}
	return nil
}

// nerMatchedNodes names, sorted, the nodes of the selected groups the render gives
// the extension to, node labels included (nerMatchesNode).
func nerMatchedNodes(ner *deckhousev1alpha1.NodeExtensionRequest, nodes []corev1.Node, groups []string) []string {
	var matched []string
	for i := range nodes {
		group := nodes[i].Labels[nodecommon.NodeGroupLabel]
		if !slices.Contains(groups, group) {
			continue
		}
		if nerMatchesNode(ner, &nodes[i], group) {
			matched = append(matched, nodes[i].Name)
		}
	}
	slices.Sort(matched)
	return matched
}

// nerReachesNoNode says why an accepted request is on no node: the named groups that do not
// exist, the selected ones without nodes, and the ones whose nodes the nodeSelector leaves out.
func nerReachesNoNode(reach nerReach, nodes []corev1.Node) string {
	var empty, unlabelled []string
	for _, group := range reach.targets {
		inGroup := func(node corev1.Node) bool { return node.Labels[nodecommon.NodeGroupLabel] == group }
		if slices.ContainsFunc(nodes, inGroup) {
			unlabelled = append(unlabelled, group)
			continue
		}
		empty = append(empty, group)
	}
	var more []string
	if len(unlabelled) > 0 {
		more = append(more, fmt.Sprintf("no node of NodeGroup(s) %s has the labels from nodeSelector", strings.Join(unlabelled, ", ")))
	}
	return noNodeMessage(reach.missing, empty, more...)
}

// matchedNodeGroups returns the NodeGroups a selector picks out of groups: the
// listed names it intersects, or all of them when it names none.
func matchedNodeGroups(matchNames, groups []string) []string {
	var matched []string
	for _, name := range groups {
		if len(matchNames) == 0 || slices.Contains(matchNames, name) {
			matched = append(matched, name)
		}
	}
	slices.Sort(matched)
	return matched
}

// nerStatusReason returns the reason this controller refuses a request and a human
// message for its Accepted condition, or empty strings when it accepts it: an invalid
// extension first, then a lost contest, then nowhere to go.
func nerStatusReason(ner *deckhousev1alpha1.NodeExtensionRequest, conflicts map[string]nerConflict, reach nerReach) (string, string) {
	if !extensionValid(ner) {
		return reasonInvalidExtension, "extension must set both name and digest"
	}
	if conflict, lost := conflicts[ner.Name]; lost {
		return conflict.reason, conflictMessage(conflict)
	}
	kind := ner.Spec.Extension.Kind
	if kind == deckhousev1alpha1.ExtensionKindPackage && len(ner.Spec.Extension.KernelModules) > 0 {
		return reasonNotDeliverable, "kernel modules load only from a system extension, and this image is a registry package. No node installs it"
	}
	if len(reach.selected) > 0 && len(reach.targets) == 0 {
		return reasonNotDeliverable, notDeliverableMessage(kind, reach)
	}
	return "", ""
}

// notDeliverableMessage says why the selected groups take none of this image.
func notDeliverableMessage(kind deckhousev1alpha1.ExtensionKind, reach nerReach) string {
	if len(reach.refused) == 0 {
		return fmt.Sprintf("no selected NodeGroup installs a %s", kindWords(kind))
	}
	return fmt.Sprintf("no node can install the %s: %s", kindWords(kind), strings.Join(reach.refused, "; "))
}

// kindWords names a kind of image in a message.
func kindWords(kind deckhousev1alpha1.ExtensionKind) string {
	switch kind {
	case deckhousev1alpha1.ExtensionKindSystemExtension:
		return "system extension"
	case deckhousev1alpha1.ExtensionKindPackage:
		return "registry package"
	default:
		return "extension"
	}
}

// conflictMessage explains a lost uniqueness contest for the condition message.
func conflictMessage(conflict nerConflict) string {
	switch conflict.reason {
	case reasonReservedName:
		return "extension name is reserved for a platform extension"
	case reasonConflict:
		if conflict.platformPackage != "" {
			return platformConflictMessage(conflict)
		}
		return fmt.Sprintf("the extension %s is already used by NodeExtensionRequest %q", conflict.field, conflict.winner)
	case reasonModuleConflict:
		return fmt.Sprintf("kernel module %q is already loaded with different parameters by NodeExtensionRequest %q", conflict.field, conflict.winner)
	default:
		return conflict.reason
	}
}

// platformConflictMessage explains a registry package request that yields to the platform.
func platformConflictMessage(conflict nerConflict) string {
	if conflict.field == "name" {
		return fmt.Sprintf("name %q is taken by a platform package. Choose another name", conflict.platformPackage)
	}
	return fmt.Sprintf("the platform installs this digest as package %q. Deleting the request would delete the files of that package",
		conflict.platformPackage)
}
