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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cniswitcherv1alpha1 "deckhouse.io/cni-switch-helper/api/v1alpha1"
)

const (
	EffectiveCNIAnnotation = "effective-cni.network.deckhouse.io"
)

// CNIMigrationReconciler reconciles a CNIMigration object
type CNIMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CNIMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch the CNIMigration object
	cniMigration := &cniswitcherv1alpha1.CNIMigration{}
	if err := r.Get(ctx, req.NamespacedName, cniMigration); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("CNIMigration resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get CNIMigration")
		return ctrl.Result{}, err
	}

	// 2. Get the current node name from the NODE_NAME environment variable
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		err := fmt.Errorf("NODE_NAME environment variable not set")
		logger.Error(err, "Unable to determine current node name. This must be set in the Pod spec.")
		// Do not requeue, as this is a configuration error
		return ctrl.Result{}, err
	}

	// 3. Fetch or create CNINodeMigration for the current node
	cniNodeMigration := &cniswitcherv1alpha1.CNINodeMigration{}
	err := r.Get(ctx, types.NamespacedName{Name: nodeName}, cniNodeMigration)
	if err != nil {
		if errors.IsNotFound(err) {
			// CNINodeMigration for this node not found, create it.
			logger.Info("CNINodeMigration for this node not found, creating a new one", "Node", nodeName)
			cniNodeMigration = &cniswitcherv1alpha1.CNINodeMigration{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(cniMigration, cniswitcherv1alpha1.GroupVersion.WithKind("CNIMigration")),
					},
				},
				Spec: cniswitcherv1alpha1.CNINodeMigrationSpec{},
				Status: cniswitcherv1alpha1.CNINodeMigrationStatus{
					Phase: "Pending", // Initial phase
					Conditions: []metav1.Condition{
						{
							Type:               "Initialized",
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             "CNINodeMigrationCreated",
							Message:            "CNINodeMigration resource created for this node.",
						},
					},
				},
			}
			if createErr := r.Create(ctx, cniNodeMigration); createErr != nil {
				logger.Error(createErr, "Failed to create CNINodeMigration resource", "Node", nodeName)
				return ctrl.Result{}, createErr
			}
			logger.Info("Created CNINodeMigration for node", "Node", nodeName)
			// Requeue immediately to process the newly created object
			return ctrl.Result{Requeue: true}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get CNINodeMigration for node", "Node", nodeName)
		return ctrl.Result{}, err
	}

	// 4. Handle phase-specific logic
	switch cniMigration.Spec.Phase {
	case "Prepare":
		return r.reconcilePrepare(ctx, cniMigration, cniNodeMigration)
	case "Migrate":
		return r.reconcileMigrate(ctx, cniMigration, cniNodeMigration)
	default:
		logger.Info("Unknown or unhandled CNIMigration phase, no action taken",
			"Phase", cniMigration.Spec.Phase)
	}

	return ctrl.Result{}, nil
}

