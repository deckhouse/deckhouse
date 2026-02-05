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
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

const (
	// NodeGroupLabel is the label on Node that indicates which NodeGroup it belongs to
	NodeGroupLabel = "node.deckhouse.io/group"

	// ConfigurationChecksumAnnotation is the annotation on Node with configuration checksum
	ConfigurationChecksumAnnotation = "node.deckhouse.io/configuration-checksum"

	// MachineNamespace is the namespace where Machines and MachineDeployments are created
	MachineNamespace = "d8-cloud-instance-manager"

	// ConfigurationChecksumsSecretName is the name of the secret with configuration checksums
	ConfigurationChecksumsSecretName = "configuration-checksums"

	// CloudProviderSecretName is the name of the secret with cloud provider data (zones)
	CloudProviderSecretName = "d8-node-manager-cloud-provider"

	// DisruptiveApprovalAnnotation is the annotation on Node requesting disruptive update approval
	DisruptiveApprovalAnnotation = "update.node.deckhouse.io/disruption-required"

	// ApprovedAnnotation is the annotation on Node indicating disruptive update is approved
	ApprovedAnnotation = "update.node.deckhouse.io/approved"
)

var (
	MCMMachineGVK = schema.GroupVersionKind{
		Group:   "machine.sapcloud.io",
		Version: "v1alpha1",
		Kind:    "Machine",
	}
	MCMMachineDeploymentGVK = schema.GroupVersionKind{
		Group:   "machine.sapcloud.io",
		Version: "v1alpha1",
		Kind:    "MachineDeployment",
	}
)

var (
	CAPIMachineGVK = schema.GroupVersionKind{
		Group:   "cluster.x-k8s.io",
		Version: "v1beta1",
		Kind:    "Machine",
	}
	CAPIMachineDeploymentGVK = schema.GroupVersionKind{
		Group:   "cluster.x-k8s.io",
		Version: "v1beta1",
		Kind:    "MachineDeployment",
	}
)

// Condition types matching the original Python hook
const (
	ConditionTypeReady                        = "Ready"
	ConditionTypeUpdating                     = "Updating"
	ConditionTypeWaitingForDisruptiveApproval = "WaitingForDisruptiveApproval"
	ConditionTypeError                        = "Error"
	// Additional conditions for CloudEphemeral
	ConditionTypeScaling = "Scaling"
	ConditionTypeFrozen  = "Frozen"
)

// NodeGroupStatusReconciler updates NodeGroup.status based on actual cluster state.
//
// It watches NodeGroup, Node, Machine, MachineDeployment and recalculates status
// whenever any of these resources change.
//
// IMPORTANT: This controller only updates specific status fields and preserves
// fields managed by other controllers (e.g., deckhouse.processed, deckhouse.synced).
type NodeGroupStatusReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// SetupNodeGroupStatusController registers the NodeGroupStatus controller with the manager.
func SetupNodeGroupStatusController(mgr ctrl.Manager) error {
	return (&NodeGroupStatusReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeGroupStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.NodeGroup{}).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.nodeToNodeGroup),
		).
		Watches(
			r.newUnstructured(MCMMachineGVK),
			handler.EnqueueRequestsFromMapFunc(r.machineToNodeGroup),
		).
		Watches(
			r.newUnstructured(MCMMachineDeploymentGVK),
			handler.EnqueueRequestsFromMapFunc(r.machineDeploymentToNodeGroup),
		).
		Watches(
			r.newUnstructured(CAPIMachineGVK),
			handler.EnqueueRequestsFromMapFunc(r.machineToNodeGroup),
		).
		Watches(
			r.newUnstructured(CAPIMachineDeploymentGVK),
			handler.EnqueueRequestsFromMapFunc(r.machineDeploymentToNodeGroup),
		).
		Named("nodegroup-status").
		Complete(r)
}

// newUnstructured creates an unstructured object with the given GVK for watching.
// This is needed because Machine and MachineDeployment CRDs may not be installed.
func (r *NodeGroupStatusReconciler) newUnstructured(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	return u
}

// nodeToNodeGroup maps Node events to NodeGroup reconcile requests.
func (r *NodeGroupStatusReconciler) nodeToNodeGroup(ctx context.Context, obj client.Object) []reconcile.Request {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return nil
	}

	ngName, exists := node.Labels[NodeGroupLabel]
	if !exists {
		return nil
	}

	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: ngName}},
	}
}

