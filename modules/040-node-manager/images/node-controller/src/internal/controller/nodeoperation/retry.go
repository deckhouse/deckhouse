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

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// recordRetry writes Retry on a failed Disruption; a completed one is never retried. Read from the
// cache: the watches on the other attempts and on the NodeConfig bring it back once it caught up.
// It returns when the next attempt is requested, zero unless node-manager waits to make one.
func (r *Reconciler) recordRetry(ctx context.Context, op *v1alpha1.NodeOperation) (time.Time, error) {
	if op.Status.Phase != v1alpha1.NodeOperationPhaseFailed {
		return time.Time{}, nil
	}
	nc, err := nodecommon.GetNodeConfig(ctx, r.Client, op.Spec.NodeName)
	if err != nil {
		return time.Time{}, err
	}
	ops, err := operationsOfNode(ctx, r.Client, op.Spec.NodeName)
	if err != nil {
		return time.Time{}, fmt.Errorf("list the operations of %s: %w", op.Spec.NodeName, err)
	}

	retry, requestedAt := retryCondition(op, ops, nc)
	retry.ObservedGeneration = op.Generation
	err = r.patchStatus(ctx, op, fmt.Sprintf("record whether %s is retried", op.Name), func() {
		meta.SetStatusCondition(&op.Status.Conditions, retry)
	})
	return requestedAt, err
}

// retryCondition is Retry of op, read off the attempts of its revision on the node and the
// node's NodeConfig. The time is when the next attempt is requested, set for Scheduled only.
func retryCondition(op *v1alpha1.NodeOperation, ops []v1alpha1.NodeOperation, nc *internalv1alpha1.NodeConfig) (metav1.Condition, time.Time) {
	generation := ptr.Deref(op.Spec.ConfigGeneration, 0)
	revision := []v1alpha1.NodeOperation{*op}
	var next *v1alpha1.NodeOperation
	for i := range ops {
		other := &ops[i]
		if other.Name == op.Name || !v1alpha1.DisruptionOf(other, op.Spec.NodeConfigUID, generation) {
			continue
		}
		revision = append(revision, *other)
		if createdAfter(other, op) && (next == nil || createdAfter(next, other)) {
			next = other
		}
	}
	attempts := v1alpha1.FailedDisruptions(revision, op.Spec.NodeConfigUID, generation)

	if next != nil {
		number := 1
		for _, attempt := range attempts {
			if createdAfter(next, attempt) {
				number++
			}
		}
		return retryWith(metav1.ConditionTrue, reasonRetryCreated,
			fmt.Sprintf("Attempt %d of %d is NodeOperation %s", number, nodecommon.MaxDisruptionAttempts, next.Name)), time.Time{}
	}

	if nc == nil || nc.UID != op.Spec.NodeConfigUID {
		return retryWith(metav1.ConditionFalse, reasonRetrySuperseded,
			fmt.Sprintf("The NodeConfig of node %s is no longer the one this operation was for", op.Spec.NodeName)), time.Time{}
	}
	if nc.Generation != generation {
		return retryWith(metav1.ConditionFalse, reasonRetrySuperseded,
			fmt.Sprintf("The NodeConfig moved to generation %d. node-manager makes no new attempt for generation %d", nc.Generation, generation)), time.Time{}
	}
	// The nodeconfig controller makes the next attempt only for a node that asks for it.
	if !stillAsks(nc) {
		return retryWith(metav1.ConditionFalse, reasonRetryNotRequested,
			fmt.Sprintf("The node no longer asks to be interrupted for generation %d. node-manager makes no new attempt", generation)), time.Time{}
	}

	count := len(attempts)
	if count >= nodecommon.MaxDisruptionAttempts {
		// The count drops below the limit once this one is collected: the oldest, unless an
		// operator added attempts past the limit.
		collected := attempts[count-nodecommon.MaxDisruptionAttempts]
		return retryWith(metav1.ConditionFalse, reasonRetryAttemptsExhausted,
			fmt.Sprintf("%d of %d attempts for generation %d failed. node-manager makes no new attempt until the NodeConfig changes, or until %s, when NodeOperation %s is deleted",
				count, nodecommon.MaxDisruptionAttempts, generation,
				v1alpha1.NodeOperationFinishedAt(collected).Add(nodecommon.NodeOperationRetention).UTC().Format(time.RFC3339), collected.Name)), time.Time{}
	}

	// The same wait the nodeconfig controller sits out before it creates the next attempt.
	requestedAt := v1alpha1.NodeOperationFinishedAt(attempts[count-1]).Add(nodecommon.DisruptionRetryBackoff(count))
	return retryWith(metav1.ConditionTrue, reasonRetryScheduled,
		fmt.Sprintf("Attempt %d of %d is requested at %s", count+1, nodecommon.MaxDisruptionAttempts, requestedAt.UTC().Format(time.RFC3339))), requestedAt
}

func retryWith(status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{Type: conditionRetry, Status: status, Reason: reason, Message: message}
}

func createdAfter(a, b *v1alpha1.NodeOperation) bool {
	return a.CreationTimestamp.After(b.CreationTimestamp.Time)
}

// stillAsks reports a node that asks to be interrupted, or that reports AttemptsExhausted
// because node-manager stopped answering its request.
func stillAsks(nc *internalv1alpha1.NodeConfig) bool {
	reason, _ := nodecommon.DisruptionApprovalWithheld(nc)
	return reason == internalv1alpha1.NodeConfigReasonApprovalPending || reason == internalv1alpha1.NodeConfigReasonAttemptsExhausted
}

// failedDisruptionsOn names the failed Disruptions of a node, whose Retry a change of their
// siblings or of the node's NodeConfig can move.
func (r *Reconciler) failedDisruptionsOn(ctx context.Context, nodeName string) []reconcile.Request {
	ops := &v1alpha1.NodeOperationList{}
	if err := r.Client.List(ctx, ops, client.MatchingFields{v1alpha1.NodeOperationNodeNameField: nodeName}); err != nil {
		log.FromContext(ctx).Error(err, "list the operations of a node whose disruption attempts changed", "node", nodeName)
		return nil
	}
	var requests []reconcile.Request
	for i := range ops.Items {
		op := &ops.Items[i]
		if op.Spec.Type == v1alpha1.NodeOperationTypeDisruption && op.Status.Phase == v1alpha1.NodeOperationPhaseFailed {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: op.Name}})
		}
	}
	return requests
}

// retryInputChanged reports whether a NodeConfig update touched what Retry is read from: the
// generation and the DisruptionApproved condition.
func retryInputChanged(before, after client.Object) bool {
	oldConfig, okOld := before.(*internalv1alpha1.NodeConfig)
	newConfig, okNew := after.(*internalv1alpha1.NodeConfig)
	if !okOld || !okNew {
		return true
	}
	if oldConfig.Generation != newConfig.Generation {
		return true
	}
	return !equality.Semantic.DeepEqual(
		meta.FindStatusCondition(oldConfig.Status.Conditions, internalv1alpha1.NodeConfigConditionTypeDisruptionApproved),
		meta.FindStatusCondition(newConfig.Status.Conditions, internalv1alpha1.NodeConfigConditionTypeDisruptionApproved))
}
