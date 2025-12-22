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
	"strings"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cnimigrationv1alpha1 "deckhouse.io/cni-migration/api/v1alpha1"
)

const (
	EffectiveCNIAnnotation = "effective-cni.network.deckhouse.io"
)

// CNIMigrationReconciler reconciles a CNIMigration object
type CNIMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is the main manager loop
func (r *CNIMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the CNIMigration object
	cniMigration := &cnimigrationv1alpha1.CNIMigration{}
	if err := r.Get(ctx, req.NamespacedName, cniMigration); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Safety check: Ensure only one migration is active at a time.
	// We pick the oldest one as the "winner".
	allMigrations := &cnimigrationv1alpha1.CNIMigrationList{}
	if err := r.List(ctx, allMigrations); err != nil {
		return ctrl.Result{}, err
	}

	for _, m := range allMigrations.Items {
		if m.Name == cniMigration.Name {
			continue
		}
		// If there is an older migration that is not Succeeded/Failed, we wait.
		if m.CreationTimestamp.Before(&cniMigration.CreationTimestamp) {
			isFinished := false
			for _, cond := range m.Status.Conditions {
				if (cond.Type == cnimigrationv1alpha1.ConditionSucceeded) && cond.Status == metav1.ConditionTrue {
					isFinished = true
					break
				}
			}
			if !isFinished {
				return ctrl.Result{RequeueAfter: 30 * time.Second}, r.setCondition(ctx, cniMigration, "Conflict", metav1.ConditionTrue, "AnotherMigrationActive", fmt.Sprintf("Migration '%s' is already in progress. Waiting...", m.Name))
			}
		}
	}

	// State machine steps
	steps := []struct {
		condition string
		handler   func(context.Context, *cnimigrationv1alpha1.CNIMigration) (bool, error)
	}{
		{cnimigrationv1alpha1.ConditionValidated, r.ensureValidated},
		{cnimigrationv1alpha1.ConditionNamespaceReady, r.ensureNamespaceReady},
		{cnimigrationv1alpha1.ConditionComponentsReady, r.ensureComponentsReady},
		{cnimigrationv1alpha1.ConditionPodsAnnotated, r.ensurePodsAnnotated},
		{cnimigrationv1alpha1.ConditionTargetCNIEnabled, r.ensureTargetCNIEnabled},
		{cnimigrationv1alpha1.ConditionOldCNIDisabled, r.ensureOldCNIDisabled},
		{cnimigrationv1alpha1.ConditionNodesCleaned, r.ensureNodesCleaned},
		{cnimigrationv1alpha1.ConditionTargetCNIReady, r.ensureTargetCNIReady},
		{cnimigrationv1alpha1.ConditionWebhookDeleted, r.ensureWebhookDeleted},
		{cnimigrationv1alpha1.ConditionPodsRestarted, r.ensurePodsRestarted},
		{cnimigrationv1alpha1.ConditionSucceeded, r.ensureSucceeded},
	}

	for _, step := range steps {
		if r.hasCondition(cniMigration, step.condition) {
			continue
		}

		completed, err := step.handler(ctx, cniMigration)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !completed {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		if err := r.setCondition(ctx, cniMigration, step.condition, metav1.ConditionTrue, "StepCompleted", "Migration step completed successfully"); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *CNIMigrationReconciler) hasCondition(m *cnimigrationv1alpha1.CNIMigration, condType string) bool {
	for _, c := range m.Status.Conditions {
		if c.Type == condType && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func (r *CNIMigrationReconciler) setCondition(ctx context.Context, m *cnimigrationv1alpha1.CNIMigration, condType string, status metav1.ConditionStatus, reason, message string) error {
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

func (r *CNIMigrationReconciler) ensureValidated(_ context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	if m.Spec.TargetCNI == "" {
		return false, fmt.Errorf("targetCNI is not set")
	}
	// TODO: Add more validation (e.g., target CNI is supported)
	return true, nil
}

func (r *CNIMigrationReconciler) ensureNamespaceReady(_ context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	return true, nil
}

func (r *CNIMigrationReconciler) ensureComponentsReady(_ context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	return true, nil
}

func (r *CNIMigrationReconciler) ensurePodsAnnotated(ctx context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList); err != nil {
		return false, err
	}

	currentCNI := m.Status.CurrentCNI
	if currentCNI == "" {
		// We need to know current CNI to annotate.
		// TODO: Implement current CNI detection if not set
		return false, fmt.Errorf("currentCNI is not set in status")
	}

	podsToPatch := 0
	for _, pod := range podList.Items {
		if pod.Spec.HostNetwork {
			continue
		}
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		if pod.Annotations[EffectiveCNIAnnotation] == currentCNI {
			continue
		}

		patchedPod := pod.DeepCopy()
		if patchedPod.Annotations == nil {
			patchedPod.Annotations = make(map[string]string)
		}
		patchedPod.Annotations[EffectiveCNIAnnotation] = currentCNI

		if err := r.Patch(ctx, patchedPod, client.MergeFrom(&pod)); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return false, err
		}
		podsToPatch++
	}

	return podsToPatch == 0, nil
}

func (r *CNIMigrationReconciler) ensureTargetCNIEnabled(ctx context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	moduleName := "cni-" + strings.ToLower(m.Spec.TargetCNI)
	return r.toggleModule(ctx, moduleName, true)
}

func (r *CNIMigrationReconciler) ensureOldCNIDisabled(ctx context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	moduleName := "cni-" + strings.ToLower(m.Status.CurrentCNI)
	return r.toggleModule(ctx, moduleName, false)
}

func (r *CNIMigrationReconciler) toggleModule(ctx context.Context, moduleName string, enabled bool) (bool, error) {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    "ModuleConfig",
	})

	err := r.Get(ctx, types.NamespacedName{Name: moduleName}, mc)
	if err != nil {
		if errors.IsNotFound(err) {
			// If we are enabling and it's not found, maybe it's not supposed to be there or handled differently
			// In Deckhouse, modules might not have ModuleConfig if they are using defaults.
			// But for CNI we usually have them.
			return false, fmt.Errorf("ModuleConfig %s not found", moduleName)
		}
		return false, err
	}

	spec, found, err := unstructured.NestedMap(mc.Object, "spec")
	if err != nil {
		return false, err
	}
	if !found {
		spec = make(map[string]any)
	}

	if currentEnabled, ok := spec["enabled"].(bool); ok && currentEnabled == enabled {
		return true, nil
	}

	spec["enabled"] = enabled
	if err := unstructured.SetNestedMap(mc.Object, spec, "spec"); err != nil {
		return false, err
	}

	if err := r.Update(ctx, mc); err != nil {
		return false, err
	}

	return true, nil
}

func (r *CNIMigrationReconciler) ensureNodesCleaned(ctx context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	nodeMigrations := &cnimigrationv1alpha1.CNINodeMigrationList{}
	if err := r.List(ctx, nodeMigrations); err != nil {
		return false, err
	}

	// Get total nodes in cluster
	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes); err != nil {
		return false, err
	}

	if len(nodeMigrations.Items) < len(nodes.Items) {
		// Not all nodes have started migration yet
		return false, nil
	}

	cleanedNodes := 0
	for _, nm := range nodeMigrations.Items {
		for _, cond := range nm.Status.Conditions {
			if cond.Type == cnimigrationv1alpha1.NodeConditionCleanupDone && cond.Status == metav1.ConditionTrue {
				cleanedNodes++
				break
			}
		}
	}

	// Update stats in status
	m.Status.NodesTotal = len(nodes.Items)
	m.Status.NodesSucceeded = cleanedNodes
	// TODO: Handle failed nodes

	return cleanedNodes >= len(nodes.Items), nil
}

