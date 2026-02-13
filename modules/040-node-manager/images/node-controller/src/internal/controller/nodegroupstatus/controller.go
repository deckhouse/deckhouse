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

package nodegroupstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller"
)

func init() {
	controller.Register("NodeGroupStatus", Setup)
}

const (
	NodeGroupLabel                   = "node.deckhouse.io/group"
	ConfigurationChecksumAnnotation  = "node.deckhouse.io/configuration-checksum"
	MachineNamespace                 = "d8-cloud-instance-manager"
	ConfigurationChecksumsSecretName = "configuration-checksums"
	CloudProviderSecretName          = "d8-node-manager-cloud-provider"
	DisruptiveApprovalAnnotation     = "update.node.deckhouse.io/disruption-required"
	ApprovedAnnotation               = "update.node.deckhouse.io/approved"
)

var (
	MCMMachineGVK = schema.GroupVersionKind{
		Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "Machine",
	}
	MCMMachineDeploymentGVK = schema.GroupVersionKind{
		Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineDeployment",
	}
	CAPIMachineGVK = schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta1", Kind: "Machine",
	}
	CAPIMachineDeploymentGVK = schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta1", Kind: "MachineDeployment",
	}
)

const (
	ConditionTypeReady                        = "Ready"
	ConditionTypeUpdating                     = "Updating"
	ConditionTypeWaitingForDisruptiveApproval = "WaitingForDisruptiveApproval"
	ConditionTypeError                        = "Error"
	ConditionTypeScaling                      = "Scaling"
	ConditionTypeFrozen                       = "Frozen"
)

// NodeGroupStatusReconciler updates NodeGroup.status based on actual cluster state.
type NodeGroupStatusReconciler struct {
	Client            client.Client
	Scheme            *runtime.Scheme
	Recorder          record.EventRecorder
	lastEventMessages sync.Map
}

// Setup registers the controller with the manager.
func Setup(mgr ctrl.Manager) error {
	return (&NodeGroupStatusReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("node-controller"),
	}).SetupWithManager(mgr)
}

func (r *NodeGroupStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	nodeHasGroupLabel := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, exists := obj.GetLabels()[NodeGroupLabel]
		return exists
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.NodeGroup{}).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.nodeToNodeGroup), builder.WithPredicates(nodeHasGroupLabel)).
		Watches(newUnstructured(MCMMachineGVK), handler.EnqueueRequestsFromMapFunc(r.machineToNodeGroup)).
		Watches(newUnstructured(MCMMachineDeploymentGVK), handler.EnqueueRequestsFromMapFunc(r.machineDeploymentToNodeGroup)).
		Watches(newUnstructured(CAPIMachineGVK), handler.EnqueueRequestsFromMapFunc(r.machineToNodeGroup)).
		Watches(newUnstructured(CAPIMachineDeploymentGVK), handler.EnqueueRequestsFromMapFunc(r.machineDeploymentToNodeGroup)).
		Named("nodegroup-status").
		Complete(r)
}

func newUnstructured(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	return u
}

func (r *NodeGroupStatusReconciler) nodeToNodeGroup(ctx context.Context, obj client.Object) []reconcile.Request {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return nil
	}
	ngName, exists := node.Labels[NodeGroupLabel]
	if !exists {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ngName}}}
}

func (r *NodeGroupStatusReconciler) machineToNodeGroup(ctx context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	ngName := labels[NodeGroupLabel]
	if ngName == "" {
		ngName = labels["node-group"]
	}
	if ngName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ngName}}}
}

func (r *NodeGroupStatusReconciler) machineDeploymentToNodeGroup(ctx context.Context, obj client.Object) []reconcile.Request {
	ngName := obj.GetLabels()["node-group"]
	if ngName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ngName}}}
}