// machineToNodeGroup maps Machine events to NodeGroup reconcile requests.
func (r *NodeGroupStatusReconciler) machineToNodeGroup(ctx context.Context, obj client.Object) []reconcile.Request {
	// MCM Machine has label "node.deckhouse.io/group"
	// CAPI Machine has label "node-group"
	labels := obj.GetLabels()

	ngName := labels[NodeGroupLabel]
	if ngName == "" {
		ngName = labels["node-group"]
	}
	if ngName == "" {
		return nil
	}

	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: ngName}},
	}
}

// machineDeploymentToNodeGroup maps MachineDeployment events to NodeGroup reconcile requests.
func (r *NodeGroupStatusReconciler) machineDeploymentToNodeGroup(ctx context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()

	ngName := labels["node-group"]
	if ngName == "" {
		return nil
	}

	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: ngName}},
	}
}

// Reconcile calculates and updates the status of a single NodeGroup.
func (r *NodeGroupStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("reconciling nodegroup status", "name", req.Name)

	// Get NodeGroup
	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Get nodes for this NodeGroup
	nodes, err := r.getNodesForNodeGroup(ctx, ng.Name)
	if err != nil {
		logger.Error(err, "failed to get nodes")
		return ctrl.Result{}, err
	}

	// Get configuration checksum for upToDate calculation
	configChecksum := r.getConfigurationChecksum(ctx, ng.Name)

	// Calculate basic counters
	var nodesCount, readyCount, upToDateCount int32
	var updatingNodes, waitingForApprovalNodes []string

	for _, node := range nodes {
		nodesCount++

		if isNodeReady(&node) {
			readyCount++
		}

		// Check upToDate
		if configChecksum != "" {
			nodeChecksum := node.Annotations[ConfigurationChecksumAnnotation]
			if nodeChecksum == configChecksum {
				upToDateCount++
			} else {
				// Node is not up to date - check if it's updating or waiting for approval
				if node.Annotations[DisruptiveApprovalAnnotation] != "" && node.Annotations[ApprovedAnnotation] == "" {
					waitingForApprovalNodes = append(waitingForApprovalNodes, node.Name)
				} else if nodeChecksum != configChecksum {
					updatingNodes = append(updatingNodes, node.Name)
				}
			}
		}
	}

	// Calculate desired, min, max for CloudEphemeral
	var desired, minCount, maxCount, instancesCount int32
	var lastMachineFailures []MachineFailure
	var isFrozen bool
	var errorMsg string

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		zonesCount := r.getZonesCount(ctx, ng)
		if ng.Spec.CloudInstances != nil {
			minCount = int32(ng.Spec.CloudInstances.MinPerZone) * zonesCount
			maxCount = int32(ng.Spec.CloudInstances.MaxPerZone) * zonesCount
		}

		desired, lastMachineFailures, isFrozen = r.getMachineDeploymentInfo(ctx, ng.Name)
		if minCount > desired {
			desired = minCount
		}
		instancesCount = r.getMachinesCount(ctx, ng.Name)

		// Build error message from machine failures
		if len(lastMachineFailures) > 0 {
			sort.Slice(lastMachineFailures, func(i, j int) bool {
				return lastMachineFailures[i].Time.Before(lastMachineFailures[j].Time)
			})
			errorMsg = lastMachineFailures[len(lastMachineFailures)-1].Message
		}
	} else {
		// For Static/CloudStatic/CloudPermanent, desired = actual nodes count
		desired = nodesCount
	}

	// Preserve existing error if present
	if ng.Status.Error != "" {
		if errorMsg != "" {
			errorMsg = ng.Status.Error + " " + errorMsg
		} else {
			errorMsg = ng.Status.Error
		}
	}
	if len(errorMsg) > 1024 {
		errorMsg = errorMsg[:1024]
	}

	// Calculate conditions
	conditions := r.calculateConditions(ng, nodes, readyCount, desired, instancesCount, isFrozen, errorMsg, updatingNodes, waitingForApprovalNodes)

	// Calculate conditionSummary
	conditionSummary := r.calculateConditionSummary(conditions)

	// Create patch - we use strategic merge patch to preserve fields we don't manage
	patch := client.MergeFrom(ng.DeepCopy())

	// Update only the fields this controller manages
	ng.Status.Nodes = nodesCount
	ng.Status.Ready = readyCount
	ng.Status.UpToDate = upToDateCount
	ng.Status.Conditions = conditions
	ng.Status.ConditionSummary = conditionSummary

	// Only set these for CloudEphemeral
	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		ng.Status.Desired = desired
		ng.Status.Min = minCount
		ng.Status.Max = maxCount
		ng.Status.Instances = instancesCount
	}

	// Apply patch
	if err := r.Client.Status().Patch(ctx, ng, patch); err != nil {
		logger.Error(err, "failed to patch nodegroup status")
		return ctrl.Result{}, err
	}

	logger.V(1).Info("updated nodegroup status",
		"name", ng.Name,
		"nodes", nodesCount,
		"ready", readyCount,
		"upToDate", upToDateCount,
	)

	return ctrl.Result{}, nil
}

