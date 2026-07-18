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

// Package nodeconfig renders a NodeConfig object for every node of an
// immutable-OS NodeGroup. Such nodes carry no bashible: the on-node agent
// watches its own NodeConfig, reconciles the node towards it and reports the
// outcome back through the object's status. This controller is the writer of
// that desired state, built from the NodeGroup the node belongs to plus the
// cluster's own state (API server endpoints, DNS, image digests, proxy token).
package nodeconfig

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController(controllerName, &corev1.Node{}, &Reconciler{})
}

type Reconciler struct {
	register.Base

	sources *sourceReader
}

// Setup wires an uncached reader: the secrets and config maps a NodeConfig is
// rendered from live outside the manager's cache scope.
func (r *Reconciler) Setup(mgr ctrl.Manager) error {
	r.sources = &sourceReader{Client: r.Client, Reader: mgr.GetAPIReader()}
	return nil
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// A NodeGroup change affects every node of every group, so it is funnelled
	// into one pass instead of a request per node.
	allMapper := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: allRequestName}}}
	})
	w.Watches(&v1.NodeGroup{}, allMapper)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if req.Name == allRequestName {
		return r.reconcileAllNodes(ctx, logger)
	}
	return r.reconcileNode(ctx, req.Name, logger)
}

// reconcileAllNodes re-renders every node that belongs to an immutable group.
func (r *Reconciler) reconcileAllNodes(ctx context.Context, logger logr.Logger) (ctrl.Result, error) {
	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes); err != nil {
		return ctrl.Result{}, fmt.Errorf("list nodes: %w", err)
	}

	var firstErr error
	for i := range nodes.Items {
		if _, err := r.reconcileNode(ctx, nodes.Items[i].Name, logger); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return ctrl.Result{}, firstErr
}

// reconcileNode brings one node's NodeConfig in line with its NodeGroup. A node
// that is gone, ungrouped, or in a bashible-managed group has no NodeConfig of
// ours; any leftover object is removed.
func (r *Reconciler) reconcileNode(ctx context.Context, nodeName string, logger logr.Logger) (ctrl.Result, error) {
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			// The NodeConfig is owned by the Node, so the API server collects
			// it; nothing to do here.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ngName := node.Labels[nodeGroupNameLabel]
	if ngName == "" {
		return ctrl.Result{}, r.deleteOrphaned(ctx, nodeName, logger)
	}

	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: ngName}, ng); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.deleteOrphaned(ctx, nodeName, logger)
		}
		return ctrl.Result{}, err
	}

	if ng.Spec.OSType != v1.OSTypeImmutable {
		return ctrl.Result{}, r.deleteOrphaned(ctx, nodeName, logger)
	}

	inputs, err := r.sources.readClusterInputs(ctx, ng.Status.KubernetesVersion)
	if err != nil {
		logger.Error(err, "cannot render NodeConfig yet", "node", nodeName, "nodeGroup", ngName)
		return ctrl.Result{}, err
	}

	desired := newNodeConfig(ng, node, inputs)
	return ctrl.Result{}, r.apply(ctx, desired, logger)
}

// apply creates the object or patches it when the rendered spec drifted. The
// status belongs to the node-local agent and is never touched here.
func (r *Reconciler) apply(ctx context.Context, desired *internalv1alpha1.NodeConfig, logger logr.Logger) error {
	existing := &internalv1alpha1.NodeConfig{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Client.Create(ctx, desired); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return fmt.Errorf("create NodeConfig %s: %w", desired.Name, err)
		}
		logger.Info("NodeConfig created", "node", desired.Name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get NodeConfig %s: %w", desired.Name, err)
	}

	if apiequality.Semantic.DeepEqual(existing.Spec, desired.Spec) &&
		apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) {
		logger.V(1).Info("NodeConfig unchanged", "node", desired.Name)
		return nil
	}

	patch := client.MergeFrom(existing.DeepCopy())
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	existing.OwnerReferences = desired.OwnerReferences
	if err := r.Client.Patch(ctx, existing, patch); err != nil {
		return fmt.Errorf("patch NodeConfig %s: %w", desired.Name, err)
	}
	logger.Info("NodeConfig updated", "node", desired.Name)
	return nil
}

// deleteOrphaned removes a NodeConfig this controller no longer owns, for
// instance after a node left an immutable group.
func (r *Reconciler) deleteOrphaned(ctx context.Context, name string, logger logr.Logger) error {
	existing := &internalv1alpha1.NodeConfig{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: name}, existing); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if existing.Labels[managedByLabel] != managedByValue {
		return nil
	}
	if err := r.Client.Delete(ctx, existing); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete NodeConfig %s: %w", name, err)
	}
	logger.Info("NodeConfig removed", "node", name)
	return nil
}