func (r *NodeGroupStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("reconciling nodegroup status", "name", req.Name)

	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	nodes, err := r.getNodesForNodeGroup(ctx, ng.Name)
	if err != nil {
		logger.Error(err, "failed to get nodes")
		return ctrl.Result{}, err
	}

	configChecksum := r.getConfigurationChecksum(ctx, ng.Name)

	var nodesCount, readyCount, upToDateCount int32
	var updatingNodes, waitingForApprovalNodes []string

	for _, node := range nodes {
		nodesCount++
		if isNodeReady(&node) {
			readyCount++
		}
		if configChecksum != "" {
			nodeChecksum := node.Annotations[ConfigurationChecksumAnnotation]
			if nodeChecksum == configChecksum {
				upToDateCount++
			} else {
				if node.Annotations[DisruptiveApprovalAnnotation] != "" && node.Annotations[ApprovedAnnotation] == "" {
					waitingForApprovalNodes = append(waitingForApprovalNodes, node.Name)
				} else {
					updatingNodes = append(updatingNodes, node.Name)
				}
			}
		}
	}

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
		if len(lastMachineFailures) > 0 {
			sort.Slice(lastMachineFailures, func(i, j int) bool {
				return lastMachineFailures[i].Time.Before(lastMachineFailures[j].Time)
			})
			errorMsg = lastMachineFailures[len(lastMachineFailures)-1].Message
		}
	} else {
		desired = nodesCount
	}

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

	if errorMsg != "" {
		r.createEventIfChanged(ng, errorMsg)
		errorMsg = "Machine creation failed. Check events for details."
	}

	conditions := r.calculateConditions(ng, nodes, readyCount, desired, instancesCount, isFrozen, errorMsg, updatingNodes, waitingForApprovalNodes)
	conditionSummary := r.calculateConditionSummary(conditions)

	patch := client.MergeFrom(ng.DeepCopy())
	ng.Status.Nodes = nodesCount
	ng.Status.Ready = readyCount
	ng.Status.UpToDate = upToDateCount
	ng.Status.Conditions = conditions
	ng.Status.ConditionSummary = conditionSummary

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		ng.Status.Desired = desired
		ng.Status.Min = minCount
		ng.Status.Max = maxCount
		ng.Status.Instances = instancesCount
	}

	if err := r.Client.Status().Patch(ctx, ng, patch); err != nil {
		logger.Error(err, "failed to patch nodegroup status")
		return ctrl.Result{}, err
	}

	logger.V(1).Info("updated nodegroup status", "name", ng.Name, "nodes", nodesCount, "ready", readyCount, "upToDate", upToDateCount)
	return ctrl.Result{}, nil
}

func (r *NodeGroupStatusReconciler) getNodesForNodeGroup(ctx context.Context, ngName string) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.MatchingLabels{NodeGroupLabel: ngName}); err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (r *NodeGroupStatusReconciler) getConfigurationChecksum(ctx context.Context, ngName string) string {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: MachineNamespace, Name: ConfigurationChecksumsSecretName}, secret); err != nil {
		return ""
	}
	return string(secret.Data[ngName])
}

func (r *NodeGroupStatusReconciler) getZonesCount(ctx context.Context, ng *v1.NodeGroup) int32 {
	if ng.Spec.CloudInstances != nil && len(ng.Spec.CloudInstances.Zones) > 0 {
		return int32(len(ng.Spec.CloudInstances.Zones))
	}
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: "kube-system", Name: CloudProviderSecretName}, secret); err != nil {
		return 1
	}
	var zones []string
	if err := json.Unmarshal(secret.Data["zones"], &zones); err != nil || len(zones) == 0 {
		return 1
	}
	return int32(len(zones))
}

type MachineFailure struct {
	MachineName string
	Message     string
	Time        time.Time
}

func (r *NodeGroupStatusReconciler) getMachineDeploymentInfo(ctx context.Context, ngName string) (int32, []MachineFailure, bool) {
	var desired int32
	var failures []MachineFailure
	var isFrozen bool

	for _, gvk := range []schema.GroupVersionKind{MCMMachineDeploymentGVK, CAPIMachineDeploymentGVK} {
		mdList := &unstructured.UnstructuredList{}
		mdList.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
		if err := r.Client.List(ctx, mdList, client.InNamespace(MachineNamespace), client.MatchingLabels{"node-group": ngName}); err != nil {
			continue
		}
		for _, md := range mdList.Items {
			if replicas, found, _ := unstructured.NestedInt64(md.Object, "spec", "replicas"); found {
				desired += int32(replicas)
			}
			if conditions, found, _ := unstructured.NestedSlice(md.Object, "status", "conditions"); found {
				for _, c := range conditions {
					if cond, ok := c.(map[string]interface{}); ok && cond["type"] == "Frozen" && cond["status"] == "True" {
						isFrozen = true
					}
				}
			}
			if failedMachines, found, _ := unstructured.NestedSlice(md.Object, "status", "failedMachines"); found {
				for _, fm := range failedMachines {
					if fmMap, ok := fm.(map[string]interface{}); ok {
						if lastOp, _, _ := unstructured.NestedMap(fmMap, "lastOperation"); lastOp != nil {
							if msg, _, _ := unstructured.NestedString(lastOp, "description"); msg != "" {
								failures = append(failures, MachineFailure{Message: msg, Time: time.Now()})
							}
						}
					}
				}
			}
		}
	}
	return desired, failures, isFrozen
}

