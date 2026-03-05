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

package node

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/common"
)

type NodeReconciler struct {
	client.Client
}

type nodeReconcileState struct {
	req    ctrl.Request
	node   *corev1.Node
	result ctrl.Result
}

type nodeReconcileStep func(ctx context.Context, state *nodeReconcileState) (done bool, result ctrl.Result, err error)

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
	log.V(4).Info("tick", "op", "node.reconcile.start")

	state := &nodeReconcileState{
		req:    req,
		result: ctrl.Result{RequeueAfter: time.Minute},
	}

	for _, step := range []nodeReconcileStep{
		// fetch current node object from api server
		r.reconcileNodeFetch,
		// delete node based instance when node object is gone
		r.reconcileNodeMissingInstanceDeletion,
		// ensure instance for static node and set running phase
		r.reconcileNodeInstance,
	} {
		done, result, err := step(ctx, state)
		if err != nil {
			return ctrl.Result{}, err
		}
		if done {
			return result, nil
		}
	}

	return state.result, nil
}

func (r *NodeReconciler) reconcileNodeFetch(
	ctx context.Context,
	state *nodeReconcileState,
) (bool, ctrl.Result, error) {
	node := &corev1.Node{}
	if err := r.Get(ctx, state.req.NamespacedName, node); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return false, ctrl.Result{}, err
		}

		state.node = nil
		return false, ctrl.Result{}, nil
	}

	state.node = node
	return false, ctrl.Result{}, nil
}

func (r *NodeReconciler) reconcileNodeMissingInstanceDeletion(
	ctx context.Context,
	state *nodeReconcileState,
) (bool, ctrl.Result, error) {
	if state.node != nil {
		return false, ctrl.Result{}, nil
	}

	deleted, err := r.deleteNodeBasedInstanceIfExists(ctx, state.req.Name)
	if err != nil {
		return false, ctrl.Result{}, err
	}

	ctrl.LoggerFrom(ctx).V(1).Info("node not found, node based instance delete handled", "instance", state.req.Name, "deleted", deleted)
	return true, state.result, nil
}

func (r *NodeReconciler) reconcileNodeInstance(
	ctx context.Context,
	state *nodeReconcileState,
) (bool, ctrl.Result, error) {
	if state.node == nil {
		return false, ctrl.Result{}, fmt.Errorf("node is nil in instance step")
	}

	log := ctrl.LoggerFrom(ctx)

	if !IsStaticNode(state.node) {
		log.V(4).Info("reconcileNodeInstance: node is not static, skipping")
		return true, state.result, nil
	}

	log.V(4).Info("tick", "op", "node.instance.ensure")
	instance, err := common.EnsureInstanceExists(ctx, r.Client, state.node.Name, deckhousev1alpha2.InstanceSpec{
		NodeRef: deckhousev1alpha2.NodeRef{Name: state.node.Name},
	})
	if err != nil {
		return false, ctrl.Result{}, err
	}
	if err := common.SetInstancePhase(ctx, r.Client, instance, deckhousev1alpha2.InstancePhaseRunning); err != nil {
		return false, ctrl.Result{}, err
	}

	log.V(1).Info("instance ensured for static node")
	return true, state.result, nil
}
