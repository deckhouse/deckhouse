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
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

// readyConditionType answers whether a request's sysext resolved to an image the
// nodes can pull.
const readyConditionType = "Ready"

// phaseDegraded is the NER phase when the sysext cannot be resolved; the Ready
// (phaseReady, shared with NodeConfig) phase means it resolved. Mirrors the
// NodeConfig status shape (phase + typed conditions).
const phaseDegraded = "Degraded"

// reconcileNERStatuses writes each NodeExtensionRequest's status (resolution,
// matched groups, Ready condition). Best-effort — a status write failing never
// blocks node rendering — and runs off the same pass that re-renders nodes.
func (r *Reconciler) reconcileNERStatuses(ctx context.Context, logger logr.Logger) {
	ners := &deckhousev1alpha1.NodeExtensionRequestList{}
	if err := r.Client.List(ctx, ners); err != nil {
		logger.Error(err, "cannot list NodeExtensionRequests for status")
		return
	}
	if len(ners.Items) == 0 {
		return
	}

	conflicts := resolveNERConflicts(orderedNERs(ners.Items))
	groups, err := r.immutableNodeGroupNames(ctx)
	if err != nil {
		logger.Error(err, "cannot report NodeExtensionRequest statuses")
		return
	}

	// What the nodes made of these requests. A read failure is logged and the
	// statuses are still written without the counts: knowing that a request
	// resolved is worth publishing even when the fleet's answer is unavailable.
	outcomes, err := readNEROutcomes(ctx, r.Client)
	if err != nil {
		logger.Error(err, "cannot read what the nodes report about NodeExtensionRequests")
		outcomes = nil
	}

	for i := range ners.Items {
		if err := r.updateNERStatus(ctx, &ners.Items[i], conflicts, groups, outcomes[ners.Items[i].Name]); err != nil {
			logger.Error(err, "cannot update NodeExtensionRequest status", "request", ners.Items[i].Name)
		}
	}
}

// immutableNodeGroupNames lists the immutable NodeGroups a request can select.
// A listing failure is an error, not an empty answer: publishing "no groups
// matched" from a read that did not happen misleads every operator.
func (r *Reconciler) immutableNodeGroupNames(ctx context.Context) ([]string, error) {
	ngs := &v1.NodeGroupList{}
	if err := r.Client.List(ctx, ngs); err != nil {
		return nil, fmt.Errorf("list NodeGroups: %w", err)
	}
	var names []string
	for i := range ngs.Items {
		if ngs.Items[i].Spec.SystemType == v1.SystemTypeImmutable {
			names = append(names, ngs.Items[i].Name)
		}
	}
	return names, nil
}

// updateNERStatus computes and patches one request's status, skipping the write
// when nothing changed.
func (r *Reconciler) updateNERStatus(ctx context.Context, ner *deckhousev1alpha1.NodeExtensionRequest, conflicts map[string]nerConflict, immutableGroups []string, outcome nerOutcome) error {
	desired := ner.Status.DeepCopy()
	desired.ObservedGeneration = ner.Generation
	desired.MatchedNodeGroups = matchedNodeGroups(ner, immutableGroups)
	desired.AppliedNodes = outcome.applied
	desired.FailedNodes = outcome.failed
	desired.FailureMessage = outcome.message

	// The default is the request this controller refused, reason and message
	// already in hand.
	desired.Phase = phaseDegraded
	status := metav1.ConditionFalse
	reason, message := nerStatusReason(ner, conflicts)
	switch {
	case reason == "" && outcome.failed > 0:
		// It resolved here and the nodes refused it. Reporting Ready on the
		// strength of the resolution alone is what made a rejected extension
		// indistinguishable from a working one.
		reason = reasonRefusedByNodes
		message = fmt.Sprintf("%d node(s) refused the sysext, %d applied it: %s",
			outcome.failed, outcome.applied, outcome.message)
	case reason == "":
		desired.Phase = phaseReady
		status = metav1.ConditionTrue
		reason = reasonResolved
		message = fmt.Sprintf("the sysext resolved; %d node(s) report it applied", outcome.applied)
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
	patched := ner.DeepCopy()
	patched.Status = *desired
	if err := r.Client.Status().Patch(ctx, patched, client.MergeFrom(ner)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}
	return nil
}

// matchedNodeGroups returns the immutable NodeGroups the request selects: the
// listed names it intersects, or all of them when it names none.
func matchedNodeGroups(ner *deckhousev1alpha1.NodeExtensionRequest, immutableGroups []string) []string {
	names := ner.Spec.NodeGroupSelector.MatchNames
	var matched []string
	for _, name := range immutableGroups {
		if len(names) == 0 || slices.Contains(names, name) {
			matched = append(matched, name)
		}
	}
	slices.Sort(matched)
	return matched
}

// nerStatusReason returns the reason a request is not ready and a human message
// for its Ready condition, or empty strings when it resolved. An invalid sysext
// is reported first; otherwise a lost name/digest contest (resolveNERConflicts).
func nerStatusReason(ner *deckhousev1alpha1.NodeExtensionRequest, conflicts map[string]nerConflict) (string, string) {
	// reasonInvalidSysext is the only refusal resolveExtension has.
	if _, reason := resolveExtension(ner); reason != "" {
		return reason, "sysext must set both name and digest"
	}
	if conflict, lost := conflicts[ner.Name]; lost {
		return conflict.reason, conflictMessage(conflict)
	}
	return "", ""
}

// conflictMessage explains a lost uniqueness contest for the condition message.
func conflictMessage(conflict nerConflict) string {
	switch conflict.reason {
	case reasonReservedName:
		return "sysext name is reserved for a platform extension"
	case reasonConflict:
		return fmt.Sprintf("sysext %s already claimed by NodeExtensionRequest %q", conflict.field, conflict.winner)
	case reasonModuleConflict:
		return fmt.Sprintf("kernel module %q is already loaded with different parameters by NodeExtensionRequest %q", conflict.field, conflict.winner)
	default:
		return conflict.reason
	}
}
