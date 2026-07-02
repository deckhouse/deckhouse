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

package nodetopology

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	conditionInSync = "InSync"

	reasonEffectiveStateNotCollected  = "EffectiveStateNotCollected"
	messageEffectiveStateNotCollected = "Effective topology state has not been collected yet."
)

func init() {
	register.RegisterController("node-topology", &corev1.Node{}, &Controller{})
}

type Controller struct {
	register.Base
}

func (r *Controller) SetupWatches(w register.Watcher) {
	w.Watches(
		&v1.NodeGroup{},
		handler.EnqueueRequestsFromMapFunc(r.nodeGroupToNodes),
		builder.WithPredicates(),
	)
}

func (r *Controller) nodeGroupToNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	nodeGroupName := obj.GetName()

	var nodes corev1.NodeList
	if err := r.Client.List(ctx, &nodes, client.MatchingLabels{
		nodecommon.NodeGroupLabel: nodeGroupName,
	}); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: node.Name},
		})
	}

	return requests
}

func (r *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("reconciling node topology", "node", req.Name)

	var node corev1.Node
	if err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name}, &node); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("node not found, skipping", "node", req.Name)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	nodeGroupName := node.Labels[nodecommon.NodeGroupLabel]
	if nodeGroupName == "" {
		logger.V(1).Info("node has no node group label, skipping", "node", node.Name)
		return ctrl.Result{}, nil
	}

	var nodeGroup v1.NodeGroup
	if err := r.Client.Get(ctx, client.ObjectKey{Name: nodeGroupName}, &nodeGroup); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("node group not found, skipping", "node", node.Name, "nodeGroup", nodeGroupName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	var nodeTopology v1.NodeTopology
	err := r.Client.Get(ctx, client.ObjectKey{Name: node.Name}, &nodeTopology)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		nodeTopology = v1.NodeTopology{
			ObjectMeta: metav1.ObjectMeta{
				Name: node.Name,
			},
		}

		if err := r.Client.Create(ctx, &nodeTopology); err != nil {
			return ctrl.Result{}, err
		}

		logger.V(1).Info("created node topology", "nodeTopology", nodeTopology.Name)
	}

	patch := client.MergeFrom(nodeTopology.DeepCopy())

	nodeTopology.Status.NodeName = node.Name
	nodeTopology.Status.NodeGroup = nodeGroupName
	nodeTopology.Status.Desired = desiredTopologyState(&nodeGroup)
	nodeTopology.Status.Conditions = setInSyncUnknown(nodeTopology.Status.Conditions)

	if err := r.Client.Status().Patch(ctx, &nodeTopology, patch); err != nil {
		return ctrl.Result{}, err
	}

	logger.V(1).Info("patched node topology status", "nodeTopology", nodeTopology.Name)

	return ctrl.Result{}, nil
}

func desiredTopologyState(nodeGroup *v1.NodeGroup) *v1.NodeTopologyState {
	enabled := false

	state := &v1.NodeTopologyState{
		TopologyManager: &v1.NodeTopologyManagerState{
			Enabled: &enabled,
		},
	}

	if nodeGroup.Spec.Kubelet == nil || nodeGroup.Spec.Kubelet.TopologyManager == nil {
		return state
	}

	enabled = true
	state.TopologyManager.Enabled = &enabled
	state.TopologyManager.Policy = nodeGroup.Spec.Kubelet.TopologyManager.Policy
	state.TopologyManager.Scope = nodeGroup.Spec.Kubelet.TopologyManager.Scope

	return state
}

func setInSyncUnknown(conditions []metav1.Condition) []metav1.Condition {
	now := metav1.Now()

	condition := metav1.Condition{
		Type:               conditionInSync,
		Status:             metav1.ConditionUnknown,
		Reason:             reasonEffectiveStateNotCollected,
		Message:            messageEffectiveStateNotCollected,
		LastTransitionTime: now,
	}

	for i := range conditions {
		if conditions[i].Type == conditionInSync {
			if conditions[i].Status == condition.Status &&
				conditions[i].Reason == condition.Reason &&
				conditions[i].Message == condition.Message {
				condition.LastTransitionTime = conditions[i].LastTransitionTime
			}

			conditions[i] = condition
			return conditions
		}
	}

	return append(conditions, condition)
}
