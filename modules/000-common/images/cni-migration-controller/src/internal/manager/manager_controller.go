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

package manager

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	cniMigrationPhasePause = "cni-migration.network.deckhouse.io/pause-before-phase"
)

// CNIDaemonSetMap maps short CNI names to their DaemonSet names.
var CNIDaemonSetMap = map[string]string{
	cnimigrationv1alpha1.CNINameCilium:       "agent",
	cnimigrationv1alpha1.CNINameFlannel:      "flannel",
	cnimigrationv1alpha1.CNINameSimpleBridge: "simple-bridge",
}

// CNIMigrationReconciler reconciles a CNIMigration object
type CNIMigrationReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	MigrationName   string
	WaitForWebhooks string
}

// Reconcile is the main manager loop
func (r *CNIMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// If MigrationName is set, ignore other migrations
	if r.MigrationName != "" && req.Name != r.MigrationName {
		return ctrl.Result{}, nil
	}

	// Fetch the CNIMigration object
	cniMigration := &cnimigrationv1alpha1.CNIMigration{}
	if err := r.Get(ctx, req.NamespacedName, cniMigration); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Create a patch helper
	originalCNIMigration := cniMigration.DeepCopy()

	// Detect Current CNI if not set
	if cniMigration.Status.CurrentCNI == "" {
		currentCNI, err := r.detectCurrentCNI(ctx)
		if err != nil {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, r.setCondition(
				ctx,
				cniMigration,
				cnimigrationv1alpha1.ConditionCurrentCNIDetectionFailed,
				metav1.ConditionTrue,
				"CNIDetectionError",
				err.Error(),
			)
		}
		cniMigration.Status.CurrentCNI = currentCNI
		if err := r.Status().Patch(ctx, cniMigration, client.MergeFrom(originalCNIMigration)); err != nil {
			return ctrl.Result{}, err
		}
		// Requeue to process with updated status
		return ctrl.Result{Requeue: true}, nil
	}

	// Always update node statistics (Failed, Succeeded count)
	oldStatus := cniMigration.Status.DeepCopy()
	if err := r.updateNodeStatistics(ctx, cniMigration); err != nil {
		return ctrl.Result{}, err
	}

	// If statistics changed, update status immediately to reflect progress/errors
	if r.isNodeStatisticsChanged(oldStatus, &cniMigration.Status) {
		if err := r.Status().Patch(ctx, cniMigration, client.MergeFrom(originalCNIMigration)); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check for pause annotation
	if val, ok := cniMigration.Annotations[cniMigrationPhasePause]; ok {
		if val == cniMigration.Status.Phase {
			ctrl.Log.Info("Migration paused by annotation", "phase", cniMigration.Status.Phase)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// State machine steps
	steps := []struct {
		condition string
		phase     string
		handler   func(context.Context, *cnimigrationv1alpha1.CNIMigration) (bool, string, error)
	}{
		{
			condition: cnimigrationv1alpha1.ConditionEnvironmentPrepared,
			phase:     cnimigrationv1alpha1.PhasePreparing,
			handler:   r.ensureEnvironmentPrepared,
		},
		{
			condition: cnimigrationv1alpha1.ConditionAgentsReady,
			phase:     cnimigrationv1alpha1.PhaseWaitingForAgents,
			handler:   r.ensureAgentsReady,
		},
		{
			condition: cnimigrationv1alpha1.ConditionTargetCNIEnabled,
			phase:     cnimigrationv1alpha1.PhaseEnablingTargetCNI,
			handler:   r.ensureTargetCNIEnabled,
		},
		{
			condition: cnimigrationv1alpha1.ConditionCurrentCNIDisabled,
			phase:     cnimigrationv1alpha1.PhaseDisablingCurrentCNI,
			handler:   r.ensureCurrentCNIDisabled,
		},
		{
			condition: cnimigrationv1alpha1.ConditionNodesCleaned,
			phase:     cnimigrationv1alpha1.PhaseCleaningNodes,
			handler:   r.ensureNodesCleaned,
		},
		{
			condition: cnimigrationv1alpha1.ConditionTargetCNIReady,
			phase:     cnimigrationv1alpha1.PhaseWaitingTargetCNI,
			handler:   r.ensureTargetCNIReady,
		},
		{
			condition: cnimigrationv1alpha1.ConditionPodsRestarted,
			phase:     cnimigrationv1alpha1.PhaseRestartingPods,
			handler:   r.ensurePodsRestarted,
		},
		{
			condition: cnimigrationv1alpha1.ConditionSucceeded,
			phase:     cnimigrationv1alpha1.PhaseCompleted,
			handler:   r.ensureSucceeded,
		},
	}

	for _, step := range steps {
		if r.hasCondition(cniMigration, step.condition) {
			continue
		}

		// Update Phase to current step
		if cniMigration.Status.Phase != step.phase {
			cniMigration.Status.Phase = step.phase
			if err := r.Status().Patch(ctx, cniMigration, client.MergeFrom(originalCNIMigration)); err != nil {
				return ctrl.Result{}, err
			}
			// Requeue to process with new Phase
			return ctrl.Result{Requeue: true}, nil
		}

		completed, msg, err := step.handler(ctx, cniMigration)
		if err != nil {
			// Set Condition to False with Error reason
			_ = r.setCondition(
				ctx,
				cniMigration,
				step.condition,
				metav1.ConditionFalse,
				"Error",
				err.Error(),
			)
			return ctrl.Result{}, err
		}

		if !completed {
			if msg == "" {
				msg = "Step in progress"
			}
			// Step started but not finished. Set Condition to False with InProgress reason.
			if err := r.setCondition(
				ctx,
				cniMigration,
				step.condition,
				metav1.ConditionFalse,
				"InProgress",
				msg,
			); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		// Success
		if err := r.setCondition(
			ctx,
			cniMigration,
			step.condition,
			metav1.ConditionTrue,
			"Success",
			"Step completed successfully",
		); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *CNIMigrationReconciler) updateNodeStatistics(ctx context.Context, m *cnimigrationv1alpha1.CNIMigration) error {
	// 1. Get Node list to know Total expected
	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes); err != nil {
		return err
	}
	m.Status.NodesTotal = len(nodes.Items)

	// 2. Get CNINodeMigrations
	nodeMigrations := &cnimigrationv1alpha1.CNINodeMigrationList{}
	listOpts := []client.ListOption{}
	if m.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(m.Namespace))
	}
	if err := r.List(ctx, nodeMigrations, listOpts...); err != nil {
		return err
	}

	failed := 0
	var failedSummary []cnimigrationv1alpha1.FailedNodeSummary

	for _, nm := range nodeMigrations.Items {
		for _, c := range nm.Status.Conditions {
			// Check for any condition that is explicitly False with reason Error
			if c.Status == metav1.ConditionFalse && c.Reason == "Error" {
				failed++
				failedSummary = append(failedSummary, cnimigrationv1alpha1.FailedNodeSummary{
					Node:   nm.Name,
					Reason: fmt.Sprintf("[%s] %s", nm.Status.Phase, c.Message),
				})
				break
			}
		}
	}

	m.Status.NodesFailed = failed
	m.Status.FailedSummary = failedSummary

	return nil
}

func (r *CNIMigrationReconciler) isNodeStatisticsChanged(old, new *cnimigrationv1alpha1.CNIMigrationStatus) bool {
	if old.NodesTotal != new.NodesTotal {
		return true
	}
	if old.NodesFailed != new.NodesFailed {
		return true
	}
	if old.NodesSucceeded != new.NodesSucceeded {
		return true
	}
	if len(old.FailedSummary) != len(new.FailedSummary) {
		return true
	}
	return false
}

func (r *CNIMigrationReconciler) hasCondition(m *cnimigrationv1alpha1.CNIMigration, condType string) bool {
	for _, c := range m.Status.Conditions {
		if c.Type == condType && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func (r *CNIMigrationReconciler) setCondition(
	ctx context.Context,
	m *cnimigrationv1alpha1.CNIMigration,
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
			// Update LastTransitionTime if Status OR Reason changed.
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

func (r *CNIMigrationReconciler) detectCurrentCNI(ctx context.Context) (string, error) {
	var enabledCNIs []string
	for cni := range CNIDaemonSetMap {
		moduleName := "cni-" + cni
		enabled, err := r.isModuleEnabled(ctx, moduleName)
		if err != nil {
			continue // Skip errors, maybe module doesn't exist
		}
		if enabled {
			enabledCNIs = append(enabledCNIs, cni)
		}
	}

	if len(enabledCNIs) == 0 {
		return "", fmt.Errorf("could not detect any enabled CNI module")
	}

	if len(enabledCNIs) > 1 {
		return "", fmt.Errorf("multiple CNI modules are enabled: %v", enabledCNIs)
	}

	return enabledCNIs[0], nil
}

func (r *CNIMigrationReconciler) isModuleEnabled(ctx context.Context, moduleName string) (bool, error) {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    "ModuleConfig",
	})
	if err := r.Get(ctx, types.NamespacedName{Name: moduleName}, mc); err != nil {
		return false, err
	}

	spec, found, err := unstructured.NestedMap(mc.Object, "spec")
	if err != nil || !found {
		return false, err
	}
	if enabled, ok := spec["enabled"].(bool); ok {
		return enabled, nil
	}
	return false, nil
}

func (r *CNIMigrationReconciler) ensureEnvironmentPrepared(
	ctx context.Context,
	m *cnimigrationv1alpha1.CNIMigration,
) (bool, string, error) {
	if m.Spec.TargetCNI == "" {
		return false, "", fmt.Errorf("targetCNI is not set")
	}

	targetCNI := strings.ToLower(m.Spec.TargetCNI)
	if _, ok := CNIDaemonSetMap[targetCNI]; !ok {
		return false, "", fmt.Errorf("unsupported target CNI: %s", m.Spec.TargetCNI)
	}

	if strings.EqualFold(m.Spec.TargetCNI, m.Status.CurrentCNI) {
		return false, "", fmt.Errorf("target CNI (%s) is same as current CNI", m.Spec.TargetCNI)
	}

	// Also wait for webhooks to be disabled here, to ensure environment is ready before starting
	if r.WaitForWebhooks != "" {
		if disabled, msg, err := r.checkWebhooksDisabled(ctx); err != nil {
			return false, "", err
		} else if !disabled {
			return false, msg, nil
		}
	}

	return true, "", nil
}

func (r *CNIMigrationReconciler) ensureAgentsReady(
	ctx context.Context,
	m *cnimigrationv1alpha1.CNIMigration,
) (bool, string, error) {
	nodeMigrations := &cnimigrationv1alpha1.CNINodeMigrationList{}
	if err := r.List(ctx, nodeMigrations); err != nil {
		return false, "", err
	}

	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes); err != nil {
		return false, "", err
	}

	if len(nodeMigrations.Items) < len(nodes.Items) {
		return false, fmt.Sprintf(
			"Waiting for node registrations: %d/%d",
			len(nodeMigrations.Items),
			len(nodes.Items),
		), nil
	}

	readyAgents := 0
	for _, nm := range nodeMigrations.Items {
		isReady := false
		for _, cond := range nm.Status.Conditions {
			if cond.Type == cnimigrationv1alpha1.NodeConditionPodsAnnotated && cond.Status == metav1.ConditionTrue {
				readyAgents++
				isReady = true
				break
			}
		}
		if !isReady {
			ctrl.Log.Info("Agent not ready (pods not annotated)", "node", nm.Name, "conditions", nm.Status.Conditions)
		}
	}

	if readyAgents < len(nodes.Items) {
		return false, fmt.Sprintf(
			"Waiting for agents to be prepared (pods annotated): %d/%d",
			readyAgents,
			len(nodes.Items),
		), nil
	}

	return true, "", nil
}

func (r *CNIMigrationReconciler) ensureTargetCNIEnabled(
	ctx context.Context,
	m *cnimigrationv1alpha1.CNIMigration,
) (bool, string, error) {
	targetCNI := strings.ToLower(m.Spec.TargetCNI)
	moduleName := "cni-" + targetCNI

	// 1. Enable module
	done, err := r.toggleModule(ctx, moduleName, true)
	if err != nil {
		return false, "", err
	}
	if !done {
		return false, fmt.Sprintf("Enabling module %s...", moduleName), nil
	}

	// 2. Wait for DaemonSet to appear and schedule pods
	dsName, ok := CNIDaemonSetMap[targetCNI]
	if !ok {
		return false, "", fmt.Errorf("unknown CNI: %s", targetCNI)
	}
	dsNamespace := "d8-" + moduleName

	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: dsName, Namespace: dsNamespace}, ds); err != nil {
		if errors.IsNotFound(err) {
			return false, fmt.Sprintf("Waiting for %s DaemonSet creation (module %s)...", dsName, moduleName), nil
		}
		return false, "", err
	}

	if ds.Status.DesiredNumberScheduled == 0 {
		return false, fmt.Sprintf("Waiting for %s pods to be scheduled (module %s)...", dsName, moduleName), nil
	}

	// 3. Verify that pods are actually created and scheduled
	pods := &corev1.PodList{}
	if err := r.List(
		ctx,
		pods,
		client.InNamespace(dsNamespace),
		client.MatchingLabels(ds.Spec.Selector.MatchLabels),
	); err != nil {
		return false, "", err
	}

	if len(pods.Items) < int(ds.Status.DesiredNumberScheduled) {
		return false, fmt.Sprintf(
			"Waiting for %s pods creation (module %s): %d/%d",
			dsName,
			moduleName,
			len(pods.Items),
			ds.Status.DesiredNumberScheduled,
		), nil
	}

	for _, pod := range pods.Items {
		if pod.Spec.NodeName == "" {
			return false, fmt.Sprintf("Waiting for pod %s to be scheduled on a node", pod.Name), nil
		}

		// Check InitContainerStatuses to ensure images are pulling and no early crashes occur.
		if len(pod.Status.InitContainerStatuses) == 0 {
			if pod.Status.Phase == corev1.PodPending {
				return false, fmt.Sprintf("Waiting for pod %s init containers to start...", pod.Name), nil
			}
		}

		for _, status := range pod.Status.InitContainerStatuses {
			// Check for waiting errors
			if status.State.Waiting != nil {
				reason := status.State.Waiting.Reason
				if isCriticalWaitingReason(reason) {
					return false, "", fmt.Errorf("pod %s init container %s failed: %s", pod.Name, status.Name, reason)
				}
			}
			// Check for terminated errors
			if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
				return false, "", fmt.Errorf(
					"pod %s init container %s terminated with error (exit code %d)",
					pod.Name,
					status.Name,
					status.State.Terminated.ExitCode,
				)
			}
		}
	}

	return true, "", nil
}

