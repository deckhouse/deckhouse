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
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	// Stale requests are looked for on every pass, not only while a disruption is requested: the
	// newer generation may need none. The cache rules them out, and the ones it finds are read again.
	if !disruptionRequested(nc) {
		cached, err := listApprovals(ctx, r.Client, nc)
		if err != nil || len(cached.stale) == 0 {
			return ctrl.Result{}, err
		}
	}
	approvals, err := r.approvalsFor(ctx, nc)
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, op := range approvals.stale {
		if err := r.deleteStale(ctx, nc, op, approvals.current, logger); err != nil {
			return ctrl.Result{}, err
		}
	}

	if !disruptionRequested(nc) {
		return ctrl.Result{}, nil
	}

	// An operation in flight needs nothing. Nor does a node whose machine is being deleted: an
	// interruption would only be ended at once with MachineDeleted.
	if approvals.current != nil || approvals.machineDeleted {
		return ctrl.Result{}, nil
	}

	if approvals.failures >= nodecommon.MaxDisruptionAttempts {
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
	current *v1alpha1.NodeOperation
	// failures counts the failed attempts. A completed one is a window the node released,
	// with the revision applied or not, and never counts.
	failures int
	// lastFailed is the newest attempt when it failed, whose message the request statuses
	// carry. Nil once a completed one follows it: the next attempt is a new window.
	lastFailed *v1alpha1.NodeOperation
	// stale are the requests left unapproved for older revisions.
	stale []*v1alpha1.NodeOperation
	// machineDeleted: the node has the Drain of a deleted Cluster API Machine, see
	// internal/controller/machinedrain.
	machineDeleted bool
}

// retryIn is what is left of the backoff after the newest failed attempt. Each
// attempt waits longer than the one before: a disruption that failed once fails
// the same way at once, and every try drains the node again for nothing. After a
// completed attempt the next one comes at once.
func (s approvalState) retryIn() time.Duration {
	if s.lastFailed == nil {
		return 0
	}
	return time.Until(s.retryAt())
}

// retryAt is when the next attempt is created: the backoff after the newest failed one.
func (s approvalState) retryAt() time.Time {
	return v1alpha1.NodeOperationFinishedAt(s.lastFailed).Add(nodecommon.DisruptionRetryBackoff(s.failures))
}

// approvalsFor reads the approvals past the cache: a cached list that missed the previous
// approval would mint a second one for the same revision.
func (r *Reconciler) approvalsFor(ctx context.Context, nc *internalv1alpha1.NodeConfig) (approvalState, error) {
	return listApprovals(ctx, r.sources.Reader, nc)
}

// listApprovals reads the operations that cover this revision, and the stale ones of earlier
// revisions. From the cache it needs the spec.nodeName index the nodeoperation controller adds.
func listApprovals(ctx context.Context, reader client.Reader, nc *internalv1alpha1.NodeConfig) (approvalState, error) {
	ops := &v1alpha1.NodeOperationList{}
	if err := reader.List(ctx, ops, client.MatchingFields{v1alpha1.NodeOperationNodeNameField: nc.Name}); err != nil {
		return approvalState{}, fmt.Errorf("list NodeOperations of %s: %w", nc.Name, err)
	}

	var state approvalState
	var lastCompletedAt time.Time
	for i := range ops.Items {
		op := &ops.Items[i]
		if _, ofMachine := op.Labels[v1alpha1.NodeOperationMachineUIDLabel]; ofMachine {
			state.machineDeleted = true
		}
		if op.Spec.Type != v1alpha1.NodeOperationTypeDisruption {
			continue
		}
		if !v1alpha1.DisruptionOf(op, nc.UID, nc.Generation) {
			if outdated(op, nc) {
				state.stale = append(state.stale, op)
			}
			continue
		}
		switch op.Status.Phase {
		case v1alpha1.NodeOperationPhaseCompleted:
			if finished := v1alpha1.NodeOperationFinishedAt(op); finished.After(lastCompletedAt) {
				lastCompletedAt = finished
			}
		case v1alpha1.NodeOperationPhaseFailed:
			// Counted by FailedDisruptions below.
		default:
			state.current = op
		}
	}
	failed := v1alpha1.FailedDisruptions(ops.Items, nc.UID, nc.Generation)
	state.failures = len(failed)
	if len(failed) > 0 && v1alpha1.NodeOperationFinishedAt(failed[len(failed)-1]).After(lastCompletedAt) {
		state.lastFailed = failed[len(failed)-1]
	}
	return state, nil
}

