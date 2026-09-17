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

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
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
	groups, err := r.allNodeGroupNames(ctx)
	if err != nil {
		return err
	}

	// What the fleet made of these pods, from the two places the answer can live.
	// Nothing is published without both: counts absent read as counts zero, so a
	// transient read failure would turn "every node refused it" into a clean
	// Ready and lose the reason with it.
	outcomes, err := readNodeConfigOutcomes(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("read what the Engine nodes report about NodeStaticPodRequests: %w", err)
	}

	// The bashible half. Removable as a unit: these lines go together with
	// nsprapplied_annotation.go when the last mutable NodeGroup is gone, and
	// mergeOutcomes keeps working with one source. The immutable groups are read
	// here because that source is the only thing left that needs them.
	immutable, err := r.immutableNodeGroupNames(ctx)
	if err != nil {
		return err
	}
	fromAnnotations, err := readAnnotationOutcomes(ctx, r.Client, immutable)
	if err != nil {
		return fmt.Errorf("read what the bashible nodes report about NodeStaticPodRequests: %w", err)
	}
	outcomes = mergeOutcomes(outcomes, fromAnnotations)

	// Iterated in contest order so a reader of the log sees the winner before the
	// objects that lost to it.
	for _, nspr := range ordered {
		if err := r.updateNSPRStatus(ctx, nspr, rejected, groups, outcomes[nspr.Name]); err != nil {
			logger.Error(err, "cannot update NodeStaticPodRequest status", "staticPodRequest", nspr.Name)
		}
	}
	return nil
}

// updateNSPRStatus computes and patches one object's status, skipping the write
// when nothing changed.
func (r *Reconciler) updateNSPRStatus(ctx context.Context, nspr *deckhousev1alpha1.NodeStaticPodRequest, rejected map[string]nsprRefusal, nodeGroups []string, outcome nsprOutcome) error {
	desired := nspr.Status.DeepCopy()
	desired.ObservedGeneration = nspr.Generation
	desired.MatchedNodeGroups = matchedNodeGroups(nspr.Spec.NodeGroupSelector.MatchNames, nodeGroups)
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
	switch {
	case reason == "" && outcome.failed > 0:
		reason = reasonRefusedByNodes
		message = fmt.Sprintf("%d node(s) refused the static pod, %d wrote it: %s",
			outcome.failed, outcome.applied, outcome.message)
	case reason == "":
		desired.Phase = phaseReady
		status = metav1.ConditionTrue
		reason = reasonResolved
		message = fmt.Sprintf("the static pod resolved; %d node(s) report it written", outcome.applied)
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
