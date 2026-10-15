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

package cpnconditions

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/cpn/cpnplanner"
)

const requeueInterval = 5 * time.Minute

// Reconciler owns ControlPlaneNode.status.conditions only; status.caChecksum and
// status.components belong to cpnreconcile.
type Reconciler struct {
	client client.Client
	// podReader is the manager cache: the manager client bypasses the cache for pods (DisableFor).
	podReader client.Reader
}

func New(cl client.Client, podReader client.Reader) *Reconciler {
	return &Reconciler{client: cl, podReader: podReader}
}

var _ reconcile.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconcile started")

	cpn := &controlplanev1alpha1.ControlPlaneNode{}
	if err := r.client.Get(ctx, req.NamespacedName, cpn); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("get ControlPlaneNode %s: %w", req.NamespacedName, err)
	}

	current, err := r.listOperationsForNode(ctx, cpn)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("list ControlPlaneOperations for %s: %w", req.NamespacedName, err)
	}

	pods, err := r.getPodsForNode(ctx, cpn)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get control plane pods for %s: %w", req.NamespacedName, err)
	}

	computed := cpnplanner.ComputeConditions(cpn, pods, current)
	if equality.Semantic.DeepEqual(cpn.Status.Conditions, computed) {
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	base := cpn.DeepCopy()
	cpn.Status.Conditions = computed
	if err := r.patchStatus(ctx, cpn, base); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("patch ControlPlaneNode %s status: %w", req.NamespacedName, err)
	}
	logger.Info("Conditions updated")

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

func (r *Reconciler) listOperationsForNode(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) ([]controlplanev1alpha1.ControlPlaneOperation, error) {
	list := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(ctx, list,
		client.InNamespace(cpn.Namespace),
		client.MatchingLabels{constants.ControlPlaneNodeNameLabelKey: cpn.Name},
	); err != nil {
		return nil, err
	}
	// Owner UID check: skip not yet garbage collected operations of a previous same-name CPN.
	return cpnplanner.OwnedOperations(cpn, list.Items), nil
}

// getPodsForNode returns the node's control plane mirror pods by component; a missing pod has no key.
func (r *Reconciler) getPodsForNode(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) (map[controlplanev1alpha1.OperationComponent]*corev1.Pod, error) {
	pods := make(map[controlplanev1alpha1.OperationComponent]*corev1.Pod)
	for component := range controlplanev1alpha1.ComponentRegistry() {
		pod := &corev1.Pod{}
		key := client.ObjectKey{Namespace: constants.KubeSystemNamespace, Name: component.MirrorPodName(cpn.Name)}
		if err := r.podReader.Get(ctx, key, pod); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		pods[component] = pod
	}
	return pods, nil
}

func (r *Reconciler) patchStatus(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, base *controlplanev1alpha1.ControlPlaneNode) error {
	return r.client.Status().Patch(ctx, cpn, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
}
