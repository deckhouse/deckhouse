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

package controlplane

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrastructurev1alpha1 "github.com/deckhouse/node-controller/api/infrastructure.cluster.x-k8s.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	controlPlaneNamespace = "d8-cloud-instance-manager"
	masterNodeGroupLabel  = "node-role.kubernetes.io/control-plane"
)

func init() {
	dynr.RegisterReconciler(rcname.ControlPlane, &infrastructurev1alpha1.DeckhouseControlPlane{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler watches DeckhouseControlPlane resources and updates their status
// based on the state of master nodes. It sets the control plane as initialized,
// ready, and externally managed, and reports replica counts from master nodes.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupWatches(w dynr.Watcher) {
	w.Watches(
		&corev1.Node{},
		handler.EnqueueRequestsFromMapFunc(r.masterNodeToControlPlane),
	)
}

// masterNodeToControlPlane maps changes to master nodes to reconcile requests
// for all DeckhouseControlPlane resources.
func (r *Reconciler) masterNodeToControlPlane(ctx context.Context, obj client.Object) []reconcile.Request {
	if _, ok := obj.GetLabels()[masterNodeGroupLabel]; !ok {
		return nil
	}

	cpList := &infrastructurev1alpha1.DeckhouseControlPlaneList{}
	if err := r.Client.List(ctx, cpList, client.InNamespace(controlPlaneNamespace)); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list DeckhouseControlPlane resources")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(cpList.Items))
	for _, cp := range cpList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      cp.Name,
				Namespace: cp.Namespace,
			},
		})
	}
	return requests
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	cp := &infrastructurev1alpha1.DeckhouseControlPlane{}
	if err := r.Client.Get(ctx, req.NamespacedName, cp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get DeckhouseControlPlane %s/%s: %w", req.Namespace, req.Name, err)
	}

	// Count master nodes and ready master nodes.
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.MatchingLabels{masterNodeGroupLabel: ""}); err != nil {
		return ctrl.Result{}, fmt.Errorf("list master nodes: %w", err)
	}

	totalReplicas := int32(len(nodeList.Items))
	var readyReplicas int32
	for i := range nodeList.Items {
		if isNodeReady(&nodeList.Items[i]) {
			readyReplicas++
		}
	}

	patch := client.MergeFrom(cp.DeepCopy())

	cp.Status.Initialized = true
	cp.Status.Ready = true
	cp.Status.ExternalManagedControlPlane = true
	cp.Status.Replicas = totalReplicas
	cp.Status.ReadyReplicas = readyReplicas
	cp.Status.UnavailableReplicas = totalReplicas - readyReplicas

	if err := r.Client.Status().Patch(ctx, cp, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch DeckhouseControlPlane %s/%s status: %w", req.Namespace, req.Name, err)
	}

	log.V(1).Info("reconciled DeckhouseControlPlane status",
		"name", req.Name,
		"replicas", totalReplicas,
		"readyReplicas", readyReplicas,
	)
	return ctrl.Result{}, nil
}

// isNodeReady returns true if the node has the Ready condition set to True.
func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}