func (r *CNIMigrationReconciler) reconcilePrepare(
	ctx context.Context,
	cniMigration *cniswitcherv1alpha1.CNIMigration,
	cniNodeMigration *cniswitcherv1alpha1.CNINodeMigration,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// If the node is already prepared or failed, skip this phase.
	if cniNodeMigration.Status.Phase == "Prepared" || cniNodeMigration.Status.Phase == "Failed" {
		logger.Info("CNINodeMigration is already in its final state for the preparation phase.",
			"Phase", cniNodeMigration.Status.Phase)
		return ctrl.Result{}, nil
	}

	logger.Info("Starting Prepare phase for node")

	// Get all pods running on this node
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.MatchingFields{"spec.nodeName": cniNodeMigration.Name}); err != nil {
		logger.Error(err, "Failed to list pods on node")
		return ctrl.Result{}, err
	}

	podsAnnotated := 0
	totalPodsToProcess := 0
	var patchErrors []error

	for _, pod := range podList.Items {
		if pod.Spec.HostNetwork {
			continue
		}
		// Skip terminal pods as they don't need migration
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		totalPodsToProcess++

		if pod.Annotations[EffectiveCNIAnnotation] == cniMigration.Status.CurrentCNI {
			continue
		}

		patchedPod := pod.DeepCopy()
		if patchedPod.Annotations == nil {
			patchedPod.Annotations = make(map[string]string)
		}
		patchedPod.Annotations[EffectiveCNIAnnotation] = cniMigration.Status.CurrentCNI

		if err := r.Patch(ctx, patchedPod, client.MergeFrom(&pod)); err != nil {
			logger.Error(err, "Failed to annotate pod", "Pod", pod.Name)
			patchErrors = append(patchErrors, err)
			continue // Continue trying other pods
		}
		podsAnnotated++
	}

	if len(patchErrors) > 0 {
		logger.Error(fmt.Errorf("encountered %d errors while annotating pods",
			len(patchErrors)), "Partial failure during pod annotation")
		// Return an error to trigger requeue, but do NOT update status to Failed immediately.
		// This allows the controller to keep retrying without alarming the user or CLI prematurely.
		// The CLI will see that the node is not "PreparationSucceeded" yet and keep waiting.
		return ctrl.Result{}, fmt.Errorf("failed to annotate some pods: %v", patchErrors[0])
	}

	logger.Info("Finished annotating pods", "AnnotatedCount", podsAnnotated)

	// Update status to reflect completion using RetryOnConflict
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Refetch the object inside the retry loop
		if err := r.Get(ctx, client.ObjectKeyFromObject(cniNodeMigration), cniNodeMigration); err != nil {
			return err
		}

		cniNodeMigration.Status.Phase = "Prepared"
		cniNodeMigration.Status.Conditions = append(cniNodeMigration.Status.Conditions, metav1.Condition{
			Type:               "PreparationSucceeded",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "PodsAnnotated",
			Message: fmt.Sprintf("%d pods on the node received annotation.",
				totalPodsToProcess),
		})

		return r.Status().Update(ctx, cniNodeMigration)
	})
	if err != nil {
		logger.Error(err, "Failed to update CNINodeMigration status with retry")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully completed Prepare phase for node")
	return ctrl.Result{}, nil
}