func (r *CNIMigrationReconciler) ensureTargetCNIReady(ctx context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	moduleName := "cni-" + strings.ToLower(m.Spec.TargetCNI)
	dsName := ""
	switch moduleName {
	case "cni-cilium":
		dsName = "agent"
	case "cni-flannel":
		dsName = "flannel"
	case "cni-simple-bridge":
		dsName = "simple-bridge"
	default:
		return false, fmt.Errorf("unknown module name: %s", moduleName)
	}

	ds := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{Name: dsName, Namespace: "d8-" + moduleName}, ds)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if ds.Status.DesiredNumberScheduled == 0 {
		return false, nil
	}

	return ds.Status.NumberReady >= ds.Status.DesiredNumberScheduled, nil
}

func (r *CNIMigrationReconciler) ensureWebhookDeleted(ctx context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{}
	webhook.Name = "cni-migration-webhook"

	if err := r.Delete(ctx, webhook); err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	return true, nil
}

func (r *CNIMigrationReconciler) ensurePodsRestarted(ctx context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	nodeMigrations := &cnimigrationv1alpha1.CNINodeMigrationList{}
	if err := r.List(ctx, nodeMigrations); err != nil {
		return false, err
	}

	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes); err != nil {
		return false, err
	}

	restartedNodes := 0
	for _, nm := range nodeMigrations.Items {
		for _, cond := range nm.Status.Conditions {
			if cond.Type == "PodsRestarted" && cond.Status == metav1.ConditionTrue {
				restartedNodes++
				break
			}
		}
	}

	return restartedNodes >= len(nodes.Items), nil
}

func (r *CNIMigrationReconciler) ensureSucceeded(_ context.Context, m *cnimigrationv1alpha1.CNIMigration) (bool, error) {
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CNIMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cnimigrationv1alpha1.CNIMigration{}).
		Owns(&cnimigrationv1alpha1.CNINodeMigration{}).
		Complete(r)
}