func (r *NodeGroupStatusReconciler) getMachinesCount(ctx context.Context, ngName string) int32 {
	var count int32

	// MCM Machines
	mcmList := &unstructured.UnstructuredList{}
	mcmList.SetGroupVersionKind(schema.GroupVersionKind{Group: MCMMachineGVK.Group, Version: MCMMachineGVK.Version, Kind: "MachineList"})
	if err := r.Client.List(ctx, mcmList, client.InNamespace(MachineNamespace)); err == nil {
		for _, m := range mcmList.Items {
			if labels, found, _ := unstructured.NestedStringMap(m.Object, "spec", "nodeTemplate", "metadata", "labels"); found && labels[NodeGroupLabel] == ngName {
				count++
			}
		}
	}

	// CAPI Machines
	capiList := &unstructured.UnstructuredList{}
	capiList.SetGroupVersionKind(schema.GroupVersionKind{Group: CAPIMachineGVK.Group, Version: CAPIMachineGVK.Version, Kind: "MachineList"})
	if err := r.Client.List(ctx, capiList, client.InNamespace(MachineNamespace), client.MatchingLabels{"node-group": ngName}); err == nil {
		count += int32(len(capiList.Items))
	}
	return count
}

func (r *NodeGroupStatusReconciler) createEventIfChanged(ng *v1.NodeGroup, msg string) {
	if prev, _ := r.lastEventMessages.Load(ng.Name); prev == msg {
		return
	}
	eventType, reason := corev1.EventTypeWarning, "MachineFailed"
	if msg == "Started Machine creation process" {
		eventType, reason = corev1.EventTypeNormal, "MachineCreating"
	}
	r.Recorder.Event(ng, eventType, reason, msg)
	r.lastEventMessages.Store(ng.Name, msg)
}

