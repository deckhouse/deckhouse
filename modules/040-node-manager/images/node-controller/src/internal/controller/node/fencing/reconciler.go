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

package fencing

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	fencingEnabledLabel = "node-manager.deckhouse.io/fencing-enabled"

	annotationDisruptionApproved = "update.node.deckhouse.io/disruption-approved"
	annotationApproved           = "update.node.deckhouse.io/approved"
	annotationFencingDisable     = "node-manager.deckhouse.io/fencing-disable"
)

// maintenanceAnnotations lists annotations whose presence means the node
// is under maintenance and fencing must be skipped.
var maintenanceAnnotations = []string{
	annotationDisruptionApproved,
	annotationApproved,
	annotationFencingDisable,
}

func init() {
	dynr.RegisterReconciler(rcname.NodeFencing, &corev1.Node{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler implements fencing logic for nodes that have the
// "node-manager.deckhouse.io/fencing-enabled" label. It periodically checks
// each such node's lease in kube-node-lease; if the lease has been expired
// for more than 60 seconds, it force-deletes all pods on the node and then
// deletes the Node object itself.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			_, ok := obj.GetLabels()[fencingEnabledLabel]
			return ok
		}),
	}
}

func (r *Reconciler) SetupWatches(w dynr.Watcher) {
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get node %s: %w", req.Name, err)
	}

	// Skip nodes that are under maintenance.
	if hasMaintenanceAnnotation(node) {
		log.V(1).Info("node has maintenance annotation, skipping fencing", "node", req.Name)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Check the node lease.
	expired, err := isLeaseExpired(ctx, r.Client, node.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("check lease for node %s: %w", req.Name, err)
	}
	if !expired {
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	log.Info("node lease is expired, starting fencing", "node", req.Name)

	// Force delete all pods on the node.
	if err := r.forceDeletePods(ctx, node.Name); err != nil {
		return ctrl.Result{}, fmt.Errorf("force delete pods on node %s: %w", req.Name, err)
	}

	// Delete the Node object.
	log.Info("deleting node", "node", req.Name)
	if err := r.Client.Delete(ctx, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("delete node %s: %w", req.Name, err)
	}

	return ctrl.Result{}, nil
}

// hasMaintenanceAnnotation returns true if the node has any of the
// maintenance annotations that should prevent fencing.
func hasMaintenanceAnnotation(node *corev1.Node) bool {
	for _, annotation := range maintenanceAnnotations {
		if _, ok := node.Annotations[annotation]; ok {
			return true
		}
	}
	return false
}

// forceDeletePods lists all pods scheduled on the given node and
// force-deletes them with GracePeriodSeconds=0.
func (r *Reconciler) forceDeletePods(ctx context.Context, nodeName string) error {
	log := logf.FromContext(ctx)

	podList := &corev1.PodList{}
	if err := r.Client.List(ctx, podList, &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}),
	}); err != nil {
		return fmt.Errorf("list pods on node %s: %w", nodeName, err)
	}

	gracePeriod := int64(0)
	for i := range podList.Items {
		pod := &podList.Items[i]
		log.Info("force deleting pod", "pod", pod.Name, "namespace", pod.Namespace, "node", nodeName)
		if err := r.Client.Delete(ctx, pod, &client.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		}); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "failed to delete pod", "pod", pod.Name, "namespace", pod.Namespace)
		}
	}

	return nil
}
