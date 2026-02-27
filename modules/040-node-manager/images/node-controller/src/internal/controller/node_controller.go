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

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	nodeTypeLabelKey            = "node.deckhouse.io/type"
	staticNodeTypeValue         = "Static"
	cloudPermanentNodeTypeValue = "CloudPermanent"
)

type NodeReconciler struct {
	client.Client
}

func SetupNodeController(mgr ctrl.Manager) error {
	if err := (&NodeReconciler{
		Client: mgr.GetClient(),
	}).
		SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup node reconciler: %w", err)
	}

	return nil
}

func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("node-controller").
		For(&corev1.Node{}, builder.WithPredicates(staticNodePredicate())).
		Complete(r)
}

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("node", req.Name)

	node := &corev1.Node{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !isStaticNode(node) {
		return ctrl.Result{}, nil
	}

	if _, err := ensureInstanceExists(ctx, r.Client, node.Name); err != nil {
		return ctrl.Result{}, err
	}

	log.V(1).Info("instance ensured for static node")
	return ctrl.Result{}, nil
}

func staticNodePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isStaticNodeObject(e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isStaticNodeObject(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return isStaticNodeObject(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode, oldOK := e.ObjectOld.(*corev1.Node)
			newNode, newOK := e.ObjectNew.(*corev1.Node)
			if !oldOK || !newOK {
				return false
			}

			labelsChanged := !apiequality.Semantic.DeepEqual(oldNode.Labels, newNode.Labels)
			if !labelsChanged {
				return false
			}

			return isStaticNode(oldNode) || isStaticNode(newNode)
		},
	}
}

func isStaticNode(node *corev1.Node) bool {
	nodeType := node.Labels[nodeTypeLabelKey]
	return nodeType == staticNodeTypeValue || nodeType == cloudPermanentNodeTypeValue
}

func isStaticNodeObject(obj client.Object) bool {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return false
	}

	return isStaticNode(node)
}
