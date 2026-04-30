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

package operationsapprover

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
)

var logger = log.NewLogger().Named("operations-approver-controller")

type reconciler struct {
	client client.Client
}

func Register(mgr manager.Manager) error {
	r := &reconciler{
		client: mgr.GetClient(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named("operations_approver_controller").
		For(
			&controlplanev1alpha1.ControlPlaneOperation{},
			builder.WithPredicates(getPredicates()),
		).
		Complete(r)
}

func getPredicates() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldOperation, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
			newOperation, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)

			if !okOld || !okNew {
				return false
			}

			if !oldOperation.IsTerminal() && newOperation.IsTerminal() {
				return true
			}

			return false
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger.Info("Reconcile started")

	nodes, err := r.getNodeCounts(ctx)
	if err != nil {
		logger.Error("failed to get node count", log.Err(err))
		return reconcile.Result{}, err
	}
	if nodes.isZero() {
		logger.Warn("nodes not found")
		return reconcile.Result{}, nil
	}

	operations := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(ctx, operations, operationsMatchingReadyNodes(nodes)); err != nil {
		return reconcile.Result{}, err
	}
	if len(operations.Items) == 0 {
		logger.Warn("no control plane operations found")
		return reconcile.Result{}, nil
	}

	approver := newApprover(nodes, operations.Items)

	for _, unapprovedOperation := range approver.approveQueue {
		canApprove := approver.tryApprove(unapprovedOperation)

		if canApprove {
			original := unapprovedOperation.DeepCopy()
			unapprovedOperation.Spec.Approved = true

			if err := r.client.Patch(ctx, &unapprovedOperation, client.MergeFrom(original)); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to approve ControlPlaneOperation %q: %w", unapprovedOperation.Name, err)
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) getNodeCounts(ctx context.Context) (nodeCounts, error) {
	nodes := nodeCounts{
		readyNodeNames: make(map[string]struct{}),
	}

	masterList := &corev1.NodeList{}
	if err := r.client.List(ctx, masterList, client.MatchingLabels{constants.ControlPlaneNodeLabelKey: ""}); err != nil {
		return nodeCounts{}, fmt.Errorf("failed to list master nodes: %w", err)
	}
	for _, node := range masterList.Items {
		if isNodeReady(node) {
			nodes.masters++
			nodes.readyNodeNames[node.Name] = struct{}{}
		}
	}

	arbiterList := &corev1.NodeList{}
	if err := r.client.List(ctx, arbiterList, client.MatchingLabels{constants.EtcdArbiterNodeLabelKey: ""}); err != nil {
		return nodeCounts{}, fmt.Errorf("failed to list arbiter nodes: %w", err)
	}
	for _, node := range arbiterList.Items {
		if isNodeReady(node) {
			nodes.arbiters++
			nodes.readyNodeNames[node.Name] = struct{}{}
		}
	}

	return nodes, nil
}

func operationsMatchingReadyNodes(nodes nodeCounts) client.ListOption {
	nodeNames := make([]string, 0, len(nodes.readyNodeNames))
	for nodeName := range nodes.readyNodeNames {
		nodeNames = append(nodeNames, nodeName)
	}

	requirement, err := labels.NewRequirement(constants.ControlPlaneNodeNameLabelKey, selection.In, nodeNames)
	if err != nil {
		return client.MatchingLabels{}
	}

	return client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*requirement)}
}

func isNodeReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}