func isCriticalWaitingReason(reason string) bool {
	switch reason {
	case "ErrImagePull", "ImagePullBackOff", "CrashLoopBackOff", "CreateContainerConfigError", "InvalidImageName":
		return true
	}
	return false
}

func (r *CNIMigrationReconciler) ensureCurrentCNIDisabled(
	ctx context.Context,
	m *cnimigrationv1alpha1.CNIMigration,
) (bool, string, error) {
	currentCNI := strings.ToLower(m.Status.CurrentCNI)
	moduleName := "cni-" + currentCNI

	// 1. Disable module
	done, err := r.toggleModule(ctx, moduleName, false)
	if err != nil {
		return false, "", err
	}
	if !done {
		return false, fmt.Sprintf("Disabling module %s...", moduleName), nil
	}

	// 2. Wait for DaemonSet to be deleted
	dsName, ok := CNIDaemonSetMap[currentCNI]
	if !ok {
		return true, "", nil
	}
	dsNamespace := "d8-" + moduleName

	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: dsName, Namespace: dsNamespace}, ds); err != nil {
		if errors.IsNotFound(err) {
			return true, "", nil
		}
		return false, "", err
	}

	// DaemonSet still exists
	return false, fmt.Sprintf("Waiting for %s DaemonSet deletion (module %s)...", dsName, moduleName), nil
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

