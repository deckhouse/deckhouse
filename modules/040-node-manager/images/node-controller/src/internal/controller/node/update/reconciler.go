/*
Copyright 2025 Flant JSC

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

package update

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/drain"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	nodeGroupLabel = "node.deckhouse.io/group"

	annotationWaitingForApproval = "update.node.deckhouse.io/waiting-for-approval"
	annotationApproved           = "update.node.deckhouse.io/approved"
	annotationDisruptionRequired = "update.node.deckhouse.io/disruption-required"
	annotationDisruptionApproved = "update.node.deckhouse.io/disruption-approved"
	annotationDraining           = "update.node.deckhouse.io/draining"
	annotationDrained            = "update.node.deckhouse.io/drained"
	annotationRollingUpdate      = "update.node.deckhouse.io/rolling-update"
	annotationConfigChecksum     = "node.deckhouse.io/configuration-checksum"

	// drainingSourceBashible is the source value used by bashible for draining annotations.
	drainingSourceBashible = "bashible"

	defaultDrainTimeout = 10 * time.Minute
)

func init() {
	dynr.RegisterReconciler(rcname.NodeUpdate, &corev1.Node{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler handles the node update approval and drain workflow.
//
// The workflow mirrors the logic from the legacy hooks update_approval.go and handle_draining.go:
//  1. processUpdatedNodes - if node is approved, ready, and its config checksum matches the NodeGroup,
//     clean up all update annotations (node is up-to-date).
//  2. handleDraining - if node has "draining" annotation, cordon and evict pods, then set "drained".
//  3. approveDisruptions - if node is approved and has disruption-required or rolling-update annotation,
//     decide whether to approve disruption, start draining, or delete the instance (RollingUpdate mode).
//  4. approveUpdates - if node is waiting-for-approval, decide whether to approve update based on
//     concurrency limits and node readiness within the NodeGroup.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{nodeUpdatePredicate()}
}

func (r *Reconciler) SetupWatches(w dynr.Watcher) {
	w.Watches(
		&deckhousev1.NodeGroup{},
		handler.EnqueueRequestsFromMapFunc(r.nodeGroupToNodes),
	)
}

// nodeGroupToNodes maps a NodeGroup change to reconcile requests for all nodes in that group.
func (r *Reconciler) nodeGroupToNodes(_ context.Context, obj client.Object) []reconcile.Request {
	ng, ok := obj.(*deckhousev1.NodeGroup)
	if !ok {
		return nil
	}

	nodeList := &corev1.NodeList{}
	if err := r.Client.List(context.Background(), nodeList,
		client.MatchingLabels{nodeGroupLabel: ng.Name},
	); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: node.Name},
		})
	}
	return requests
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ngName := node.Labels[nodeGroupLabel]

	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: ngName}, ng); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nodeInfo := extractNodeInfo(node)

	// Phase 1: Check if node is already up-to-date.
	// If node is approved, ready, and config checksums match — remove all update annotations.
	ngChecksum := ng.Annotations[annotationConfigChecksum]
	if nodeInfo.isApproved && nodeInfo.isReady &&
		nodeInfo.configChecksum != "" && ngChecksum != "" &&
		nodeInfo.configChecksum == ngChecksum {
		log.Info("node is up-to-date, cleaning update annotations", "node", node.Name, "nodeGroup", ngName)
		if err := r.markNodeUpToDate(ctx, node, nodeInfo.isDrained); err != nil {
			return ctrl.Result{}, fmt.Errorf("mark node up-to-date: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// Phase 2: Handle active draining (if draining annotation is set by bashible).
	if nodeInfo.isDraining {
		log.Info("node is draining", "node", node.Name)
		if err := r.handleDraining(ctx, node, ng); err != nil {
			return ctrl.Result{}, fmt.Errorf("handle draining for node %s: %w", node.Name, err)
		}
		return ctrl.Result{}, nil
	}

	// Phase 3: Approve disruptions for already approved nodes that need disruption.
	if nodeInfo.isApproved &&
		(nodeInfo.isDisruptionRequired || nodeInfo.isRollingUpdate) &&
		!nodeInfo.isDisruptionApproved {
		result, err := r.approveDisruption(ctx, node, ng, &nodeInfo)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("approve disruption for node %s: %w", node.Name, err)
		}
		if result != nil {
			return *result, nil
		}
	}

	// Phase 4: Approve updates for nodes waiting for approval.
	if nodeInfo.isWaitingForApproval && !nodeInfo.isApproved {
		if err := r.approveUpdate(ctx, node, ng); err != nil {
			return ctrl.Result{}, fmt.Errorf("approve update for node %s: %w", node.Name, err)
		}
	}

	return ctrl.Result{}, nil
}

// markNodeUpToDate removes all update-related annotations and uncordons the node if it was drained.
func (r *Reconciler) markNodeUpToDate(ctx context.Context, node *corev1.Node, wasDrained bool) error {
	patch := client.MergeFrom(node.DeepCopy())

	delete(node.Annotations, annotationApproved)
	delete(node.Annotations, annotationWaitingForApproval)
	delete(node.Annotations, annotationDisruptionRequired)
	delete(node.Annotations, annotationDisruptionApproved)
	delete(node.Annotations, annotationDrained)

	if wasDrained {
		node.Spec.Unschedulable = false
	}

	return r.Client.Patch(ctx, node, patch)
}

// handleDraining cordons the node, evicts pods, then marks it as drained.
func (r *Reconciler) handleDraining(ctx context.Context, node *corev1.Node, ng *deckhousev1.NodeGroup) error {
	log := logf.FromContext(ctx)

	drainTimeout := getDrainTimeout(ng)
	drainer := &drain.Drainer{Client: r.Client}

	if err := drainer.DrainNode(ctx, node); err != nil {
		log.Error(err, "drain failed, will retry", "node", node.Name)
		return err
	}

	if err := drainer.WaitForEviction(ctx, node, drainTimeout); err != nil {
		// Timeout is not fatal — proceed with marking drained, matching original hook behavior.
		log.Error(err, "drain timed out, proceeding anyway", "node", node.Name)
	}

	return r.markNodeDrained(ctx, node)
}

// markNodeDrained replaces the draining annotation with the drained annotation.
func (r *Reconciler) markNodeDrained(ctx context.Context, node *corev1.Node) error {
	// Re-read to get the latest resource version.
	fresh := &corev1.Node{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(node), fresh); err != nil {
		return err
	}

	patch := client.MergeFrom(fresh.DeepCopy())

	source := fresh.Annotations[annotationDraining]
	delete(fresh.Annotations, annotationDraining)
	if fresh.Annotations == nil {
		fresh.Annotations = make(map[string]string)
	}
	fresh.Annotations[annotationDrained] = source

	return r.Client.Patch(ctx, fresh, patch)
}

// listNodeGroupNodes lists all nodes belonging to the given NodeGroup.
func (r *Reconciler) listNodeGroupNodes(ctx context.Context, ngName string) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.MatchingLabelsSelector{
		Selector: labels.SelectorFromSet(labels.Set{nodeGroupLabel: ngName}),
	}); err != nil {
		return nil, fmt.Errorf("list nodes for node group %s: %w", ngName, err)
	}
	return nodeList.Items, nil
}

// getDrainTimeout returns the configured drain timeout or the default.
func getDrainTimeout(ng *deckhousev1.NodeGroup) time.Duration {
	if ng.Spec.NodeDrainTimeoutSecond != nil {
		return time.Duration(*ng.Spec.NodeDrainTimeoutSecond) * time.Second
	}
	return defaultDrainTimeout
}
