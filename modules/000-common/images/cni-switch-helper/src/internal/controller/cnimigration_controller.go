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

	cniswitcherv1alpha1 "deckhouse.io/cni-switch-helper/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	EffectiveCNIAnnotation = "effective-cni.network.deckhouse.io"
)

// CNIMigrationReconciler reconciles a CNIMigration object
type CNIMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=network.deckhouse.io,resources=cnimigrations,verbs=get;list;watch
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=cnimigrations/status,verbs=get
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=cninodemigrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=cninodemigrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;patch;update;delete

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
	case "Cleanup":
		logger.Info("Cleanup phase not yet implemented, skipping reconciliation.")
		// TODO: Implement Cleanup phase
	case "Abort":
		logger.Info("Abort phase not yet implemented, skipping reconciliation.")
		// TODO: Implement Abort phase
	default:
		logger.Info("Unknown CNIMigration phase, no action taken", "Phase", cniMigration.Spec.Phase)
	}

	return ctrl.Result{}, nil
}

func (r *CNIMigrationReconciler) reconcilePrepare(ctx context.Context, cniMigration *cniswitcherv1alpha1.CNIMigration, cniNodeMigration *cniswitcherv1alpha1.CNINodeMigration) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if preparation is already done for this node
	for _, cond := range cniNodeMigration.Status.Conditions {
		if cond.Type == "PreparationSucceeded" && cond.Status == metav1.ConditionTrue {
			logger.Info("Preparation already completed for this node")
			return ctrl.Result{}, nil
		}
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
	for _, pod := range podList.Items {
		if pod.Spec.HostNetwork || pod.Status.Phase != corev1.PodRunning {
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
			// TODO: Update status with error and reason
			return ctrl.Result{}, err
		}
		podsAnnotated++
	}

	logger.Info("Finished annotating pods", "AnnotatedCount", podsAnnotated)

	// Update status to reflect completion
	cniNodeMigration.Status.Phase = "Prepared"
	cniNodeMigration.Status.PodsToRestartCount = totalPodsToProcess
	cniNodeMigration.Status.Conditions = append(cniNodeMigration.Status.Conditions, metav1.Condition{
		Type:               "PreparationSucceeded",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "PodsAnnotated",
		Message:            fmt.Sprintf("%d pods on the node have been annotated.", podsAnnotated),
	})

	if err := r.Status().Update(ctx, cniNodeMigration); err != nil {
		logger.Error(err, "Failed to update CNINodeMigration status")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully completed Prepare phase for node")
	return ctrl.Result{}, nil
}

func (r *CNIMigrationReconciler) reconcileMigrate(ctx context.Context, cniMigration *cniswitcherv1alpha1.CNIMigration, cniNodeMigration *cniswitcherv1alpha1.CNINodeMigration) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Run node cleanup
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
		cniNodeMigration.Status.Conditions = append(cniNodeMigration.Status.Conditions, metav1.Condition{
			Type:               "CleanupSucceeded",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "OldCNIArtifactsRemoved",
			Message:            fmt.Sprintf("Artifacts for CNI '%s' were successfully removed.", cniMigration.Status.CurrentCNI),
		})

		if err := r.Status().Update(ctx, cniNodeMigration); err != nil {
			logger.Error(err, "Failed to update CNINodeMigration status after cleanup")
			return ctrl.Result{}, err
		}
		// Requeue to proceed to the next step (pod restart)
		return ctrl.Result{Requeue: true}, nil
	}

	// Step 2: Restart Pods
	logger.Info("Starting Pod restart for node")

	// Check if pods have already been restarted
	for _, cond := range cniNodeMigration.Status.Conditions {
		if cond.Type == "PodsRestarted" && cond.Status == metav1.ConditionTrue {
			logger.Info("Pods have already been restarted on this node")
			// Final step, migration on this node is complete
			cniNodeMigration.Status.Phase = "Succeeded"
			if err := r.Status().Update(ctx, cniNodeMigration); err != nil {
				logger.Error(err, "Failed to update CNINodeMigration status to Succeeded")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.MatchingFields{"spec.nodeName": cniNodeMigration.Name}); err != nil {
		logger.Error(err, "Failed to list pods on node for restart")
		return r.updateNodeStatusWithError(ctx, cniNodeMigration, "PodListFailed", err)
	}

	podsDeleted := 0
	for _, pod := range podList.Items {
		if pod.Annotations[EffectiveCNIAnnotation] == cniMigration.Status.CurrentCNI {
			if err := r.Delete(ctx, &pod); err != nil {
				logger.Error(err, "Failed to delete pod for restart", "Pod", pod.Name)
				return r.updateNodeStatusWithError(ctx, cniNodeMigration, "PodDeletionFailed", err)
			}
			podsDeleted++
		}
	}

	logger.Info("Finished deleting pods", "DeletedCount", podsDeleted)

	// Update status to reflect pod restart completion
	cniNodeMigration.Status.Conditions = append(cniNodeMigration.Status.Conditions, metav1.Condition{
		Type:               "PodsRestarted",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "OldPodsDeleted",
		Message:            fmt.Sprintf("%d pods with old CNI annotation were deleted.", podsDeleted),
	})

	if err := r.Status().Update(ctx, cniNodeMigration); err != nil {
		logger.Error(err, "Failed to update CNINodeMigration status after pod deletion")
		return ctrl.Result{}, err
	}

	// Requeue to run the check again, which will then mark the node as Succeeded
	return ctrl.Result{Requeue: true}, nil
}

func (r *CNIMigrationReconciler) updateNodeStatusWithError(ctx context.Context, cniNodeMigration *cniswitcherv1alpha1.CNINodeMigration, reason string, err error) (ctrl.Result, error) {
	cniNodeMigration.Status.Phase = "Failed"
	cniNodeMigration.Status.Conditions = append(cniNodeMigration.Status.Conditions, metav1.Condition{
		Type:               "Failed",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            err.Error(),
	})

	if updateErr := r.Status().Update(ctx, cniNodeMigration); updateErr != nil {
		logger := log.FromContext(ctx)
		logger.Error(updateErr, "Failed to update CNINodeMigration status with error")
		// Return the original error and the update error
		return ctrl.Result{}, fmt.Errorf("original error: %w, update error: %w", err, updateErr)
	}

	// Return the original error to trigger a requeue with backoff
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *CNIMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index pods by node name for efficient listing
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod := rawObj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&cniswitcherv1alpha1.CNIMigration{}). // Watch CNIMigration resources
		Owns(&cniswitcherv1alpha1.CNINodeMigration{}). // Watch owned CNINodeMigration resources
		Complete(r)
}