func (r *CNIMigrationReconciler) ensureNodesCleaned(
	ctx context.Context,
	m *cnimigrationv1alpha1.CNIMigration,
) (bool, string, error) {
	nodeMigrations := &cnimigrationv1alpha1.CNINodeMigrationList{}
	if err := r.List(ctx, nodeMigrations); err != nil {
		return false, "", err
	}

	// Get total nodes in cluster
	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes); err != nil {
		return false, "", err
	}

	cleanedNodes := 0
	for _, nm := range nodeMigrations.Items {
		isCleaned := false
		for _, cond := range nm.Status.Conditions {
			if cond.Type == cnimigrationv1alpha1.NodeConditionCleanupDone && cond.Status == metav1.ConditionTrue {
				cleanedNodes++
				isCleaned = true
				break
			}
		}
		if !isCleaned {
			ctrl.Log.Info("Node not cleaned yet", "node", nm.Name, "conditions", nm.Status.Conditions)
		}
	}

	// Update stats in status
	m.Status.NodesTotal = len(nodes.Items)
	m.Status.NodesSucceeded = cleanedNodes

	if len(nodeMigrations.Items) < len(nodes.Items) {
		return false, fmt.Sprintf(
			"Waiting for node registrations: %d/%d",
			len(nodeMigrations.Items),
			len(nodes.Items),
		), nil
	}

	if cleanedNodes < len(nodes.Items) {
		return false, fmt.Sprintf("Waiting for nodes cleanup: %d/%d succeeded", cleanedNodes, len(nodes.Items)), nil
	}

	return true, "", nil
}

