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

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/domain/fsm"
)

// writeStatus publishes the part of the status the controller owns: the phase,
// the conditions and observedGeneration. The failed and fallback sections belong
// to the agents, so the merge patch is built from a copy and carries the changed
// fields only.
func (r *Reconciler) writeStatus(
	ctx context.Context,
	incident *v1alpha1.FencingFailedNodeState,
	state fsm.State,
	condition metav1.Condition,
) error {
	updated := incident.DeepCopy()

	// StateHealthy is not a phase of a live object: the object exists, so the
	// machine is still at its entry state and has nothing to publish yet.
	if state != fsm.StateHealthy {
		updated.Status.Phase = state.Phase()
	}

	updated.Status.ObservedGeneration = incident.Generation
	meta.SetStatusCondition(&updated.Status.Conditions, condition)

	if equality.Semantic.DeepEqual(incident.Status, updated.Status) {
		return nil
	}

	if err := r.client.Status().Patch(ctx, updated, client.MergeFrom(incident)); err != nil {
		return fmt.Errorf("patch status of fencingfailednodestate %q: %w", incident.Name, err)
	}

	return nil
}
