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

package agent

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cnimigrationv1alpha1 "deckhouse.io/cni-migration/api/v1alpha1"
)

const (
	effectiveCNIAnnotation = "effective-cni.network.deckhouse.io"
)

// CNIAgentReconciler reconciles a CNINodeMigration object on a specific node
type CNIAgentReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	MigrationName string
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
			// If CNINodeMigration does not exist, check if we should create it.
			activeMigration := &cnimigrationv1alpha1.CNIMigration{}
			if err := r.Get(ctx, types.NamespacedName{Name: r.MigrationName}, activeMigration); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}

			// Create CNINodeMigration
			logger.Info("Creating CNINodeMigration for node", "node", nodeName)
			newNodeMigration := &cnimigrationv1alpha1.CNINodeMigration{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
			}

			// Set OwnerReference so that CNINodeMigration is automatically deleted
			// by Kubernetes Garbage Collector when the parent CNIMigration is deleted.
			if err = controllerutil.SetControllerReference(activeMigration, newNodeMigration, r.Scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to set owner reference: %w", err)
			}

			if err = r.Create(ctx, newNodeMigration); err != nil {
				return ctrl.Result{}, err
			}
			// Requeue to process the newly created resource
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	// Create a patch helper
	originalNodeMigration := nodeMigration.DeepCopy()

	// Fetch parent CNIMigration to know the TargetCNI and current state
	cniMigration := &cnimigrationv1alpha1.CNIMigration{}
	if err := r.Get(ctx, types.NamespacedName{Name: r.MigrationName}, cniMigration); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 0. Prepare Phase: continuously annotate pods.
	if r.hasParentCondition(cniMigration, cnimigrationv1alpha1.ConditionEnvironmentPrepared) &&
		!r.hasParentCondition(cniMigration, cnimigrationv1alpha1.ConditionCurrentCNIDisabled) {
		if nodeMigration.Status.Phase != cnimigrationv1alpha1.NodePhasePreparing {
			nodeMigration.Status.Phase = cnimigrationv1alpha1.NodePhasePreparing
			if err := r.Status().Patch(ctx, nodeMigration, client.MergeFrom(originalNodeMigration)); err != nil {
				return ctrl.Result{}, err
			}
			// Requeue after status update
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.ensurePodsAnnotated(ctx, nodeName, cniMigration.Status.CurrentCNI); err != nil {
			_ = r.setNodeCondition(
				ctx,
				nodeMigration,
				cnimigrationv1alpha1.NodeConditionPodsAnnotated,
				metav1.ConditionFalse,
				"Error",
				err.Error(),
			)
			return ctrl.Result{}, err
		}
		if err := r.setNodeCondition(
			ctx,
			nodeMigration,
			cnimigrationv1alpha1.NodeConditionPodsAnnotated,
			metav1.ConditionTrue,
			"Success",
			"Pods annotated",
		); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 1. Cleanup Phase: remove network artifacts of the current CNI.
	if !r.hasNodeCondition(nodeMigration, cnimigrationv1alpha1.NodeConditionCleanupDone) {
		// We cleanup when CurrentCNIDisabled is True in parent
		if r.hasParentCondition(cniMigration, cnimigrationv1alpha1.ConditionCurrentCNIDisabled) {
			if nodeMigration.Status.Phase != cnimigrationv1alpha1.NodePhaseCleaning {
				nodeMigration.Status.Phase = cnimigrationv1alpha1.NodePhaseCleaning
				if err := r.Status().Patch(ctx, nodeMigration, client.MergeFrom(originalNodeMigration)); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}

			logger.Info("Starting node cleanup")
			if err := RunCleanup(ctx, cniMigration.Status.CurrentCNI); err != nil {
				logger.Error(err, "Cleanup failed")
				_ = r.setNodeCondition(
					ctx,
					nodeMigration,
					cnimigrationv1alpha1.NodeConditionCleanupDone,
					metav1.ConditionFalse,
					"Error",
					err.Error(),
				)
				return ctrl.Result{}, err
			}
			logger.Info("Cleanup completed successfully")
			if err := r.setNodeCondition(
				ctx,
				nodeMigration,
				cnimigrationv1alpha1.NodeConditionCleanupDone,
				metav1.ConditionTrue,
				"Success",
				"Artifacts removed",
			); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// 2. Restart Pods Phase: delete pods that were annotated with the previous CNI.
	// We restart pods only after TargetCNIReady is True in parent.
	if r.hasParentCondition(cniMigration, cnimigrationv1alpha1.ConditionTargetCNIReady) {
		if !r.hasNodeCondition(nodeMigration, cnimigrationv1alpha1.NodeConditionPodsRestarted) {
			if nodeMigration.Status.Phase != cnimigrationv1alpha1.NodePhaseRestarting {
				nodeMigration.Status.Phase = cnimigrationv1alpha1.NodePhaseRestarting
				if err := r.Status().Patch(ctx, nodeMigration, client.MergeFrom(originalNodeMigration)); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}

			logger.Info("Starting pod restart on node")
			podList := &corev1.PodList{}
			if err := r.List(ctx, podList, client.MatchingFields{"spec.nodeName": nodeName}); err != nil {
				return ctrl.Result{}, err
			}

			podsDeleted := 0
			for _, pod := range podList.Items {
				if pod.Annotations[effectiveCNIAnnotation] == cniMigration.Status.CurrentCNI {
					if pod.DeletionTimestamp != nil {
						continue
					}
					if err := r.Delete(ctx, &pod); err != nil {
						if !errors.IsNotFound(err) {
							logger.Error(err, "Failed to delete pod", "pod", pod.Name, "namespace", pod.Namespace)
							_ = r.setNodeCondition(
								ctx,
								nodeMigration,
								cnimigrationv1alpha1.NodeConditionPodsRestarted,
								metav1.ConditionFalse,
								"Error",
								err.Error(),
							)
							return ctrl.Result{}, err
						}
					}
					logger.Info("Deleted pod", "pod", pod.Name, "namespace", pod.Namespace)
					podsDeleted++
				}
			}
			logger.Info("Finished deleting pods", "Count", podsDeleted)

			if err := r.setNodeCondition(
				ctx,
				nodeMigration,
				cnimigrationv1alpha1.NodeConditionPodsRestarted,
				metav1.ConditionTrue,
				"Success",
				fmt.Sprintf("%d pods restarted", podsDeleted),
			); err != nil {
				return ctrl.Result{}, err
			}
			// Final phase: Completed
			nodeMigration.Status.Phase = cnimigrationv1alpha1.NodePhaseCompleted
			if err := r.Status().Patch(ctx, nodeMigration, client.MergeFrom(originalNodeMigration)); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *CNIAgentReconciler) ensurePodsAnnotated(ctx context.Context, nodeName, currentCNI string) error {
	logger := log.FromContext(ctx)
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.MatchingFields{"spec.nodeName": nodeName}); err != nil {
		return err
	}

	for _, pod := range podList.Items {
		if pod.Spec.HostNetwork {
			continue
		}
		// Skip pods that are already terminated
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}

		if pod.Annotations[effectiveCNIAnnotation] == currentCNI {
			continue
		}

		patchedPod := pod.DeepCopy()
		if patchedPod.Annotations == nil {
			patchedPod.Annotations = make(map[string]string)
		}
		patchedPod.Annotations[effectiveCNIAnnotation] = currentCNI

		if err := r.Patch(ctx, patchedPod, client.MergeFrom(&pod)); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			logger.Error(err, "Failed to annotate pod", "pod", pod.Name, "namespace", pod.Namespace)
			return err
		}
		logger.Info("Annotated pod", "pod", pod.Name, "namespace", pod.Namespace)
	}
	return nil
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

func (r *CNIAgentReconciler) setNodeCondition(
	ctx context.Context,
	m *cnimigrationv1alpha1.CNINodeMigration,
	condType string,
	status metav1.ConditionStatus,
	reason, message string,
) error {
	// Create a local snapshot for patching
	original := m.DeepCopy()

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
			if c.Status == status && c.Reason == reason && c.Message == message {
				return nil
			}
			if c.Status == status && c.Reason == reason {
				newCond.LastTransitionTime = c.LastTransitionTime
			}
			m.Status.Conditions[i] = newCond
			found = true
			break
		}
	}
	if !found {
		m.Status.Conditions = append(m.Status.Conditions, newCond)
	}

	return r.Status().Patch(ctx, m, client.MergeFrom(original))
}

// SetupWithManager sets up the controller with the Manager.
func (r *CNIAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index pods by node name for efficient listing
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.Pod{},
		"spec.nodeName",
		func(rawObj client.Object) []string {
			pod := rawObj.(*corev1.Pod)
			return []string{pod.Spec.NodeName}
		},
	); err != nil {
		return err
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		// Assuming we can't run without NODE_NAME
		return fmt.Errorf("NODE_NAME env var is required")
	}

	// Map CNIMigration event to a request for this node's CNINodeMigration
	mapMigrationToRequest := func(ctx context.Context, obj client.Object) []reconcile.Request {
		// Only react to the specific migration we are configured for
		if obj.GetName() != r.MigrationName {
			return []reconcile.Request{}
		}
		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{
				Name: nodeName,
			}},
		}
	}

	// Map Pod event to a request for this node's CNINodeMigration
	mapPodToRequest := func(ctx context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{
				Name: nodeName,
			}},
		}
	}

	// Predicate to filter Pod events:
	// 1. Create: Always process new pods.
	// 2. Update: Process only if the pod is missing the annotation (e.g. user removed it).
	// 3. Delete: Ignore.
	// 4. Generic: Ignore.
	podPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew == nil {
				return false
			}
			_, hasAnnotation := e.ObjectNew.GetAnnotations()[effectiveCNIAnnotation]
			return !hasAnnotation
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&cnimigrationv1alpha1.CNINodeMigration{}).
		Watches(
			&cnimigrationv1alpha1.CNIMigration{},
			handler.EnqueueRequestsFromMapFunc(mapMigrationToRequest),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(mapPodToRequest),
			builder.WithPredicates(podPredicate),
		).
		Complete(r)
}