func (r *CNIMigrationReconciler) ensureTargetCNIReady(
	ctx context.Context,
	m *cnimigrationv1alpha1.CNIMigration,
) (bool, string, error) {
	targetCNI := strings.ToLower(m.Spec.TargetCNI)
	moduleName := "cni-" + targetCNI
	dsName, ok := CNIDaemonSetMap[targetCNI]
	if !ok {
		return false, "", fmt.Errorf("unknown module name: %s", moduleName)
	}

	ds := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{Name: dsName, Namespace: "d8-" + moduleName}, ds)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, fmt.Sprintf("Waiting for target CNI DaemonSet to be created (module %s)...", moduleName), nil
		}
		return false, "", err
	}

	if ds.Status.DesiredNumberScheduled == 0 {
		return false, fmt.Sprintf("Waiting for target CNI DaemonSet to schedule pods (module %s)...", moduleName), nil
	}

	if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
		return false, fmt.Sprintf(
			"Waiting for target CNI pods ready (module %s): %d/%d",
			moduleName,
			ds.Status.NumberReady,
			ds.Status.DesiredNumberScheduled,
		), nil
	}

	return true, "", nil
}

func (r *CNIMigrationReconciler) ensurePodsRestarted(
	ctx context.Context,
	_ *cnimigrationv1alpha1.CNIMigration,
) (bool, string, error) {
	nodeMigrations := &cnimigrationv1alpha1.CNINodeMigrationList{}
	if err := r.List(ctx, nodeMigrations); err != nil {
		return false, "", err
	}

	nodes := &corev1.NodeList{}
	if err := r.List(ctx, nodes); err != nil {
		return false, "", err
	}

	restartedNodes := 0
	for _, nm := range nodeMigrations.Items {
		isRestarted := false
		for _, cond := range nm.Status.Conditions {
			if cond.Type == cnimigrationv1alpha1.NodeConditionPodsRestarted && cond.Status == metav1.ConditionTrue {
				restartedNodes++
				isRestarted = true
				break
			}
		}
		if !isRestarted {
			ctrl.Log.Info("Node pods restart pending", "node", nm.Name, "conditions", nm.Status.Conditions)
		}
	}

	if restartedNodes < len(nodes.Items) {
		return false, fmt.Sprintf(
			"Waiting for pod restarts: %d/%d nodes completed",
			restartedNodes,
			len(nodes.Items),
		), nil
	}

	// After all pods are restarted, ensure that critical webhook pods are Ready
	if ready, msg, err := r.checkWebhookPodsReady(ctx); err != nil {
		return false, "", err
	} else if !ready {
		return false, msg, nil
	}

	return true, "", nil
}

