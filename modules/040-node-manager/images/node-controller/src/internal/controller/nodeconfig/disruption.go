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
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
)

// reconcileDisruption answers a node that cannot apply its config without a
// restart, by creating a NodeOperation — the same resource an operator uses.
// The operation names the config revision, so it authorises one change only.
func (r *Reconciler) reconcileDisruption(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, nc *internalv1alpha1.NodeConfig, logger logr.Logger) (ctrl.Result, error) {
	if !disruptionRequested(nc) {
		return ctrl.Result{}, nil
	}

	approvals, err := r.approvalsFor(ctx, nc)
	if err != nil {
		return ctrl.Result{}, err
	}
	if approvals.current != nil {
		// An in-flight operation needs nothing. A completed one means the node is
		// asking again for a revision already carried out — it will be refused
		// forever, so surface it instead of silently holding the rollout slot.
		if approvals.current.Status.Phase == v1alpha1.NodeOperationPhaseCompleted {
			logger.V(1).Info("node is asking again for a disruption already carried out",
				"node", node.Name, "nodeGroup", ng.Name, "configGeneration", nc.Generation, "operation", approvals.current.Name)
			r.Recorder.Event(ng, corev1.EventTypeWarning, "DisruptionAlreadyDone",
				fmt.Sprintf("Node %s is still asking to be interrupted for config generation %d, which NodeOperation %s already completed",
					node.Name, nc.Generation, approvals.current.Name))
		}
		return ctrl.Result{}, nil
	}

	if v1.DisruptionApprovalMode(ua.GetApprovalMode(ng)) == v1.DisruptionApprovalModeManual {
		logger.Info("node needs a disruption an operator has to approve",
			"node", node.Name, "nodeGroup", ng.Name, "configGeneration", nc.Generation)
		r.Recorder.Event(ng, corev1.EventTypeNormal, "DisruptionRequired",
			fmt.Sprintf("Node %s is waiting for a NodeOperation of type ApproveDisruption for config generation %d",
				node.Name, nc.Generation))
		return ctrl.Result{}, nil
	}

	if approvals.failures >= maxDisruptionAttempts {
		r.recordApprovalExhausted(ng, nc, approvals.failures, logger)
		return ctrl.Result{}, nil
	}
	// Nothing else wakes this controller when the wait runs out: the node's
	// status is unchanged, and the failed operation is not watched here.
	if wait := approvals.retryIn(); wait > 0 {
		logger.V(1).Info("waiting before asking again to interrupt the node",
			"node", node.Name, "nodeGroup", ng.Name, "configGeneration", nc.Generation,
			"failedAttempts", approvals.failures, "retryIn", wait)
		return ctrl.Result{RequeueAfter: wait}, nil
	}

	return ctrl.Result{}, r.createApproval(ctx, ng, node, nc, logger)
}

// approvalState is what the API server knows about one revision's approvals:
// the operation that still stands, and the failures that came before it.
type approvalState struct {
	current      *v1alpha1.NodeOperation
	failures     int
	lastFailedAt time.Time
}

// retryIn is what is left of the backoff after the newest failed attempt. Each
// attempt waits longer than the one before: a disruption that failed once fails
// the same way at once, and every try drains the node again for nothing.
func (s approvalState) retryIn() time.Duration {
	if s.failures == 0 {
		return 0
	}
	backoff := min(disruptionRetryBackoff<<(s.failures-1), disruptionRetryBackoffMax)
	return backoff - time.Since(s.lastFailedAt)
}

// approvalsFor reads the operations that cover this revision. The read goes
// straight to the API server: a cached list that missed the previous approval
// would mint a second one for the same revision.
func (r *Reconciler) approvalsFor(ctx context.Context, nc *internalv1alpha1.NodeConfig) (approvalState, error) {
	ops := &v1alpha1.NodeOperationList{}
	if err := r.sources.Reader.List(ctx, ops, client.MatchingLabels{
		v1alpha1.NodeOperationNodeLabel: nc.Name,
		nodeConfigUIDLabel:              string(nc.UID),
	}); err != nil {
		return approvalState{}, fmt.Errorf("list NodeOperations of %s: %w", nc.Name, err)
	}

	var state approvalState
	for i := range ops.Items {
		op := &ops.Items[i]
		if op.Spec.Type != v1alpha1.NodeOperationTypeApproveDisruption || op.Spec.NodeName != nc.Name {
			continue
		}
		if op.Spec.ConfigGeneration == nil || *op.Spec.ConfigGeneration != nc.Generation {
			continue
		}
		// A failed operation is an attempt spent, not one in flight. A completed
		// one is kept: it stops a second approval, and a second drain, while the
		// node is still applying the config and has not cleared DisruptionRequired.
		if op.Status.Phase == v1alpha1.NodeOperationPhaseFailed {
			state.failures++
			state.lastFailedAt = later(state.lastFailedAt, failedAt(op))
			continue
		}
		state.current = op
	}
	return state, nil
}