func (r *NodeGroupStatusReconciler) calculateConditions(ng *v1.NodeGroup, nodes []corev1.Node, readyCount, desired, instances int32, isFrozen bool, errorMsg string, updatingNodes, waitingForApprovalNodes []string) []metav1.Condition {
	now := metav1.Now()
	existing := ng.Status.Conditions
	conditions := make([]metav1.Condition, 0, 6)
	nodesCount := int32(len(nodes))

	// Ready
	readyCond := metav1.Condition{Type: ConditionTypeReady}
	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		if readyCount >= desired && desired > 0 {
			readyCond.Status, readyCond.Reason, readyCond.Message = metav1.ConditionTrue, "AllNodesReady", fmt.Sprintf("All %d nodes are ready", readyCount)
		} else if desired == 0 {
			readyCond.Status, readyCond.Reason, readyCond.Message = metav1.ConditionFalse, "NoNodesDesired", "No nodes desired"
		} else {
			readyCond.Status, readyCond.Reason, readyCond.Message = metav1.ConditionFalse, "NotAllNodesReady", fmt.Sprintf("%d of %d nodes are ready", readyCount, desired)
		}
	} else {
		if nodesCount > 0 && readyCount == nodesCount {
			readyCond.Status, readyCond.Reason, readyCond.Message = metav1.ConditionTrue, "AllNodesReady", fmt.Sprintf("All %d nodes are ready", readyCount)
		} else if nodesCount == 0 {
			readyCond.Status, readyCond.Reason, readyCond.Message = metav1.ConditionFalse, "NoNodes", "No nodes in the group"
		} else {
			readyCond.Status, readyCond.Reason, readyCond.Message = metav1.ConditionFalse, "NotAllNodesReady", fmt.Sprintf("%d of %d nodes are ready", readyCount, nodesCount)
		}
	}
	setConditionTime(&readyCond, existing, now)
	conditions = append(conditions, readyCond)

	// Updating
	updatingCond := metav1.Condition{Type: ConditionTypeUpdating, Status: metav1.ConditionFalse, Reason: "NoUpdatesInProgress"}
	if len(updatingNodes) > 0 {
		updatingCond.Status, updatingCond.Reason, updatingCond.Message = metav1.ConditionTrue, "NodesUpdating", fmt.Sprintf("Nodes updating: %s", strings.Join(updatingNodes, ", "))
	}
	setConditionTime(&updatingCond, existing, now)
	conditions = append(conditions, updatingCond)

	// WaitingForDisruptiveApproval
	waitingCond := metav1.Condition{Type: ConditionTypeWaitingForDisruptiveApproval, Status: metav1.ConditionFalse, Reason: "NoDisruptiveUpdates"}
	if len(waitingForApprovalNodes) > 0 {
		waitingCond.Status, waitingCond.Reason, waitingCond.Message = metav1.ConditionTrue, "WaitingForApproval", fmt.Sprintf("Nodes waiting for approval: %s", strings.Join(waitingForApprovalNodes, ", "))
	}
	setConditionTime(&waitingCond, existing, now)
	conditions = append(conditions, waitingCond)

	// Error
	errorCond := metav1.Condition{Type: ConditionTypeError, Status: metav1.ConditionFalse, Reason: "NoErrors"}
	if errorMsg != "" {
		errorCond.Status, errorCond.Reason, errorCond.Message = metav1.ConditionTrue, "ErrorOccurred", strings.TrimSpace(errorMsg)
	}
	setConditionTime(&errorCond, existing, now)
	conditions = append(conditions, errorCond)

	// Scaling (CloudEphemeral only)
	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		scalingCond := metav1.Condition{Type: ConditionTypeScaling, Status: metav1.ConditionFalse, Reason: "NotScaling", Message: "Desired number of instances reached"}
		if instances < desired {
			scalingCond.Status, scalingCond.Reason, scalingCond.Message = metav1.ConditionTrue, "ScalingUp", fmt.Sprintf("Scaling up: %d instances, %d desired", instances, desired)
		} else if instances > desired {
			scalingCond.Status, scalingCond.Reason, scalingCond.Message = metav1.ConditionTrue, "ScalingDown", fmt.Sprintf("Scaling down: %d instances, %d desired", instances, desired)
		}
		setConditionTime(&scalingCond, existing, now)
		conditions = append(conditions, scalingCond)

		if isFrozen {
			frozenCond := metav1.Condition{Type: ConditionTypeFrozen, Status: metav1.ConditionTrue, Reason: "MachineDeploymentFrozen", Message: "MachineDeployment is frozen due to errors"}
			setConditionTime(&frozenCond, existing, now)
			conditions = append(conditions, frozenCond)
		}
	}
	return conditions
}

func setConditionTime(cond *metav1.Condition, existing []metav1.Condition, now metav1.Time) {
	for i := range existing {
		if existing[i].Type == cond.Type && existing[i].Status == cond.Status {
			cond.LastTransitionTime = existing[i].LastTransitionTime
			return
		}
	}
	cond.LastTransitionTime = now
}

func (r *NodeGroupStatusReconciler) calculateConditionSummary(conditions []metav1.Condition) *v1.ConditionSummary {
	summary := &v1.ConditionSummary{Ready: "False"}
	var messages []string
	for _, cond := range conditions {
		if cond.Type == ConditionTypeReady && cond.Status == metav1.ConditionTrue {
			summary.Ready = "True"
		}
		if cond.Status == metav1.ConditionTrue && cond.Message != "" && (cond.Type == ConditionTypeError || cond.Type == ConditionTypeUpdating || cond.Type == ConditionTypeWaitingForDisruptiveApproval) {
			messages = append(messages, cond.Message)
		}
	}
	if len(messages) > 0 {
		summary.StatusMessage = strings.Join(messages, "; ")
	}
	return summary
}
