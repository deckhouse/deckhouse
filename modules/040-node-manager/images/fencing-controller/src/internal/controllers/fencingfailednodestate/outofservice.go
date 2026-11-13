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

package fencingfailednodestate

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SetupOutOfServiceMetric adds the controller that publishes the
// out-of-service taint of each Node, no matter who set it. Like the
// reconciler of the incidents, it runs only on the leader.
func SetupOutOfServiceMetric(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("node-out-of-service").
		// The kubelet updates the status of a Node often. Only a change of the
		// taint changes the metric.
		For(&corev1.Node{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return hasOutOfServiceTaint(e.ObjectOld) != hasOutOfServiceTaint(e.ObjectNew)
			},
		})).
		Complete(reconcileOutOfServiceMetric(mgr.GetClient()))
}

func reconcileOutOfServiceMetric(c client.Reader) reconcile.Func {
	return func(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
		var node corev1.Node

		err := c.Get(ctx, req.NamespacedName, &node)

		switch {
		case apierrors.IsNotFound(err):
			reportOutOfServiceTaint(req.Name, false)
		case err != nil:
			return ctrl.Result{}, fmt.Errorf("get node %q: %w", req.Name, err)
		default:
			reportOutOfServiceTaint(node.Name, hasOutOfServiceTaint(&node))
		}

		return ctrl.Result{}, nil
	}
}

func hasOutOfServiceTaint(obj client.Object) bool {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return false
	}

	for i := range node.Spec.Taints {
		if node.Spec.Taints[i].Key == corev1.TaintNodeOutOfService {
			return true
		}
	}

	return false
}
