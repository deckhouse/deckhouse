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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
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

// drainTimeout is the bound the draining controller will use for this node's
// group, resolved the way it resolves it. Read once, when the eviction is asked
// for, and pinned from there on.
func (r *Reconciler) drainTimeout(ctx context.Context, node *corev1.Node) time.Duration {
	ngName := node.Labels[nodecommon.NodeGroupLabel]
	if ngName == "" {
		return defaultDrainTimeout
	}
	ng, err := nodecommon.GetNodeGroup(ctx, r.Client, ngName)
	if err != nil {
		log.FromContext(ctx).V(1).Info("could not read the group's drain timeout, using the default",
			"nodeGroup", ngName, "default", defaultDrainTimeout, "error", err.Error())
		return defaultDrainTimeout
	}
	return drainTimeoutOf(ng)
}

// drainTimeoutOf clamps as well as reads: an object stored before the CRD
// bounded nodeDrainTimeoutSecond can carry a value whose seconds-to-Duration
// multiplication overflows into a negative deadline.
func drainTimeoutOf(ng *v1.NodeGroup) time.Duration {
	if ng.Spec.NodeDrainTimeoutSecond == nil {
		return defaultDrainTimeout
	}
	seconds := int64(*ng.Spec.NodeDrainTimeoutSecond)
	if seconds <= 0 {
		return defaultDrainTimeout
	}
	if seconds > int64(maxDrainTimeout/time.Second) {
		return maxDrainTimeout
	}
	return time.Duration(seconds) * time.Second
}

// adoptDrainDeadline gives a parent the deadline its child eviction runs to;
// otherwise the parent fails out from under a still-running drain, which then
// finishes onto a node whose operation has already released it.
func (r *Reconciler) adoptDrainDeadline(ctx context.Context, op *v1alpha1.NodeOperation, deadline *metav1.Time) error {
	patch := client.MergeFromWithOptions(op.DeepCopy(), client.MergeFromWithOptimisticLock{})
	op.Status.DrainDeadline = deadline.DeepCopy()
	if err := r.Client.Status().Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("take over the eviction deadline of %s: %w", op.Name, err)
	}
	return nil
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
