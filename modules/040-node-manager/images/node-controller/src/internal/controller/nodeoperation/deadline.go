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

package nodeoperation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

// hardDeadline is the moment this operation runs out of time. Read off the
// operation alone, never recomputed from the cluster: a later NodeGroup edit
// or deletion must not cut short a drain that is still legitimately running.
func hardDeadline(op *v1alpha1.NodeOperation) time.Time {
	switch {
	case op.Status.StartedAt != nil:
		// The node has the work and owes an answer.
		return op.Status.StartedAt.Time.Add(operationTimeout)
	case op.Status.DrainDeadline != nil:
		// Waiting for an eviction that was given until then, plus the margin
		// every stretch gets for the plumbing around it.
		return op.Status.DrainDeadline.Time.Add(operationTimeout)
	default:
		return op.CreationTimestamp.Time.Add(operationTimeout)
	}
}

// adoptDrainDeadline gives a parent the deadline its child eviction runs to;
// otherwise the parent fails out from under a still-running drain, which then
// finishes onto a node whose operation has already released it.
func (r *Reconciler) adoptDrainDeadline(ctx context.Context, op *v1alpha1.NodeOperation, deadline *metav1.Time) error {
	return r.patchStatus(ctx, op, fmt.Sprintf("take over the eviction deadline of %s", op.Name), func() {
		op.Status.DrainDeadline = deadline.DeepCopy()
	})
}

// expire fails an operation that ran out of time and says so. Announced only
// once the phase is actually written: an event for a transition a conflict
// rolled back would be a lie, and the retry would emit it again.
func (r *Reconciler) expire(ctx context.Context, op *v1alpha1.NodeOperation, deadline time.Time, logger logr.Logger) error {
	reason, message := timedOut(op, deadline)
	if err := r.fail(ctx, op, reason, message, logger); err != nil {
		return err
	}
	r.Recorder.Event(op, corev1.EventTypeWarning, reason, message)
	return nil
}

// timedOut names what the operation was still waiting for when the deadline
// passed, so the record says which node is holding the group up.
func timedOut(op *v1alpha1.NodeOperation, deadline time.Time) (string, string) {
	when := deadline.UTC().Format(time.RFC3339)
	if op.Status.StartedAt != nil {
		return "NodeTimedOut", fmt.Sprintf("node %s did not report back by %s", op.Spec.NodeName, when)
	}
	return "PreparationTimedOut", fmt.Sprintf("node %s was not prepared by %s: its workload has not finished leaving", op.Spec.NodeName, when)
}