func (r *CNIMigrationReconciler) ensureSucceeded(
	ctx context.Context,
	m *cnimigrationv1alpha1.CNIMigration,
) (bool, string, error) {
	ctrl.Log.Info("Migration successfully completed", "migration", m.Name)
	return true, "", nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CNIMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cnimigrationv1alpha1.CNIMigration{}).
		Owns(&cnimigrationv1alpha1.CNINodeMigration{}).
		Complete(r)
}

func (r *CNIMigrationReconciler) checkWebhooksDisabled(ctx context.Context) (bool, string, error) {
	for name := range strings.SplitSeq(r.WaitForWebhooks, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		kinds := []string{"ValidatingWebhookConfiguration", "MutatingWebhookConfiguration"}
		for _, kind := range kinds {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "admissionregistration.k8s.io",
				Version: "v1",
				Kind:    kind,
			})

			err := r.Get(ctx, types.NamespacedName{Name: name}, obj)
			if err != nil {
				if errors.IsNotFound(err) {
					// Resource deleted - OK
					continue
				}
				return false, "", fmt.Errorf("failed to check %s %s: %w", kind, name, err)
			}

			// Resource exists, check failurePolicy
			webhooks, found, err := unstructured.NestedSlice(obj.Object, "webhooks")
			if err != nil {
				return false, "", fmt.Errorf("failed to parse webhooks for %s %s: %w", kind, name, err)
			}
			if !found {
				// No webhooks defined - treat as safe
				continue
			}

			for i, w := range webhooks {
				webhookMap, ok := w.(map[string]any)
				if !ok {
					return false, "", fmt.Errorf("webhook #%d in %s %s has invalid format", i, kind, name)
				}
				failurePolicy, _, _ := unstructured.NestedString(webhookMap, "failurePolicy")
				if failurePolicy != "Ignore" {
					// Found a blocking webhook
					return false, fmt.Sprintf(
						"Waiting for %s %s (policy: %s) to be disabled or removed by Helm...",
						kind,
						name,
						failurePolicy,
					), nil
				}
			}
		}
	}

	return true, "", nil
}

