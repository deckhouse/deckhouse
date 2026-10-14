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

// readyConditionType answers whether a request's extension reaches the nodes it selects.
const readyConditionType = "Ready"

// phaseDegraded is the NER phase when the extension reaches no node it should; the Ready
// (phaseReady, shared with NodeConfig) phase means it does. Mirrors the NodeConfig status
// shape (phase + typed conditions).
const phaseDegraded = "Degraded"

// reconcileNERStatuses writes each NodeExtensionRequest's status: the groups it reaches
// and the Ready condition. It settles the contest the renders settle, so the status says
// what the renders did; a failing write blocks no render.
func (r *Reconciler) reconcileNERStatuses(ctx context.Context, logger logr.Logger) error {
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
	outcomes, err := readNEROutcomes(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("read what the nodes report about NodeExtensionRequests: %w", err)
	}

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
		if err := r.updateNERStatus(ctx, ner, conflicts, reach, nodes.Items, outcomes[ner.Name]); err != nil {
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
}

// nerReachOf is where a request's image goes: Engine groups take a system extension, groups
// bashible manages take a registry package. Only named groups are reported as refused: an
// empty selector means "wherever it works".
func nerReachOf(ner *deckhousev1alpha1.NodeExtensionRequest, immutable, mutable []string) nerReach {
	named := ner.Spec.NodeGroupSelector.MatchNames
	engine := matchedNodeGroups(named, immutable)
	bashible := matchedNodeGroups(named, mutable)
	reach := nerReach{selected: slices.Concat(engine, bashible)}
	switch ner.Spec.Extension.Kind {
	case deckhousev1alpha1.ExtensionKindSystemExtension:
		reach.targets = engine
		reach.refused = refusedGroups(named, bashible, "are managed by bashible, which installs no system extension")
	case deckhousev1alpha1.ExtensionKindPackage:
		reach.refused = refusedGroups(named, engine, "run Deckhouse Engine, which installs no registry package")
		if len(ner.Spec.KernelModules) == 0 {
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
func (r *Reconciler) updateNERStatus(ctx context.Context, ner *deckhousev1alpha1.NodeExtensionRequest, conflicts map[string]nerConflict, reach nerReach, nodes []corev1.Node, outcome nerOutcome) error {
	kind := ner.Spec.Extension.Kind
	desired := ner.Status.DeepCopy()
	desired.ObservedGeneration = ner.Generation
	desired.MatchedNodeGroups = reach.targets
	desired.MatchedNodes = nerMatchedNodeCount(ner, nodes, reach.targets)
	desired.AppliedNodes = outcome.applied
	desired.FailedNodes = outcome.failed
	desired.FailureMessage = outcome.message

	// The default is the request this controller refused, reason and message
	// already in hand.
	desired.Phase = phaseDegraded
	status := metav1.ConditionFalse
	reason, message := nerStatusReason(ner, conflicts, reach)
	// A refusal here reached no node, so nobody is late with an answer. Only a
	// refusal by the nodes keeps the arithmetic: there they did get it.
	desired.PendingNodes = pendingNodeCount(desired.MatchedNodes, outcome.applied, outcome.failed)
	if reason != "" {
		desired.PendingNodes = 0
	}
	switch {
	case reason == "" && outcome.failed > 0:
		// It resolved here and the nodes refused it. Reporting Ready on the
		// strength of the resolution alone is what made a rejected extension
		// indistinguishable from a working one.
		reason = reasonRefusedByNodes
		message = fmt.Sprintf("%d node(s) refused the %s, %d applied it: %s",
			outcome.failed, kindWords(kind), outcome.applied, outcome.message)
	case reason == "":
		desired.Phase = phaseReady
		status = metav1.ConditionTrue
		reason = reasonResolved
		message = strings.Join(append([]string{fmt.Sprintf("the %s resolved; %d of %d node(s) report it applied",
			kindWords(kind), outcome.applied, desired.MatchedNodes)}, reach.refused...), "; ")
	}
	meta.SetStatusCondition(&desired.Conditions, metav1.Condition{
		Type:               readyConditionType,
		Status:             status,
		ObservedGeneration: ner.Generation,
		Reason:             reason,
		Message:            message,
	})

	if apiequality.Semantic.DeepEqual(&ner.Status, desired) {
		return nil
	}
	// The whole status in one JSON patch, as nsprstatus.go writes it: a merge patch
	// leaves out the counters that stay 0 and keeps the fields that went empty.
	patch, err := json.Marshal([]map[string]any{{"op": "add", "path": "/status", "value": desired}})
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	if err := r.Client.Status().Patch(ctx, ner, client.RawPatch(types.JSONPatchType, patch)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}
	return nil
}

// nerMatchedNodeCount counts the nodes of the selected groups the render gives
// the extension to, node labels included (nerMatchesNode).
func nerMatchedNodeCount(ner *deckhousev1alpha1.NodeExtensionRequest, nodes []corev1.Node, groups []string) int32 {
	var count int32
	for i := range nodes {
		group := nodes[i].Labels[nodecommon.NodeGroupLabel]
		if !slices.Contains(groups, group) {
			continue
		}
		if nerMatchesNode(ner, &nodes[i], group) {
			count++
		}
	}
	return count
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

// nerStatusReason returns the reason a request is not ready and a human message for
// its Ready condition, or empty strings when it reaches its nodes: an invalid extension
// first, then a lost contest, then nowhere to go.
func nerStatusReason(ner *deckhousev1alpha1.NodeExtensionRequest, conflicts map[string]nerConflict, reach nerReach) (string, string) {
	if !extensionValid(ner) {
		return reasonInvalidExtension, "extension must set both name and digest"
	}
	if conflict, lost := conflicts[ner.Name]; lost {
		return conflict.reason, conflictMessage(conflict)
	}
	kind := ner.Spec.Extension.Kind
	if kind == deckhousev1alpha1.ExtensionKindPackage && len(ner.Spec.KernelModules) > 0 {
		return reasonNotDeliverable, "kernel modules load from a system extension only, and this image is a registry package: the request reaches no node"
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
	return fmt.Sprintf("the %s reaches no node: %s", kindWords(kind), strings.Join(reach.refused, "; "))
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
		return fmt.Sprintf("extension %s already claimed by NodeExtensionRequest %q", conflict.field, conflict.winner)
	case reasonModuleConflict:
		return fmt.Sprintf("kernel module %q is already loaded with different parameters by NodeExtensionRequest %q", conflict.field, conflict.winner)
	default:
		return conflict.reason
	}
}

// platformConflictMessage explains a registry package request that yields to the platform.
func platformConflictMessage(conflict nerConflict) string {
	if conflict.field == "name" {
		return fmt.Sprintf("name %q is taken by a platform package; the name is only a label, pick another", conflict.platformPackage)
	}
	return fmt.Sprintf("this digest is installed by the platform as package %q; removing the request would remove that package's files",
		conflict.platformPackage)
}
