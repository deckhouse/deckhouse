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

package node

import (
	"context"
	"fmt"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		For(&corev1.Node{}, builder.WithPredicates(nodePredicate())).
		Complete(r)
}

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("node", req.Name)

	node := &corev1.Node{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		deleted, err := r.deleteStaticInstanceIfExists(ctx, req.Name)
		if err != nil {
			return ctrl.Result{}, err
		}

		log.V(1).Info("node not found, static instance delete handled", "instance", req.Name, "deleted", deleted)
		return ctrl.Result{}, nil
	}

	if IsStaticNode(node) {
		instance, err := common.EnsureInstanceExists(ctx, r.Client, node.Name, deckhousev1alpha2.InstanceSpec{
			NodeRef: deckhousev1alpha2.NodeRef{Name: node.Name},
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := r.setInstancePhase(ctx, instance, deckhousev1alpha2.InstancePhaseRunning); err != nil {
			return ctrl.Result{}, err
		}

		log.V(1).Info("instance ensured for static node")
		return ctrl.Result{}, nil
	}

	log.V(1).Info("node is not static, skipping")
	return ctrl.Result{}, nil
}
