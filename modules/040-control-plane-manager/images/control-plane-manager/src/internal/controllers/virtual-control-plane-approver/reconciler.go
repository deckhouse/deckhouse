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

package virtualcontrolplaneapprover

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/controller-runtime/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/approver"
	"control-plane-manager/internal/constants"
)

var _ reconcile.Reconciler = (*reconciler)(nil)

type reconciler struct {
	client client.Client
	// approver is stateless/immutable, so it is safe to build once and reuse across reconciles.
	approver *approver.Approver
}

func newReconciler(cl client.Client) *reconciler {
	return &reconciler{
		client:   cl,
		approver: approver.NewApprover(approver.VirtualPipeline),
	}
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log.FromContext(ctx).Info("Reconcile started")

	nodeList := &controlplanev1alpha1.ControlPlaneNodeList{}
	if err := r.client.List(ctx, nodeList, client.InNamespace(request.Namespace)); err != nil {
		return reconcile.Result{}, err
	}

	nodes, nodeNames := r.getReadyNodes(nodeList)
	if nodes.IsZero() {
		log.FromContext(ctx).V(1).Info("no ready virtual control plane nodes found")
		return reconcile.Result{}, nil
	}

	operations := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(ctx, operations, client.InNamespace(request.Namespace)); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list virtual control plane operations: %w", err)
	}
	if len(operations.Items) == 0 {
		log.FromContext(ctx).V(1).Info("no virtual control plane operations found")
		return reconcile.Result{}, nil
	}

	operations.Items = filterOperationsTargetingReadyNodes(operations.Items, nodeNames)
	if len(operations.Items) == 0 {
		return reconcile.Result{}, nil
	}

	approvable := r.approver.SelectApprovable(operations.Items, nodes)

	for _, op := range approvable {
		original := op.DeepCopy()
		op.Spec.Approved = true

		if err := r.client.Patch(ctx, &op, client.MergeFrom(original)); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to approve ControlPlaneOperation %q: %w", op.Name, err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) getReadyNodes(nodeList *controlplanev1alpha1.ControlPlaneNodeList) (approver.Nodes, map[string]struct{}) {
	readyNodeNames := make(map[string]struct{}, len(nodeList.Items))
	for _, node := range nodeList.Items {
		if isNodeReady(node) {
			readyNodeNames[node.Name] = struct{}{}
		}
	}

	return approver.Nodes{Masters: len(readyNodeNames), Arbiters: 0}, readyNodeNames
}

func isNodeReady(node controlplanev1alpha1.ControlPlaneNode) bool {
	return true
}

func filterOperationsTargetingReadyNodes(
	operations []controlplanev1alpha1.ControlPlaneOperation,
	readyNodeNames map[string]struct{},
) []controlplanev1alpha1.ControlPlaneOperation {
	filtered := make([]controlplanev1alpha1.ControlPlaneOperation, 0, len(operations))
	for _, op := range operations {
		if _, ok := readyNodeNames[op.Labels[constants.ControlPlaneNodeNameLabelKey]]; ok {
			filtered = append(filtered, op)
		}
	}
	return filtered
}