// outdated reports a request nobody approved for an older revision of this
// NodeConfig that has not finished. An approved one is never touched: the drain
// for it may already run.
func outdated(op *v1alpha1.NodeOperation, nc *internalv1alpha1.NodeConfig) bool {
	if ptr.Deref(op.Spec.Approved, false) {
		return false
	}
	if op.Status.Phase == v1alpha1.NodeOperationPhaseCompleted || op.Status.Phase == v1alpha1.NodeOperationPhaseFailed {
		return false
	}
	if op.Spec.NodeConfigUID != nc.UID || op.Spec.ConfigGeneration == nil {
		return false
	}
	return *op.Spec.ConfigGeneration < nc.Generation
}

// deleteStale deletes an outdated request: approved later, it would drain the node for a
// revision the NodeConfig no longer holds. Deleted rather than failed, which stays listed next
// to the current request; the preconditions let an operator approving it at this moment win.
func (r *Reconciler) deleteStale(ctx context.Context, nc *internalv1alpha1.NodeConfig, op, newer *v1alpha1.NodeOperation, logger logr.Logger) error {
	err := r.Client.Delete(ctx, op, client.Preconditions{UID: &op.UID, ResourceVersion: &op.ResourceVersion})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if apierrors.IsConflict(err) {
		logger.V(1).Info("left a stale request that changed after it was read", "node", nc.Name, "operation", op.Name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete the stale NodeOperation %s: %w", op.Name, err)
	}

	message := fmt.Sprintf("NodeOperation %s for generation %d is deleted: the NodeConfig moved to generation %d before it was approved",
		op.Name, ptr.Deref(op.Spec.ConfigGeneration, 0), nc.Generation)
	if newer != nil {
		message += fmt.Sprintf(". NodeOperation %s requests approval for generation %d", newer.Name, nc.Generation)
	}
	logger.Info("deleted an unapproved request for an older NodeConfig generation",
		"node", nc.Name, "configGeneration", nc.Generation, "operation", op.Name)
	r.Recorder.Event(nc, corev1.EventTypeNormal, "DisruptionRequestSuperseded", message)
	return nil
}

// recordApprovalExhausted says the cluster has given up interrupting this node
// for this revision. Reported on both objects: the operator watching the group
// and the one looking at the node's configuration are not the same person.
func (r *Reconciler) recordApprovalExhausted(ng *v1.NodeGroup, nc *internalv1alpha1.NodeConfig, failures int, logger logr.Logger) {
	logger.Info("stopped asking to interrupt the node",
		"node", nc.Name, "nodeGroup", ng.Name, "configGeneration", nc.Generation, "failedAttempts", failures)
	message := fmt.Sprintf("%d attempts at generation %d failed on node %s. No new NodeOperation is created until the configuration changes or the oldest attempt is deleted, %d hours after it finished",
		failures, nc.Generation, nc.Name, int(nodecommon.NodeOperationRetention.Hours()))
	r.Recorder.Event(ng, corev1.EventTypeWarning, disruptionApprovalExhaustedEvent, message)
	r.Recorder.Event(nc, corev1.EventTypeWarning, disruptionApprovalExhaustedEvent, message)
}

// createApproval answers the node at once in either mode: Automatic approves on
// creation, Manual leaves the request in the API for an operator to approve.
func (r *Reconciler) createApproval(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, nc *internalv1alpha1.NodeConfig, logger logr.Logger) error {
	approved := v1.DisruptionApprovalMode(ua.GetApprovalMode(ng)) != v1.DisruptionApprovalModeManual
	// needDrain asks about the Deckhouse pod only below two Ready nodes.
	runsDeckhouse := false
	if ng.Status.Ready < 2 {
		found, err := r.runsDeckhouse(ctx, node.Name)
		if err != nil {
			return err
		}
		runsDeckhouse = found
	}
	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			// Generated rather than derived from the generation: a retry after
			// a failed attempt needs a name of its own, and the history of what
			// was tried is worth keeping.
			GenerateName: fmt.Sprintf("disruption-%s-", nc.Name),
			Labels: map[string]string{
				nodecommon.NodeGroupLabel: ng.Name,
				managedByLabel:            managedByValue,
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
			Type:             v1alpha1.NodeOperationTypeDisruption,
			NodeName:         nc.Name,
			ConfigGeneration: ptr.To(nc.Generation),
			NodeConfigUID:    nc.UID,
			Approved:         ptr.To(approved),
			Drain:            &v1alpha1.NodeOperationDrainSpec{Skip: !needDrain(ng, runsDeckhouse)},
			Cause:            disruptionCause(nc),
		},
	}
	// No IsAlreadyExists branch: the name is generated, and the API server
	// retries the generation itself until it lands on a free one.
	if err := r.Client.Create(ctx, op); err != nil {
		return fmt.Errorf("ask for a disruption of %s: %w", nc.Name, err)
	}
	logger.Info("asked to interrupt the node for its new config",
		"node", nc.Name, "nodeGroup", ng.Name, "configGeneration", nc.Generation, "operation", op.Name, "approved", approved)
	message := fmt.Sprintf("Created NodeOperation %s to interrupt node %s", op.Name, nc.Name)
	if !approved {
		message += ". Waiting for an operator to set spec.approved to true"
	}
	r.Recorder.Event(ng, corev1.EventTypeNormal, "DisruptionRequested", message)
	return nil
}