func (r *CNIMigrationReconciler) checkWebhookPodsReady(ctx context.Context) (bool, string, error) {
	if r.WaitForWebhooks == "" {
		return true, "", nil
	}

	for name := range strings.SplitSeq(r.WaitForWebhooks, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// Check both Validating and Mutating
		kinds := []string{"ValidatingWebhookConfiguration", "MutatingWebhookConfiguration"}
		for _, kind := range kinds {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "admissionregistration.k8s.io",
				Version: "v1",
				Kind:    kind,
			})

			err := r.Get(ctx, types.NamespacedName{Name: name}, obj)
			if err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				return false, "", fmt.Errorf("failed to get %s %s: %w", kind, name, err)
			}

			webhooks, found, err := unstructured.NestedSlice(obj.Object, "webhooks")
			if err != nil {
				return false, "", fmt.Errorf("failed to parse webhooks for %s %s: %w", kind, name, err)
			}
			if !found {
				continue
			}

			for i, w := range webhooks {
				webhook, ok := w.(map[string]any)
				if !ok {
					return false, "", fmt.Errorf("webhook #%d in %s %s has invalid format", i, kind, name)
				}

				clientConfig, found, _ := unstructured.NestedMap(webhook, "clientConfig")
				if !found {
					continue
				}

				svcRef, found, _ := unstructured.NestedMap(clientConfig, "service")
				if !found {
					// URL-based webhook, cannot check pod readiness
					continue
				}

				ns, _, _ := unstructured.NestedString(svcRef, "namespace")
				svcName, _, _ := unstructured.NestedString(svcRef, "name")

				if ns == "" || svcName == "" {
					return false, "", fmt.Errorf("webhook #%d in %s %s has invalid service reference", i, kind, name)
				}

				// Get Service to find selector
				svc := &corev1.Service{}
				if err := r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: ns}, svc); err != nil {
					if errors.IsNotFound(err) {
						return false, fmt.Sprintf("Waiting for service %s/%s for webhook %s...", ns, svcName, name), nil
					}
					return false, "", err
				}

				if len(svc.Spec.Selector) == 0 {
					// No selector
					continue
				}

				// Check Pods
				pods := &corev1.PodList{}
				if err := r.List(
					ctx,
					pods,
					client.InNamespace(ns),
					client.MatchingLabels(svc.Spec.Selector),
				); err != nil {
					return false, "", err
				}

				if len(pods.Items) == 0 {
					return false, fmt.Sprintf(
						"Waiting for pods for webhook %s (service %s/%s)...",
						name,
						ns,
						svcName,
					), nil
				}

				anyReady := false
				for _, pod := range pods.Items {
					if isPodReady(&pod) {
						anyReady = true
						break
					}
				}

				if !anyReady {
					return false, fmt.Sprintf(
						"Waiting for ready pods for webhook %s (service %s/%s)...",
						name,
						ns,
						svcName,
					), nil
				}
			}
		}
	}
	return true, "", nil
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return false
	}
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