// failedAt is when an operation ended. The stamp is written a moment after the
// phase, so a failure the operation controller has not stamped yet counts from
// when it was created rather than from nothing.
func failedAt(op *v1alpha1.NodeOperation) time.Time {
	if op.Status.FinishedAt != nil {
		return op.Status.FinishedAt.Time
	}
	return op.CreationTimestamp.Time
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// recordApprovalExhausted says the cluster has given up interrupting this node
// for this revision. Reported on both objects: the operator watching the group
// and the one looking at the node's configuration are not the same person.
func (r *Reconciler) recordApprovalExhausted(ng *v1.NodeGroup, nc *internalv1alpha1.NodeConfig, failures int, logger logr.Logger) {
	logger.Info("stopped asking to interrupt the node",
		"node", nc.Name, "nodeGroup", ng.Name, "configGeneration", nc.Generation, "failedAttempts", failures)
	message := fmt.Sprintf("Gave up interrupting node %s for config generation %d after %d failed NodeOperations; nothing more is created until the configuration changes",
		nc.Name, nc.Generation, failures)
	r.Recorder.Event(ng, corev1.EventTypeWarning, disruptionApprovalExhaustedEvent, message)
	r.Recorder.Event(nc, corev1.EventTypeWarning, disruptionApprovalExhaustedEvent, message)
}

func (r *Reconciler) createApproval(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, nc *internalv1alpha1.NodeConfig, logger logr.Logger) error {
	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			// Generated rather than derived from the generation: a retry after
			// a failed attempt needs a name of its own, and the history of what
			// was tried is worth keeping.
			GenerateName: fmt.Sprintf("approve-%s-", nc.Name),
			Labels: map[string]string{
				nodecommon.NodeGroupLabel:       ng.Name,
				managedByLabel:                  managedByValue,
				v1alpha1.NodeOperationNodeLabel: nc.Name,
				nodeConfigUIDLabel:              string(nc.UID),
			},
			// Owned by the node: when the node goes, so does the record of what
			// was done to it, instead of accumulating forever.
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Node",
				Name:       node.Name,
				UID:        node.UID,
			}},
		},
		Spec: v1alpha1.NodeOperationSpec{
			Type:             v1alpha1.NodeOperationTypeApproveDisruption,
			NodeName:         nc.Name,
			ConfigGeneration: ptr.To(nc.Generation),
			Drain:            &v1alpha1.NodeOperationDrainSpec{Skip: !needDrain(ng)},
		},
	}
	// No IsAlreadyExists branch: the name is generated, and the API server
	// retries the generation itself until it lands on a free one.
	if err := r.Client.Create(ctx, op); err != nil {
		return fmt.Errorf("ask for a disruption of %s: %w", nc.Name, err)
	}
	logger.Info("asked to interrupt the node for its new config",
		"node", nc.Name, "nodeGroup", ng.Name, "configGeneration", nc.Generation, "operation", op.Name)
	r.Recorder.Event(ng, corev1.EventTypeNormal, "DisruptionRequested",
		fmt.Sprintf("Created NodeOperation %s to interrupt node %s", op.Name, nc.Name))
	return nil
}

// disruptionRequested reports whether the agent is waiting for permission to
// interrupt the node, for the config revision it currently has.
func disruptionRequested(nc *internalv1alpha1.NodeConfig) bool {
	cond := meta.FindStatusCondition(nc.Status.Conditions, disruptionRequiredCondition)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		return false
	}
	return cond.ObservedGeneration == nc.Generation
}

// needDrain mirrors the update-approval rule: a group of exactly one is
// interrupted without a drain. Not "one or fewer" — status.nodes is 0 until its
// controller runs, and reading that as "one" skipped drains on whole groups.
func needDrain(ng *v1.NodeGroup) bool {
	if ng.Status.Nodes == 1 {
		return false
	}
	if ng.Spec.Disruptions == nil || ng.Spec.Disruptions.Automatic == nil {
		return true
	}
	return ptr.Deref(ng.Spec.Disruptions.Automatic.DrainBeforeApproval, true)
}