// disruptionRequested reports whether the agent is waiting for permission to
// interrupt the node, for the config revision it currently has. Only
// False/ApprovalPending asks: the condition is True at rest.
func disruptionRequested(nc *internalv1alpha1.NodeConfig) bool {
	reason, current := nodecommon.DisruptionApprovalWithheld(nc)
	return current && reason == internalv1alpha1.NodeConfigReasonApprovalPending
}

// attemptsExhausted reports a node node-manager has stopped interrupting for the revision it
// holds. It is not a request, but its attempts are still why it refuses that revision.
func attemptsExhausted(nc *internalv1alpha1.NodeConfig) bool {
	reason, current := nodecommon.DisruptionApprovalWithheld(nc)
	return current && reason == internalv1alpha1.NodeConfigReasonAttemptsExhausted
}

// approvalRequestPrefix opens the message of a node waiting for approval. Mirrors
// holdUntilApproved in internal/reconcile/loop.go of deckhouse/container-base/nodelet.
const approvalRequestPrefix = "waiting for approval to interrupt this node: "

// maxCause is the maxLength of spec.cause in crds/nodeoperation.yaml.
const maxCause = 1024

// disruptionCause is why the node asks to be interrupted, in its own words. Cut in
// bytes: a cause longer than the CRD allows would fail every create.
func disruptionCause(nc *internalv1alpha1.NodeConfig) string {
	cond := meta.FindStatusCondition(nc.Status.Conditions, internalv1alpha1.NodeConfigConditionTypeDisruptionApproved)
	if cond == nil {
		return ""
	}
	cause := strings.TrimPrefix(cond.Message, approvalRequestPrefix)
	// nodelet appends when the next attempt is requested, which is over once the attempt exists.
	cause, _, _ = strings.Cut(cause, "; Attempt ")
	return nodecommon.Truncate(cause, maxCause)
}

// runsDeckhouse reports whether the Deckhouse pod runs on the node. Read past the cache: the
// controller watches no pods, and this read happens once per disruption request.
func (r *Reconciler) runsDeckhouse(ctx context.Context, nodeName string) (bool, error) {
	pods := &corev1.PodList{}
	if err := r.sources.Reader.List(ctx, pods, client.InNamespace(d8SystemNS), client.MatchingLabels{"app": "deckhouse"}); err != nil {
		return false, fmt.Errorf("list the Deckhouse pods: %w", err)
	}
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == nodeName {
			return true, nil
		}
	}
	return false, nil
}

// needDrain mirrors NeedDrainNode in internal/controller/updateapproval/engine/processor.go:
// only a lone control-plane node, or the node of the Deckhouse pod with no other Ready node in
// its group, is interrupted without a drain. Any other group is drained, a group of one included.
func needDrain(ng *v1.NodeGroup, runsDeckhouse bool) bool {
	if ng.Name == "master" && ng.Status.Nodes == 1 {
		return false
	}
	if runsDeckhouse && ng.Status.Ready < 2 {
		return false
	}
	if ng.Spec.Disruptions == nil || ng.Spec.Disruptions.Automatic == nil {
		return true
	}
	return ptr.Deref(ng.Spec.Disruptions.Automatic.DrainBeforeApproval, true)
}
