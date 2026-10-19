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
	case awaitingEviction(op) && op.Status.DrainDeadline != nil:
		// Waiting for an eviction that was given until then, plus the margin
		// every stretch gets for the plumbing around it.
		return op.Status.DrainDeadline.Time.Add(operationTimeout)
	case op.Status.StartedAt != nil:
		// The node has the work and owes an answer.
		return op.Status.StartedAt.Time.Add(operationTimeout)
	case op.Status.Deadline != nil:
		// Pinned by the first write after the operation may go ahead, not by
		// its creation: an approval given an hour late must not start expired.
		return op.Status.Deadline.Time
	default:
		return time.Now().Add(operationTimeout)
	}
}

// publishedDeadline is the deadline the status shows: none while the operation
// waits for approval, which takes as long as it takes, and none once it ended.
func publishedDeadline(op *v1alpha1.NodeOperation) *metav1.Time {
	if terminal(op) || waitingForApproval(op) {
		return nil
	}
	deadline := metav1.NewTime(hardDeadline(op))
	return &deadline
}

// awaitingEviction reports what the operation is still waiting for. A Drain is
// the eviction, so it waits for one until it ends; any other operation only
// until the workload is gone and the node was handed the work.
func awaitingEviction(op *v1alpha1.NodeOperation) bool {
	return op.Spec.Type == v1alpha1.NodeOperationTypeDrain || op.Status.StartedAt == nil
}

// expire fails an operation that ran out of time and says so.
func (r *Reconciler) expire(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, deadline time.Time, logger logr.Logger) error {
	reason, message := timedOut(op, deadline, evictionFailure(op, node))
	return r.fail(ctx, op, reason, message, logger)
}

// timedOut names what the operation was still waiting for when the deadline
// passed, so the record says which node is holding the group up, and why when
// the draining controller said so.
func timedOut(op *v1alpha1.NodeOperation, deadline time.Time, evictionError string) (string, string) {
	when := deadline.UTC().Format(time.RFC3339)
	if !awaitingEviction(op) {
		return "NodeTimedOut", fmt.Sprintf("node %s did not report back by %s", op.Spec.NodeName, when)
	}
	if evictionError != "" {
		return "PreparationTimedOut", fmt.Sprintf("node %s was not drained by %s. Last eviction error: %s", op.Spec.NodeName, when, evictionError)
	}
	return "PreparationTimedOut", fmt.Sprintf("node %s was not drained by %s", op.Spec.NodeName, when)
}