func (r *CNIMigrationReconciler) reconcileMigrate(
	ctx context.Context,
	cniMigration *cniswitcherv1alpha1.CNIMigration,
	cniNodeMigration *cniswitcherv1alpha1.CNINodeMigration,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// If the node is already succeeded or failed, skip this phase.
	if cniNodeMigration.Status.Phase == "Succeeded" || cniNodeMigration.Status.Phase == "Failed" {
		logger.Info("CNINodeMigration is already in a terminal state for Migrate phase",
			"Phase", cniNodeMigration.Status.Phase)
		return ctrl.Result{}, nil
	}

	// 1. Run node cleanup
	cleanupCompleted := false
	for _, cond := range cniNodeMigration.Status.Conditions {
		if cond.Type == "CleanupSucceeded" && cond.Status == metav1.ConditionTrue {
			cleanupCompleted = true
			break
		}
	}

	if !cleanupCompleted {
		logger.Info("Starting node cleanup", "cni", cniMigration.Status.CurrentCNI)
		if err := RunCleanup(ctx, cniMigration.Status.CurrentCNI); err != nil {
			logger.Error(err, "Node cleanup failed")
			return r.updateNodeStatusWithError(ctx, cniNodeMigration, "CleanupFailed", err)
		}

		logger.Info("Node cleanup successful")
		// Update status to reflect completion using RetryOnConflict
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, client.ObjectKeyFromObject(cniNodeMigration), cniNodeMigration); err != nil {
				return err
			}
			cniNodeMigration.Status.Conditions = append(cniNodeMigration.Status.Conditions, metav1.Condition{
				Type:               "CleanupSucceeded",
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "OldCNIArtifactsRemoved",
				Message: fmt.Sprintf("Artifacts for CNI '%s' were successfully removed.",
					cniMigration.Status.CurrentCNI),
			})
			return r.Status().Update(ctx, cniNodeMigration)
		})
		if err != nil {
			logger.Error(err, "Failed to update CNINodeMigration status after cleanup with retry")
			return ctrl.Result{}, err
		}
		// Requeue to proceed to the next (pod restart)
		return ctrl.Result{Requeue: true}, nil
	}

	// 2. Restart Pods
	// We must wait until the new CNI is enabled and ready before restarting pods.
	newCNIEnabled := false
	for _, cond := range cniMigration.Status.Conditions {
		if cond.Type == "NewCNIEnabled" && cond.Status == metav1.ConditionTrue {
			newCNIEnabled = true
			break
		}
	}

	if !newCNIEnabled {
		logger.Info("Waiting for NewCNIEnabled condition before restarting pods")
		// Requeue to check again later
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Check if pods have already been restarted
	for _, cond := range cniNodeMigration.Status.Conditions {
		if cond.Type == "PodsRestarted" && cond.Status == metav1.ConditionTrue {
			logger.Info("Pods have already been restarted on this node")
			// Update status to reflect completion using RetryOnConflict
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(ctx, client.ObjectKeyFromObject(cniNodeMigration), cniNodeMigration); err != nil {
					return err
				}
				// Final step, migration on this node is complete
				cniNodeMigration.Status.Phase = "Succeeded"
				return r.Status().Update(ctx, cniNodeMigration)
			})
			if err != nil {
				logger.Error(err, "Failed to update CNINodeMigration status to Succeeded with retry")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	logger.Info("Starting Pod restart for node")

	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.MatchingFields{"spec.nodeName": cniNodeMigration.Name}); err != nil {
		logger.Error(err, "Failed to list pods on node for restart")
		return r.updateNodeStatusWithError(ctx, cniNodeMigration, "PodListFailed", err)
	}

	podsDeleted := 0
	for _, pod := range podList.Items {
		if pod.Annotations[EffectiveCNIAnnotation] == cniMigration.Status.CurrentCNI {
			// If pod is already marked for deletion, skip it.
			if pod.DeletionTimestamp != nil {
				continue
			}

			if err := r.Delete(ctx, &pod); err != nil {
				if !errors.IsNotFound(err) {
					logger.Error(err, "Failed to delete pod for restart", "Pod", pod.Name)
					return r.updateNodeStatusWithError(ctx, cniNodeMigration, "PodDeletionFailed", err)
				}
				// If not found, it's already deleted, which is what we want.
			}
			podsDeleted++
		}
	}

	logger.Info("Finished deleting pods", "DeletedCount", podsDeleted)

	// Update status to reflect pod restart completion using RetryOnConflict
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Get(ctx, client.ObjectKeyFromObject(cniNodeMigration), cniNodeMigration); err != nil {
			return err
		}
		cniNodeMigration.Status.Conditions = append(cniNodeMigration.Status.Conditions, metav1.Condition{
			Type:               "PodsRestarted",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "OldPodsDeleted",
			Message:            fmt.Sprintf("%d pods with old CNI annotation were deleted.", podsDeleted),
		})
		cniNodeMigration.Status.Phase = "Succeeded"
		return r.Status().Update(ctx, cniNodeMigration)
	})
	if err != nil {
		logger.Error(err, "Failed to update CNINodeMigration status after pod deletion with retry")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *CNIMigrationReconciler) updateNodeStatusWithError(
	ctx context.Context,
	cniNodeMigration *cniswitcherv1alpha1.CNINodeMigration,
	reason string,
	err error,
) (ctrl.Result, error) {
	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Refetch the object inside the retry loop
		if getErr := r.Get(ctx, client.ObjectKeyFromObject(cniNodeMigration), cniNodeMigration); getErr != nil {
			logger := log.FromContext(ctx)
			logger.Error(getErr, "Failed to re-fetch CNINodeMigration before error status update in retry loop")
			return getErr
		}

		cniNodeMigration.Status.Phase = "Failed"
		cniNodeMigration.Status.Conditions = append(cniNodeMigration.Status.Conditions, metav1.Condition{
			Type:               "Failed",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            err.Error(),
		})
		return r.Status().Update(ctx, cniNodeMigration)
	})

	if updateErr != nil {
		logger := log.FromContext(ctx)
		logger.Error(updateErr, "Failed to update CNINodeMigration status with error using retry")
		// Return the original error and the update error
		return ctrl.Result{}, fmt.Errorf("original error: %w, update error: %w", err, updateErr)
	}

	// Return the original error to trigger a requeue with backoff
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *CNIMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index pods by node name for efficient listing
	err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.Pod{},
		"spec.nodeName",
		func(rawObj client.Object) []string {
			pod := rawObj.(*corev1.Pod)
			return []string{pod.Spec.NodeName}
		},
	)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&cniswitcherv1alpha1.CNIMigration{}).      // Watch CNIMigration resources
		Owns(&cniswitcherv1alpha1.CNINodeMigration{}). // Watch owned CNINodeMigration resources
		Complete(r)
}
