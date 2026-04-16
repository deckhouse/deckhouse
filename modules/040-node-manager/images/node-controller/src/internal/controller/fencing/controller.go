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

package fencing

import (
	"context"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("node-fencing", &corev1.Node{}, &Reconciler{})
}

const (
	fencingEnabledLabel = "node-manager.deckhouse.io/fencing-enabled"
	fencingModeLabel    = "node-manager.deckhouse.io/fencing-mode"
	nodeTypeLabel       = nodecommon.NodeTypeLabel
	leaseNamespace      = "kube-node-lease"
	fencingTimeout      = 60 * time.Second
	requeueInterval     = 1 * time.Minute
	notifyMode          = "Notify"
)

var maintenanceAnnotations = []string{
	nodecommon.DisruptionApprovedAnnotation,
	nodecommon.ApprovedAnnotation,
	"node-manager.deckhouse.io/fencing-disable",
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, ok := obj.GetLabels()[fencingEnabledLabel]
		return ok
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if _, ok := node.Labels[fencingEnabledLabel]; !ok {
		logger.V(1).Info("skipping: node does not have fencing-enabled label", "node", node.Name)
		return ctrl.Result{}, nil
	}

	for _, annotation := range maintenanceAnnotations {
		if _, exists := node.Annotations[annotation]; exists {
			logger.V(1).Info("skipping: node has maintenance annotation", "node", node.Name, "annotation", annotation)
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
	}

	lease := &coordinationv1.Lease{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: leaseNamespace,
		Name:      node.Name,
	}, lease); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("lease not found for node, skipping fencing", "node", node.Name)
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
		return ctrl.Result{}, err
	}

	if lease.Spec.RenewTime == nil || time.Since(lease.Spec.RenewTime.Time) <= fencingTimeout {
		logger.V(1).Info("lease is fresh, no fencing needed", "node", node.Name,
			"renewTime", lease.Spec.RenewTime,
			"fencingTimeout", fencingTimeout)
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	leaseAge := time.Since(lease.Spec.RenewTime.Time)
	logger.Info("node lease expired, starting fencing",
		"node", node.Name,
		"leaseRenewTime", lease.Spec.RenewTime.Time,
		"leaseAge", leaseAge.Round(time.Second),
		"fencingTimeout", fencingTimeout,
	)

	podList := &corev1.PodList{}
	if err := r.Client.List(ctx, podList, &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}),
	}); err != nil {
		logger.Error(err, "failed to list pods on node", "node", node.Name)
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	logger.Info("deleting pods on fenced node", "node", node.Name, "podCount", len(podList.Items))

	gracePeriod := int64(0)
	for i := range podList.Items {
		pod := &podList.Items[i]
		logger.V(1).Info("deleting pod", "pod", pod.Name, "namespace", pod.Namespace, "node", node.Name)
		if err := r.Client.Delete(ctx, pod, &client.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		}); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "failed to delete pod", "pod", pod.Name, "namespace", pod.Namespace)
		}
	}

	if shouldDeleteNode(node) {
		logger.Info("deleting fenced node", "node", node.Name)
		if err := r.Client.Delete(ctx, node, &client.DeleteOptions{
			PropagationPolicy: propagationPtr(metav1.DeletePropagationBackground),
		}); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		logger.Info("node deleted successfully", "node", node.Name)
	} else {
		fencingMode := node.Labels[fencingModeLabel]
		nodeType := node.Labels[nodeTypeLabel]
		logger.Info("node preserved after fencing (pods deleted only)",
			"node", node.Name,
			"fencingMode", fencingMode,
			"nodeType", nodeType,
		)
	}

	return ctrl.Result{}, nil
}

func shouldDeleteNode(node *corev1.Node) bool {
	fencingMode := node.Labels[fencingModeLabel]
	nodeType := v1.NodeType(node.Labels[nodeTypeLabel])
	return fencingMode != notifyMode && nodeType != v1.NodeTypeStatic && nodeType != v1.NodeTypeCloudStatic
}

func propagationPtr(p metav1.DeletionPropagation) *metav1.DeletionPropagation {
	return &p
}
