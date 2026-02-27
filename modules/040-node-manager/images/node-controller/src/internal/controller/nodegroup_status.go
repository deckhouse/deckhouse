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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/conditions"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func init() {
	ctrlmetrics.Registry.MustRegister(machineDeploymentNodeGroupInfo)
	Register("NodeGroupStatus", SetupNodeGroupStatus)
}

var machineDeploymentNodeGroupInfo = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "machine_deployment_node_group_info",
		Help: "Info about machine deployments by node group",
	},
	[]string{"node_group", "name"},
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

// SetupNodeGroupStatus registers the controller with the manager.
func SetupNodeGroupStatus(mgr ctrl.Manager) error {
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
	nodesForConditions := make([]*conditions.Node, 0, len(nodes))

	for _, node := range nodes {
		nodesCount++
		if isNodeReady(&node) {
			readyCount++
		}
		nodesForConditions = append(nodesForConditions, conditions.NodeToConditionsNode(&node))

		if configChecksum != "" {
			nodeChecksum := node.Annotations[ConfigurationChecksumAnnotation]
			if nodeChecksum == configChecksum {
				upToDateCount++
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
		// For Static/CloudPermanent, desired stays 0.
		// conditions.CalculateNodeGroupConditions handles this:
		// Static + desired=0: isReady = readySchedulableNodes == len(nodes)
	}

	// Build error list for conditions (matches original "|" separator)
	var conditionErrors []string
	if ng.Status.Error != "" {
		conditionErrors = append(conditionErrors, ng.Status.Error)
	}
	if errorMsg != "" {
		conditionErrors = append(conditionErrors, errorMsg)
	}

	// Build combined message for event
	eventMsg := fmt.Sprintf("%s %s", ng.Status.Error, errorMsg)
	eventMsg = strings.TrimSpace(eventMsg)
	if len(eventMsg) > 1024 {
		eventMsg = eventMsg[:1024]
	}

	// statusMsg is the rewritten message for conditionSummary and status.Error
	var statusMsg string
	if eventMsg != "" {
		r.createEventIfChanged(ng, eventMsg)
		statusMsg = "Machine creation failed. Check events for details."
	}

	// Use the original conditions package for calculation
	ngForConditions := conditions.NodeGroup{
		Type:                       ngv1.NodeType(ng.Spec.NodeType),
		Desired:                    desired,
		Instances:                  instancesCount,
		HasFrozenMachineDeployment: isFrozen,
	}

	// Convert existing metav1.Condition to ngv1.NodeGroupCondition for the conditions package
	existingNgConditions := convertToNgConditions(ng.Status.Conditions)

	ngConditions := conditions.CalculateNodeGroupConditions(
		ngForConditions,
		nodesForConditions,
		existingNgConditions,
		conditionErrors,
		int(minCount),
	)

	// Convert back to metav1.Condition for controller-runtime
	newConditions := convertFromNgConditions(ngConditions)

	conditionSummary := r.calculateConditionSummary(newConditions, statusMsg)

	patch := client.MergeFrom(ng.DeepCopy())
	ng.Status.Nodes = nodesCount
	ng.Status.Ready = readyCount
	ng.Status.UpToDate = upToDateCount
	ng.Status.Conditions = newConditions
	ng.Status.ConditionSummary = conditionSummary
	ng.Status.Error = statusMsg

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		ng.Status.Desired = desired
		ng.Status.Min = minCount
		ng.Status.Max = maxCount
		ng.Status.Instances = instancesCount
		machineFailures := convertMachineFailures(lastMachineFailures)
		if machineFailures == nil {
			machineFailures = []v1.MachineFailure{} // empty array, not nil — matches original
		}
		ng.Status.LastMachineFailures = machineFailures
	} else {
		// Clear cloud-specific fields for non-cloud NodeGroups (matches original nil patch)
		ng.Status.Desired = 0
		ng.Status.Min = 0
		ng.Status.Max = 0
		ng.Status.Instances = 0
		ng.Status.LastMachineFailures = nil
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
	ProviderID  string
	OwnerRef    string
	Message     string
	Time        time.Time
}

func convertMachineFailures(failures []MachineFailure) []v1.MachineFailure {
	if len(failures) == 0 {
		return nil
	}
	result := make([]v1.MachineFailure, 0, len(failures))
	for _, f := range failures {
		mf := v1.MachineFailure{
			Name:       f.MachineName,
			ProviderID: f.ProviderID,
			OwnerRef:   f.OwnerRef,
		}
		if f.Message != "" {
			mf.LastOperation = &v1.MachineLastOperation{
				Description:    f.Message,
				LastUpdateTime: f.Time.Format(time.RFC3339),
				State:          "Failed",
				Type:           "Create",
			}
		}
		result = append(result, mf)
	}
	return result
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
			mdName := md.GetName()
			machineDeploymentNodeGroupInfo.WithLabelValues(ngName, mdName).Set(1)

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
						mf := MachineFailure{Time: time.Now()}
						if name, _, _ := unstructured.NestedString(fmMap, "name"); name != "" {
							mf.MachineName = name
						}
						if providerID, _, _ := unstructured.NestedString(fmMap, "providerID"); providerID != "" {
							mf.ProviderID = providerID
						}
						if ownerRef, _, _ := unstructured.NestedString(fmMap, "ownerRef"); ownerRef != "" {
							mf.OwnerRef = ownerRef
						}
						if lastOp, _, _ := unstructured.NestedMap(fmMap, "lastOperation"); lastOp != nil {
							if msg, _, _ := unstructured.NestedString(lastOp, "description"); msg != "" {
								mf.Message = msg
							}
							if ts, _, _ := unstructured.NestedString(lastOp, "lastUpdateTime"); ts != "" {
								if t, err := time.Parse(time.RFC3339, ts); err == nil {
									mf.Time = t
								}
							}
						}
						if mf.Message != "" {
							failures = append(failures, mf)
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

// convertToNgConditions converts controller-runtime metav1.Condition to the
// ngv1.NodeGroupCondition format expected by the conditions package.
func convertToNgConditions(conds []metav1.Condition) []ngv1.NodeGroupCondition {
	result := make([]ngv1.NodeGroupCondition, 0, len(conds))
	for _, c := range conds {
		ngCond := ngv1.NodeGroupCondition{
			Type:               ngv1.NodeGroupConditionType(c.Type),
			LastTransitionTime: c.LastTransitionTime,
		}
		switch c.Status {
		case metav1.ConditionTrue:
			ngCond.Status = ngv1.ConditionTrue
		case metav1.ConditionFalse:
			ngCond.Status = ngv1.ConditionFalse
		default:
			ngCond.Status = ngv1.ConditionFalse
		}
		ngCond.Message = c.Message
		result = append(result, ngCond)
	}
	return result
}

// convertFromNgConditions converts ngv1.NodeGroupCondition back to metav1.Condition
// for the controller-runtime status patch.
func convertFromNgConditions(conds []ngv1.NodeGroupCondition) []metav1.Condition {
	result := make([]metav1.Condition, 0, len(conds))
	for _, c := range conds {
		cond := metav1.Condition{
			Type:               string(c.Type),
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		}
		switch c.Status {
		case ngv1.ConditionTrue:
			cond.Status = metav1.ConditionTrue
		case ngv1.ConditionFalse:
			cond.Status = metav1.ConditionFalse
		default:
			cond.Status = metav1.ConditionFalse
		}

		// Set Reason based on condition type and status (required by metav1.Condition)
		cond.Reason = reasonForCondition(string(c.Type), cond.Status)
		result = append(result, cond)
	}
	return result
}

// reasonForCondition returns a CamelCase reason string for the given condition.
// metav1.Condition requires Reason to be non-empty.
func reasonForCondition(condType string, status metav1.ConditionStatus) string {
	if status == metav1.ConditionTrue {
		switch condType {
		case ConditionTypeReady:
			return "AllNodesReady"
		case ConditionTypeUpdating:
			return "NodesUpdating"
		case ConditionTypeWaitingForDisruptiveApproval:
			return "WaitingForApproval"
		case ConditionTypeError:
			return "ErrorOccurred"
		case ConditionTypeScaling:
			return "Scaling"
		default:
			return "True"
		}
	}
	switch condType {
	case ConditionTypeReady:
		return "NotReady"
	case ConditionTypeUpdating:
		return "NoUpdatesInProgress"
	case ConditionTypeWaitingForDisruptiveApproval:
		return "NoDisruptiveUpdates"
	case ConditionTypeError:
		return "NoErrors"
	case ConditionTypeScaling:
		return "NotScaling"
	default:
		return "False"
	}
}

func (r *NodeGroupStatusReconciler) calculateConditionSummary(conditions []metav1.Condition, statusMsg string) *v1.ConditionSummary {
	ready := "True"
	if len(statusMsg) > 0 {
		ready = "False"
	}

	return &v1.ConditionSummary{
		Ready:         ready,
		StatusMessage: statusMsg,
	}
}
