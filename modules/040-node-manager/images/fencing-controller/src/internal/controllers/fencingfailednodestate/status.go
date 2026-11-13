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

package fencingfailednodestate

import (
	"context"
	"fmt"

	equality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
)

// writeStatus writes the part of the status that the controller owns: the
// phase and the conditions. The agents own the attestation, contact and
// fallback sections. Thus the merge patch has the changed fields only.
//
// The patch has the resourceVersion of the object that the controller read. If
// another writer changed the object after the read, for example the node wrote
// a heartbeat, the API server rejects the patch with a conflict. Then the
// controller decides again on the new version. After a write, incident is the
// written object, and the metric and the events show its phase. The event of
// a new phase adds why, when it is not empty.
func (r *Reconciler) writeStatus(
	ctx context.Context,
	incident *v1alpha1.FencingFailedNodeState,
	phase v1alpha1.FencingFailedNodeStatePhase,
	why string,
	conditions ...metav1.Condition,
) error {
	updated := incident.DeepCopy()
	updated.Status.Phase = phase

	for _, condition := range conditions {
		meta.SetStatusCondition(&updated.Status.Conditions, condition)
	}

	// The condition Progressing tells what stops the eviction. Out of
	// Evicting the eviction is complete or did not start.
	if phase != v1alpha1.PhaseEvicting {
		meta.RemoveStatusCondition(&updated.Status.Conditions, common.ConditionTypeProgressing)
	}

	if equality.Semantic.DeepEqual(incident.Status, updated.Status) {
		return nil
	}

	patch := client.MergeFromWithOptions(incident, client.MergeFromWithOptimisticLock{})
	if err := callAPI(ctx, func(ctx context.Context) error {
		return r.client.Status().Patch(ctx, updated, patch)
	}); err != nil {
		return fmt.Errorf("patch status of fencingfailednodestate %q: %w", incident.Name, err)
	}

	if phase != incident.Status.Phase {
		logf.FromContext(ctx).Info("phase of fencingfailednodestate changed",
			"node", incident.Name, "from", string(incident.Status.Phase), "to", string(phase))
		r.announcePhase(incident, incident.Status.Phase, phase, why)
	}

	*incident = *updated

	reportPhase(incident)
	reportProgress(incident)

	return nil
}
