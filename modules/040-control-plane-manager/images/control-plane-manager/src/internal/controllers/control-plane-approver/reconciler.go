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

package controlplaneapprover

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/controller-runtime/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/approver"
	"control-plane-manager/internal/constants"
)

type reconciler struct {
	client   client.Client
	approver *approver.Approver
}

func newReconciler(cl client.Client) *reconciler {
	return &reconciler{
		client:   cl,
		approver: approver.NewApprover(approver.NormalPipeline),
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	masterList := &corev1.NodeList{}
	if err := r.client.List(ctx, masterList, client.MatchingLabels{constants.ControlPlaneNodeLabelKey: ""}); err != nil {
		return reconcile.Result{}, err
	}

	arbiterList := &corev1.NodeList{}
	if err := r.client.List(ctx, arbiterList, client.MatchingLabels{constants.EtcdArbiterNodeLabelKey: ""}); err != nil {
		return reconcile.Result{}, err
	}

	nodes, nodeNames := r.getReadyNodes(masterList, arbiterList)
	if nodes.IsZero() {
		log.FromContext(ctx).V(1).Info("no ready control plane nodes found")
		return reconcile.Result{}, nil
	}

	operations := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(
		ctx, operations,
		client.InNamespace(constants.KubeSystemNamespace),
		operationsMatchingReadyNodes(nodeNames),
	); err != nil {
		return reconcile.Result{}, err
	}
	if len(operations.Items) == 0 {
		log.FromContext(ctx).V(1).Info("no control plane operations found")
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

func (r *reconciler) getReadyNodes(masterList *corev1.NodeList, arbiterList *corev1.NodeList) (approver.Nodes, map[string]struct{}) {
	nodes := approver.Nodes{}
	nodeNames := make(map[string]struct{})

	for _, node := range masterList.Items {
		if isNodeReady(node) {
			nodes.Masters++
			nodeNames[node.Name] = struct{}{}
		}
	}

	for _, node := range arbiterList.Items {
		if isNodeReady(node) {
			nodes.Arbiters++
			nodeNames[node.Name] = struct{}{}
		}
	}

	return nodes, nodeNames
}

func operationsMatchingReadyNodes(readyNodeNames map[string]struct{}) client.ListOption {
	nodeNames := make([]string, 0, len(readyNodeNames))
	for nodeName := range readyNodeNames {
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