// getNodesForNodeGroup returns all Nodes that belong to the specified NodeGroup.
func (r *NodeGroupStatusReconciler) getNodesForNodeGroup(ctx context.Context, ngName string) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.MatchingLabels{NodeGroupLabel: ngName}); err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

// isNodeReady checks if a Node has Ready condition = True.
func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// getConfigurationChecksum gets the configuration checksum for a NodeGroup from Secret.
func (r *NodeGroupStatusReconciler) getConfigurationChecksum(ctx context.Context, ngName string) string {
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: MachineNamespace,
		Name:      ConfigurationChecksumsSecretName,
	}, secret)
	if err != nil {
		return ""
	}

	return string(secret.Data[ngName])
}

// getZonesCount returns the number of zones for the NodeGroup.
func (r *NodeGroupStatusReconciler) getZonesCount(ctx context.Context, ng *v1.NodeGroup) int32 {
	// First, check if zones are specified in NodeGroup spec
	if ng.Spec.CloudInstances != nil && len(ng.Spec.CloudInstances.Zones) > 0 {
		return int32(len(ng.Spec.CloudInstances.Zones))
	}

	// Otherwise, get from cloud provider secret
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: "kube-system",
		Name:      CloudProviderSecretName,
	}, secret)
	if err != nil {
		return 1 // Default to 1 zone
	}

	// Parse zones from secret
	zonesData := secret.Data["zones"]
	if len(zonesData) == 0 {
		return 1
	}

	// Count commas + 1 (simple heuristic for JSON array)
	count := int32(1)
	for _, b := range zonesData {
		if b == ',' {
			count++
		}
	}
	return count
}

// MachineFailure represents a machine failure.
type MachineFailure struct {
	MachineName string
	Message     string
	Time        time.Time
}

