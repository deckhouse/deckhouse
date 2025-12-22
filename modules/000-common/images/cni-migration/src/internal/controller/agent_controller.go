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
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cnimigrationv1alpha1 "deckhouse.io/cni-migration/api/v1alpha1"
)

// CNIAgentReconciler reconciles a CNINodeMigration object on a specific node
type CNIAgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the node-specific tasks
func (r *CNIAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return ctrl.Result{}, fmt.Errorf("NODE_NAME env var not set")
	}

	// Only process our own node
	if req.Name != nodeName {
		return ctrl.Result{}, nil
	}

	// Fetch CNINodeMigration
	nodeMigration := &cnimigrationv1alpha1.CNINodeMigration{}
	if err := r.Get(ctx, req.NamespacedName, nodeMigration); err != nil {
		if errors.IsNotFound(err) {
			// Manager creates this resource, agent just waits for it
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch parent CNIMigration to know the TargetCNI and current state
	// We assume there's only one CNIMigration active
	cniMigrations := &cnimigrationv1alpha1.CNIMigrationList{}
	if err := r.List(ctx, cniMigrations); err != nil {
		return ctrl.Result{}, err
	}
	if len(cniMigrations.Items) == 0 {
		return ctrl.Result{}, nil
	}

	// Pick the oldest active migration (same logic as manager)
	var cniMigration *cnimigrationv1alpha1.CNIMigration
	for i := range cniMigrations.Items {
		m := &cniMigrations.Items[i]
		isFinished := false
		for _, cond := range m.Status.Conditions {
			if cond.Type == cnimigrationv1alpha1.ConditionSucceeded && cond.Status == metav1.ConditionTrue {
				isFinished = true
				break
			}
		}
		if isFinished {
			continue
		}
		if cniMigration == nil || m.CreationTimestamp.Before(&cniMigration.CreationTimestamp) {
			cniMigration = m
		}
	}

	if cniMigration == nil {
		return ctrl.Result{}, nil
	}

	// 1. Cleanup old CNI
	if !r.hasNodeCondition(nodeMigration, cnimigrationv1alpha1.NodeConditionCleanupDone) {
		// Manager should signal when it's time to cleanup
		// For now, we'll assume we cleanup when OldCNIDisabled is True in parent
		if r.hasParentCondition(cniMigration, cnimigrationv1alpha1.ConditionOldCNIDisabled) {
			logger.Info("Starting node cleanup")
			if err := RunCleanup(ctx, cniMigration.Status.CurrentCNI); err != nil {
				return ctrl.Result{}, err
			}
			logger.Info("Cleanup completed successfully")
			if err := r.setNodeCondition(ctx, nodeMigration, cnimigrationv1alpha1.NodeConditionCleanupDone, metav1.ConditionTrue, "CleanupSuccessful", "Artifacts removed"); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// 2. Restart Pods
	// We restart pods only after TargetCNIReady is True in parent
	if r.hasParentCondition(cniMigration, cnimigrationv1alpha1.ConditionTargetCNIReady) {
		if !r.hasNodeCondition(nodeMigration, "PodsRestarted") {
			logger.Info("Starting pod restart on node")
			podList := &corev1.PodList{}
			if err := r.List(ctx, podList, client.MatchingFields{"spec.nodeName": nodeName}); err != nil {
				return ctrl.Result{}, err
			}

			podsDeleted := 0
			for _, pod := range podList.Items {
				if pod.Annotations[EffectiveCNIAnnotation] == cniMigration.Status.CurrentCNI {
					if pod.DeletionTimestamp != nil {
						continue
					}
					if err := r.Delete(ctx, &pod); err != nil {
						if !errors.IsNotFound(err) {
							return ctrl.Result{}, err
						}
					}
					podsDeleted++
				}
			}
			logger.Info("Finished deleting pods", "Count", podsDeleted)

			if err := r.setNodeCondition(ctx, nodeMigration, "PodsRestarted", metav1.ConditionTrue, "PodsDeleted", fmt.Sprintf("%d pods restarted", podsDeleted)); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *CNIAgentReconciler) hasNodeCondition(m *cnimigrationv1alpha1.CNINodeMigration, condType string) bool {
	for _, c := range m.Status.Conditions {
		if c.Type == condType && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func (r *CNIAgentReconciler) hasParentCondition(m *cnimigrationv1alpha1.CNIMigration, condType string) bool {
	for _, c := range m.Status.Conditions {
		if c.Type == condType && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func (r *CNIAgentReconciler) setNodeCondition(ctx context.Context, m *cnimigrationv1alpha1.CNINodeMigration, condType string, status metav1.ConditionStatus, reason, message string) error {
	newCond := metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	found := false
	for i, c := range m.Status.Conditions {
		if c.Type == condType {
			m.Status.Conditions[i] = newCond
			found = true
			break
		}
	}
	if !found {
		m.Status.Conditions = append(m.Status.Conditions, newCond)
	}

	return r.Status().Update(ctx, m)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CNIAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index pods by node name for efficient listing
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod := rawObj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&cnimigrationv1alpha1.CNINodeMigration{}).
		Complete(r)
}