// getMachineDeploymentInfo gets desired replicas and failure info from MachineDeployments.
func (r *NodeGroupStatusReconciler) getMachineDeploymentInfo(ctx context.Context, ngName string) (int32, []MachineFailure, bool) {
	var desired int32
	var failures []MachineFailure
	var isFrozen bool

	// MCM MachineDeployments
	mcmMDs := &unstructured.UnstructuredList{}
	mcmMDs.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   MCMMachineDeploymentGVK.Group,
		Version: MCMMachineDeploymentGVK.Version,
		Kind:    "MachineDeploymentList",
	})

	if err := r.Client.List(ctx, mcmMDs,
		client.InNamespace(MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err == nil {
		for _, md := range mcmMDs.Items {
			replicas, found, _ := unstructured.NestedInt64(md.Object, "spec", "replicas")
			if found {
				desired += int32(replicas)
			}

			// Check for Frozen condition
			conditions, found, _ := unstructured.NestedSlice(md.Object, "status", "conditions")
			if found {
				for _, c := range conditions {
					cond, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					if cond["type"] == "Frozen" && cond["status"] == "True" {
						isFrozen = true
					}
				}
			}

			// Get failed machines
			failedMachines, found, _ := unstructured.NestedSlice(md.Object, "status", "failedMachines")
			if found {
				for _, fm := range failedMachines {
					fmMap, ok := fm.(map[string]interface{})
					if !ok {
						continue
					}
					lastOp, _, _ := unstructured.NestedMap(fmMap, "lastOperation")
					if lastOp != nil {
						msg, _, _ := unstructured.NestedString(lastOp, "description")
						failures = append(failures, MachineFailure{
							Message: msg,
							Time:    time.Now(),
						})
					}
				}
			}
		}
	}

	// CAPI MachineDeployments
	capiMDs := &unstructured.UnstructuredList{}
	capiMDs.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   CAPIMachineDeploymentGVK.Group,
		Version: CAPIMachineDeploymentGVK.Version,
		Kind:    "MachineDeploymentList",
	})

	if err := r.Client.List(ctx, capiMDs,
		client.InNamespace(MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err == nil {
		for _, md := range capiMDs.Items {
			replicas, found, _ := unstructured.NestedInt64(md.Object, "spec", "replicas")
			if found {
				desired += int32(replicas)
			}
		}
	}

	return desired, failures, isFrozen
}

// getMachinesCount returns the count of Machines for a NodeGroup.
func (r *NodeGroupStatusReconciler) getMachinesCount(ctx context.Context, ngName string) int32 {
	var count int32

	// MCM Machines
	mcmMachines := &unstructured.UnstructuredList{}
	mcmMachines.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   MCMMachineGVK.Group,
		Version: MCMMachineGVK.Version,
		Kind:    "MachineList",
	})

	if err := r.Client.List(ctx, mcmMachines,
		client.InNamespace(MachineNamespace),
	); err == nil {
		for _, m := range mcmMachines.Items {
			labels, found, _ := unstructured.NestedStringMap(m.Object, "spec", "nodeTemplate", "metadata", "labels")
			if found && labels[NodeGroupLabel] == ngName {
				count++
			}
		}
	}

	// CAPI Machines
	capiMachines := &unstructured.UnstructuredList{}
	capiMachines.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   CAPIMachineGVK.Group,
		Version: CAPIMachineGVK.Version,
		Kind:    "MachineList",
	})

	if err := r.Client.List(ctx, capiMachines,
		client.InNamespace(MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err == nil {
		count += int32(len(capiMachines.Items))
	}

	return count
}

// calculateConditions calculates all conditions for the NodeGroup.
// This matches the conditions generated by the original Python hook.
func (r *NodeGroupStatusReconciler) calculateConditions(
	ng *v1.NodeGroup,
	nodes []corev1.Node,
	readyCount, desired, instances int32,
	isFrozen bool,
	errorMsg string,
	updatingNodes, waitingForApprovalNodes []string,
) []metav1.Condition {
	now := metav1.Now()
	conditions := make([]metav1.Condition, 0, 4)

	nodesCount := int32(len(nodes))

	// 1. Ready condition
	readyCondition := metav1.Condition{
		Type:               ConditionTypeReady,
		LastTransitionTime: now,
	}

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		// For CloudEphemeral, Ready when readyCount == desired
		if readyCount >= desired && desired > 0 {
			readyCondition.Status = metav1.ConditionTrue
			readyCondition.Reason = "AllNodesReady"
			readyCondition.Message = fmt.Sprintf("All %d nodes are ready", readyCount)
		} else if desired == 0 {
			readyCondition.Status = metav1.ConditionFalse
			readyCondition.Reason = "NoNodesDesired"
			readyCondition.Message = "No nodes desired"
		} else {
			readyCondition.Status = metav1.ConditionFalse
			readyCondition.Reason = "NotAllNodesReady"
			readyCondition.Message = fmt.Sprintf("%d of %d nodes are ready", readyCount, desired)
		}
	} else {
		// For Static/CloudStatic/CloudPermanent, Ready when all existing nodes are ready
		if nodesCount > 0 && readyCount == nodesCount {
			readyCondition.Status = metav1.ConditionTrue
			readyCondition.Reason = "AllNodesReady"
			readyCondition.Message = fmt.Sprintf("All %d nodes are ready", readyCount)
		} else if nodesCount == 0 {
			readyCondition.Status = metav1.ConditionFalse
			readyCondition.Reason = "NoNodes"
			readyCondition.Message = "No nodes in the group"
		} else {
			readyCondition.Status = metav1.ConditionFalse
			readyCondition.Reason = "NotAllNodesReady"
			readyCondition.Message = fmt.Sprintf("%d of %d nodes are ready", readyCount, nodesCount)
		}
	}
	conditions = append(conditions, readyCondition)

	// 2. Updating condition
	updatingCondition := metav1.Condition{
		Type:               ConditionTypeUpdating,
		LastTransitionTime: now,
	}

	if len(updatingNodes) > 0 {
		updatingCondition.Status = metav1.ConditionTrue
		updatingCondition.Reason = "NodesUpdating"
		updatingCondition.Message = fmt.Sprintf("Nodes updating: %s", strings.Join(updatingNodes, ", "))
	} else {
		updatingCondition.Status = metav1.ConditionFalse
		updatingCondition.Reason = "NoUpdatesInProgress"
		updatingCondition.Message = ""
	}
	conditions = append(conditions, updatingCondition)

	// 3. WaitingForDisruptiveApproval condition
	waitingCondition := metav1.Condition{
		Type:               ConditionTypeWaitingForDisruptiveApproval,
		LastTransitionTime: now,
	}

	if len(waitingForApprovalNodes) > 0 {
		waitingCondition.Status = metav1.ConditionTrue
		waitingCondition.Reason = "WaitingForApproval"
		waitingCondition.Message = fmt.Sprintf("Nodes waiting for approval: %s", strings.Join(waitingForApprovalNodes, ", "))
	} else {
		waitingCondition.Status = metav1.ConditionFalse
		waitingCondition.Reason = "NoDisruptiveUpdates"
		waitingCondition.Message = ""
	}
	conditions = append(conditions, waitingCondition)

	// 4. Error condition
	errorCondition := metav1.Condition{
		Type:               ConditionTypeError,
		LastTransitionTime: now,
	}

	if errorMsg != "" {
		errorCondition.Status = metav1.ConditionTrue
		errorCondition.Reason = "ErrorOccurred"
		errorCondition.Message = strings.TrimSpace(errorMsg)
	} else {
		errorCondition.Status = metav1.ConditionFalse
		errorCondition.Reason = "NoErrors"
		errorCondition.Message = ""
	}
	conditions = append(conditions, errorCondition)

	// 5. Scaling condition (only for CloudEphemeral)
	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		scalingCondition := metav1.Condition{
			Type:               ConditionTypeScaling,
			LastTransitionTime: now,
		}

		if instances < desired {
			scalingCondition.Status = metav1.ConditionTrue
			scalingCondition.Reason = "ScalingUp"
			scalingCondition.Message = fmt.Sprintf("Scaling up: %d instances, %d desired", instances, desired)
		} else if instances > desired {
			scalingCondition.Status = metav1.ConditionTrue
			scalingCondition.Reason = "ScalingDown"
			scalingCondition.Message = fmt.Sprintf("Scaling down: %d instances, %d desired", instances, desired)
		} else {
			scalingCondition.Status = metav1.ConditionFalse
			scalingCondition.Reason = "NotScaling"
			scalingCondition.Message = "Desired number of instances reached"
		}
		conditions = append(conditions, scalingCondition)
	}

	// 6. Frozen condition (only for CloudEphemeral)
	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral && isFrozen {
		frozenCondition := metav1.Condition{
			Type:               ConditionTypeFrozen,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: now,
			Reason:             "MachineDeploymentFrozen",
			Message:            "MachineDeployment is frozen due to errors",
		}
		conditions = append(conditions, frozenCondition)
	}

	return conditions
}

// ConditionSummary represents the summary of conditions.
type ConditionSummary struct {
	Ready         string `json:"ready"`
	StatusMessage string `json:"statusMessage,omitempty"`
}

// calculateConditionSummary calculates the conditionSummary based on conditions.
func (r *NodeGroupStatusReconciler) calculateConditionSummary(conditions []metav1.Condition) *v1.ConditionSummary {
	summary := &v1.ConditionSummary{
		Ready: "False",
	}

	var messages []string

	for _, cond := range conditions {
		switch cond.Type {
		case ConditionTypeReady:
			if cond.Status == metav1.ConditionTrue {
				summary.Ready = "True"
			}
		case ConditionTypeError:
			if cond.Status == metav1.ConditionTrue && cond.Message != "" {
				messages = append(messages, cond.Message)
			}
		case ConditionTypeUpdating:
			if cond.Status == metav1.ConditionTrue && cond.Message != "" {
				messages = append(messages, cond.Message)
			}
		case ConditionTypeWaitingForDisruptiveApproval:
			if cond.Status == metav1.ConditionTrue && cond.Message != "" {
				messages = append(messages, cond.Message)
			}
		}
	}

	if len(messages) > 0 {
		summary.StatusMessage = strings.Join(messages, "; ")
	}

	return summary
}
