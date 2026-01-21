/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	capi "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/capi/v1beta1"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/clusterapi"
	mcmv1alpha1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/mcm/v1alpha1"
	d8v1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
	d8v1alpha1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 1 * time.Second,
		ExecutionBurst:       2,
	},
	AllowFailure: true,
	Queue:        "/modules/node-manager/instance_controller",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "instances",
			Kind:       "Instance",
			ApiVersion: "deckhouse.io/v1alpha1",
			FilterFunc: instanceFilter,
		},
		{
			Name:       "ngs",
			Kind:       "NodeGroup",
			ApiVersion: "deckhouse.io/v1",
			FilterFunc: instanceNodeGroupFilter,
		},
		{
			Name:       "machines",
			ApiVersion: "machine.sapcloud.io/v1alpha1",
			Kind:       "Machine",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: instanceMachineFilter,
		},
		{
			Name:       "cluster_api_machines",
			ApiVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Machine",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: instanceClusterAPIMachineFilter,
		},
	},
}, instanceController)

func instanceMachineFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var machine mcmv1alpha1.Machine

	err := sdk.FromUnstructured(obj, &machine)
	if err != nil {
		return nil, err
	}

	return &machineForInstance{
		APIVersion:        machine.APIVersion,
		Kind:              machine.Kind,
		NodeGroup:         machine.Spec.NodeTemplateSpec.Labels["node.deckhouse.io/group"],
		NodeName:          machine.GetLabels()["node"],
		Name:              machine.GetName(),
		CurrentStatus:     machine.Status.CurrentStatus,
		LastOperation:     &machine.Status.LastOperation,
		DeletionTimestamp: machine.GetDeletionTimestamp(),
	}, nil
}

func instanceClusterAPIMachineFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var machine clusterapi.Machine

	err := sdk.FromUnstructured(obj, &machine)
	if err != nil {
		return nil, err
	}

	var nodeName string

	if machine.Status.NodeRef != nil {
		nodeName = machine.Status.NodeRef.Name
	}

	var lastUpdated metav1.Time

	if machine.Status.LastUpdated != nil {
		lastUpdated = *machine.Status.LastUpdated
	}

	var nodeDrainStartTime *metav1.Time
	if machine.Status.Deletion != nil {
		nodeDrainStartTime = machine.Status.Deletion.NodeDrainStartTime
	}

	conditions := capiMachineConditions(&machine)
	errorInfo := capiMachineErrorInfo(&machine)
	var lastOperation *mcmv1alpha1.LastOperation
	if errorInfo != nil {
		lastOperation = &mcmv1alpha1.LastOperation{
			Description:    errorInfo.Description,
			LastUpdateTime: errorInfo.LastUpdateTime,
			State:          mcmv1alpha1.MachineStateFailed,
			Type:           mcmv1alpha1.MachineOperationHealthCheck,
		}
	}

	return &machineForInstance{
		APIVersion: machine.APIVersion,
		Kind:       machine.Kind,
		NodeGroup:  machine.GetLabels()["node-group"],
		NodeName:   nodeName,
		Name:       machine.GetName(),
		CurrentStatus: mcmv1alpha1.CurrentStatus{
			Phase:          mcmv1alpha1.MachinePhase(machine.Status.Phase),
			LastUpdateTime: lastUpdated,
		},
		IsCAPI:             true,
		Conditions:         conditions,
		NodeDrainStartTime: nodeDrainStartTime,
		LastOperation:      lastOperation,
		DeletionTimestamp:  machine.GetDeletionTimestamp(),
		ErrorInfo:          errorInfo,
	}, nil
}

var (
	deleteFinalizersPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": nil,
		},
	}
)

func newDrainingAnnotationPatch() map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"update.node.deckhouse.io/draining": "instance-deletion",
			},
		},
	}
}

var capiMachineConditionPriority = map[capi.ConditionType]int{
	capi.InfrastructureReadyCondition:             0,
	capi.BootstrapReadyCondition:                  1,
	capi.ConditionType("Deleting"):                1,
	capi.ReadyCondition:                           2,
	capi.MachineNodeHealthyCondition:              3,
	capi.DrainingSucceededCondition:               4,
	capi.VolumeDetachSucceededCondition:           5,
	capi.PreDrainDeleteHookSucceededCondition:     6,
	capi.PreTerminateDeleteHookSucceededCondition: 7,
}

const instanceMessageMaxRunes = 200

func instanceNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng d8v1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	return &nodeGroupForInstance{
		Name:           ng.GetName(),
		UID:            ng.GetUID(),
		ClassReference: ng.Spec.CloudInstances.ClassReference,
	}, nil
}

func instanceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ic d8v1alpha1.Instance

	err := sdk.FromUnstructured(obj, &ic)
	if err != nil {
		return nil, err
	}

	return &instance{
		Name:              ic.Name,
		DeletionTimestamp: ic.DeletionTimestamp,
		Status:            ic.Status,
	}, nil
}

func instanceController(_ context.Context, input *go_hook.HookInput) error {
	instances := make(map[string]*instance, len(input.Snapshots.Get("instances")))
	machines := make(map[string]*machineForInstance, len(input.Snapshots.Get("machines")))
	clusterAPIMachines := make(map[string]*machineForInstance, len(input.Snapshots.Get("cluster_api_machines")))
	nodeGroups := make(map[string]*nodeGroupForInstance, len(input.Snapshots.Get("ngs")))
	for ins, err := range sdkobjectpatch.SnapshotIter[instance](input.Snapshots.Get("instances")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'instances' snapshots: %w", err)
		}

		instances[ins.Name] = &ins
	}

	for mc, err := range sdkobjectpatch.SnapshotIter[machineForInstance](input.Snapshots.Get("machines")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'machines' snapshots: %w", err)
		}

		machines[mc.Name] = &mc
	}

	for mc, err := range sdkobjectpatch.SnapshotIter[machineForInstance](input.Snapshots.Get("cluster_api_machines")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'cluster_api_machines' snapshots: %w", err)
		}

		clusterAPIMachines[mc.Name] = &mc
	}

	for ng, err := range sdkobjectpatch.SnapshotIter[nodeGroupForInstance](input.Snapshots.Get("ngs")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'ngs' snapshots: %w", err)
		}

		nodeGroups[ng.Name] = &ng
	}

	// first, check mapping from machines to instance
	// here we handle two cases:
	//   1. if instance exists, then we have 2 subcases:
	//  	a. first, we should compare machines fields and instance status and if we have changes path status
	//		b. next, we should check metadata.deletionTimestamp for instance
	//         if we have non-zero timestamp this means that client deletes instance, and we should delete machine.
	//         Before deletion, we should check metadata.deletionTimestamp in the machine for prevent multiple deletion
	//   2. if instance does not exist, create instance for machine
	for name, machine := range machines {
		ng, ok := nodeGroups[machine.NodeGroup]
		if !ok {
			input.Logger.Warn("NodeGroup not found", slog.String("name", machine.NodeGroup))

			continue
		}

		if ic, ok := instances[name]; ok {
			desiredPhase, _ := resolveInstancePhase(ic, machine)
			if desiredPhase == d8v1alpha1.InstanceDraining && ic.Status.CurrentStatus.Phase != d8v1alpha1.InstanceDraining {
				nodeName := machine.NodeName
				if nodeName == "" {
					nodeName = ic.Status.NodeRef.Name
				}
				if nodeName != "" {
					input.Logger.Info("Setting draining annotation on node due to Instance draining",
						slog.String("instance", ic.Name),
						slog.String("node", nodeName))
					input.PatchCollector.PatchWithMerge(newDrainingAnnotationPatch(), "v1", "Node", "", nodeName)
				} else {
					input.Logger.Warn("Cannot set draining annotation: node name is empty",
						slog.String("instance", ic.Name),
						slog.String("machine", machine.Name))
				}
			}

			statusPatch := getInstanceStatusPatch(ic, machine, ng)
			if len(statusPatch) > 0 {
				patch := map[string]interface{}{
					"status": statusPatch,
				}
				input.PatchCollector.PatchWithMerge(patch, "deckhouse.io/v1alpha1", "Instance", "", ic.Name, object_patch.WithSubresource("/status"))
			}

			if ic.DeletionTimestamp != nil && !ic.DeletionTimestamp.IsZero() {
				if machine.DeletionTimestamp == nil || machine.DeletionTimestamp.IsZero() {
					// delete in background, because machine has finalizer
					input.PatchCollector.DeleteInBackground("machine.sapcloud.io/v1alpha1", "Machine", "d8-cloud-instance-manager", machine.Name)
				}
			}
		} else {
			newIc := newInstance(machine, ng)
			input.PatchCollector.CreateIfNotExists(newIc)
		}
	}

	for name, machine := range clusterAPIMachines {
		ng, ok := nodeGroups[machine.NodeGroup]
		if !ok {
			input.Logger.Warn("NodeGroup not found", slog.String("name", machine.NodeGroup))

			continue
		}

		if ic, ok := instances[name]; ok {
			desiredPhase, _ := resolveInstancePhase(ic, machine)
			if desiredPhase == d8v1alpha1.InstanceDraining && ic.Status.CurrentStatus.Phase != d8v1alpha1.InstanceDraining {
				nodeName := machine.NodeName
				if nodeName == "" {
					nodeName = ic.Status.NodeRef.Name
				}
				if nodeName != "" {
					input.Logger.Info("Setting draining annotation on node due to Instance draining",
						slog.String("instance", ic.Name),
						slog.String("node", nodeName))
					input.PatchCollector.PatchWithMerge(newDrainingAnnotationPatch(), "v1", "Node", "", nodeName)
				} else {
					input.Logger.Warn("Cannot set draining annotation: node name is empty",
						slog.String("instance", ic.Name),
						slog.String("machine", machine.Name))
				}
			}

			statusPatch := getInstanceStatusPatch(ic, machine, ng)
			if len(statusPatch) > 0 {
				patch := map[string]interface{}{
					"status": statusPatch,
				}
				input.PatchCollector.PatchWithMerge(patch, "deckhouse.io/v1alpha1", "Instance", "", ic.Name, object_patch.WithSubresource("/status"))
			}

			if ic.DeletionTimestamp != nil && !ic.DeletionTimestamp.IsZero() {
				if machine.DeletionTimestamp == nil || machine.DeletionTimestamp.IsZero() {
					// delete in background, because machine has finalizer
					input.PatchCollector.DeleteInBackground("cluster.x-k8s.io/v1beta1", "Machine", "d8-cloud-instance-manager", machine.Name)
				}
			}
		} else {
			newIc := newInstance(machine, ng)
			input.PatchCollector.CreateIfNotExists(newIc)
		}
	}

	// next, check mapping from instance  to machines
	// here we handle next cases:
	//   1. if machine exists, then skip it, we handle it above
	//   2. if machine does not exist:
	//  	a. we should delete finalizers only if metadata.deletionTimestamp is non-zero
	//		b. we should delete finalizers and delete instance if metadata.deletionTimestamp is zero
	for _, ic := range instances {
		_, machineExists := machines[ic.Name]
		_, clusterAPIMachineExists := clusterAPIMachines[ic.Name]

		if !machineExists && !clusterAPIMachineExists {
			input.PatchCollector.PatchWithMerge(deleteFinalizersPatch, "deckhouse.io/v1alpha1", "Instance", "", ic.Name)

			ds := ic.DeletionTimestamp
			if ds == nil || ds.IsZero() {
				input.PatchCollector.Delete("deckhouse.io/v1alpha1", "Instance", "", ic.Name)
			}
		}
	}

	return nil
}

func newInstance(machine *machineForInstance, ng *nodeGroupForInstance) *d8v1alpha1.Instance {
	phase := d8v1alpha1.InstancePhase(machine.CurrentStatus.Phase)
	lastUpdateTime := machine.CurrentStatus.LastUpdateTime
	if machine.ErrorInfo != nil && !isFailurePhase(phase) {
		phase = d8v1alpha1.InstanceFailed
		lastUpdateTime = machine.ErrorInfo.LastUpdateTime
	}
	var lastOperation d8v1alpha1.LastOperation
	if machine.LastOperation != nil {
		description := machine.LastOperation.Description
		shortDescription := truncateInstanceMessage(description)
		lastOperation = d8v1alpha1.LastOperation{
			LastUpdateTime:   machine.LastOperation.LastUpdateTime,
			Description:      description,
			ShortDescription: shortDescription,
			State:            d8v1alpha1.State(machine.LastOperation.State),
			Type:             d8v1alpha1.OperationType(machine.LastOperation.Type),
		}
	} else if machine.IsCAPI {
		if op := buildCAPILastOperation(machine, phase); op != nil {
			lastOperation = *op
		}
	}

	return &d8v1alpha1.Instance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Instance",
			APIVersion: "deckhouse.io/v1alpha1",
		},

		ObjectMeta: metav1.ObjectMeta{
			Name: machine.Name,
			Labels: map[string]string{
				"node.deckhouse.io/group": machine.NodeGroup,
			},

			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "deckhouse.io/v1",
					BlockOwnerDeletion: ptr.To(false),
					Controller:         ptr.To(false),
					Kind:               "NodeGroup",
					Name:               ng.Name,
					UID:                ng.UID,
				},
			},

			Finalizers: []string{
				"node-manager.hooks.deckhouse.io/instance-controller",
			},
		},

		Status: d8v1alpha1.InstanceStatus{
			ClassReference: d8v1alpha1.ClassReference{
				Name: ng.ClassReference.Name,
				Kind: ng.ClassReference.Kind,
			},
			MachineRef: d8v1alpha1.MachineRef{
				APIVersion: machine.APIVersion,
				Kind:       machine.Kind,
				Name:       machine.Name,
				Namespace:  "d8-cloud-instance-manager",
			},
			CurrentStatus: d8v1alpha1.CurrentStatus{
				Phase:          phase,
				LastUpdateTime: lastUpdateTime,
			},
			LastOperation: lastOperation,
		},
	}
}

func instanceLastOpMap(s map[string]interface{}) map[string]interface{} {
	m, ok := s["lastOperation"]
	if !ok {
		m = make(map[string]interface{})
		s["lastOperation"] = m
	}

	return m.(map[string]interface{})
}

func isLastOperationEmpty(op d8v1alpha1.LastOperation) bool {
	return op.Description == "" && op.ShortDescription == "" && op.State == "" && op.Type == "" && op.LastUpdateTime.IsZero()
}

func getInstanceStatusPatch(ic *instance, machine *machineForInstance, ng *nodeGroupForInstance) map[string]interface{} {
	status := make(map[string]interface{})

	if ic.Status.NodeRef.Name != machine.NodeName {
		status["nodeRef"] = map[string]interface{}{
			"name": machine.NodeName,
		}
	}

	// if machine is Running we do not need bootstrap info
	if machine.CurrentStatus.Phase == mcmv1alpha1.MachineRunning && (ic.Status.BootstrapStatus.LogsEndpoint != "" || ic.Status.BootstrapStatus.Description != "") {
		status["bootstrapStatus"] = nil
	}

	desiredPhase, desiredPhaseTime := resolveInstancePhase(ic, machine)
	if string(ic.Status.CurrentStatus.Phase) != string(desiredPhase) {
		status["currentStatus"] = map[string]interface{}{
			"phase":          string(desiredPhase),
			"lastUpdateTime": desiredPhaseTime.Format(time.RFC3339),
		}
	}

	if machine.IsCAPI && machine.LastOperation == nil {
		if op := buildCAPILastOperation(machine, desiredPhase); op != nil {
			m := instanceLastOpMap(status)
			if ic.Status.LastOperation.Description != op.Description {
				m["description"] = op.Description
			}
			if ic.Status.LastOperation.ShortDescription != op.ShortDescription {
				m["shortDescription"] = op.ShortDescription
			}
			if ic.Status.LastOperation.Type != op.Type {
				m["type"] = string(op.Type)
			}
			if ic.Status.LastOperation.State != op.State {
				m["state"] = string(op.State)
			}
			if !ic.Status.LastOperation.LastUpdateTime.Equal(&op.LastUpdateTime) {
				m["lastUpdateTime"] = op.LastUpdateTime.Format(time.RFC3339)
			}
		} else if !isLastOperationEmpty(ic.Status.LastOperation) {
			status["lastOperation"] = nil
		}
	}

	if machine.LastOperation != nil {
		shouldUpdateLastOp := true
		description := machine.LastOperation.Description
		shortDescription := truncateInstanceMessage(description)

		if ic.Status.LastOperation.Description != description {
			if machine.LastOperation.Description != "Started Machine creation process" {
				m := instanceLastOpMap(status)
				m["description"] = description
			} else {
				shouldUpdateLastOp = false
			}
		}
		if shouldUpdateLastOp {
			if ic.Status.LastOperation.ShortDescription != shortDescription {
				m := instanceLastOpMap(status)
				m["shortDescription"] = shortDescription
			}
			if string(ic.Status.LastOperation.Type) != string(machine.LastOperation.Type) {
				m := instanceLastOpMap(status)
				m["type"] = string(machine.LastOperation.Type)
			}

			if string(ic.Status.LastOperation.State) != string(machine.LastOperation.State) {
				m := instanceLastOpMap(status)
				m["state"] = string(machine.LastOperation.State)
			}

			if !ic.Status.LastOperation.LastUpdateTime.Equal(&machine.LastOperation.LastUpdateTime) {
				m := instanceLastOpMap(status)
				m["lastUpdateTime"] = machine.LastOperation.LastUpdateTime.Format(time.RFC3339)
			}
		}
	}

	if ic.Status.ClassReference.Kind != ng.ClassReference.Kind || ic.Status.ClassReference.Name != ng.ClassReference.Name {
		status["classReference"] = map[string]interface{}{
			"kind": ng.ClassReference.Kind,
			"name": ng.ClassReference.Name,
		}
	}

	return status
}

func resolveInstancePhase(ic *instance, machine *machineForInstance) (d8v1alpha1.InstancePhase, metav1.Time) {
	phase := d8v1alpha1.InstancePhase(machine.CurrentStatus.Phase)
	lastUpdateTime := machine.CurrentStatus.LastUpdateTime

	deleting := false
	if ic.DeletionTimestamp != nil && !ic.DeletionTimestamp.IsZero() {
		deleting = true
	}
	if machine.DeletionTimestamp != nil && !machine.DeletionTimestamp.IsZero() {
		deleting = true
	}
	if !deleting && isDeletionPhase(phase) {
		deleting = true
	}
	if deleting {
		if machine.IsCAPI {
			draining, drainTime := isCAPIDraining(machine)
			if draining {
				if drainTime != nil {
					return d8v1alpha1.InstanceDraining, *drainTime
				}
				return d8v1alpha1.InstanceDraining, metav1.NewTime(time.Now().UTC())
			}
		} else if isMCMDraining(machine) {
			if machine.LastOperation != nil {
				return d8v1alpha1.InstanceDraining, machine.LastOperation.LastUpdateTime
			}
			return d8v1alpha1.InstanceDraining, metav1.NewTime(time.Now().UTC())
		}
		if !isDeletionPhase(phase) {
			return d8v1alpha1.InstanceTerminating, metav1.NewTime(time.Now().UTC())
		}
		return phase, lastUpdateTime
	}

	if machine.ErrorInfo != nil && !isFailurePhase(phase) {
		return d8v1alpha1.InstanceFailed, machine.ErrorInfo.LastUpdateTime
	}

	return phase, lastUpdateTime
}

func isDeletionPhase(phase d8v1alpha1.InstancePhase) bool {
	if phase == d8v1alpha1.InstanceTerminating {
		return true
	}

	switch string(phase) {
	case "Deleting", "Deleted":
		return true
	default:
		return false
	}
}

func isFailurePhase(phase d8v1alpha1.InstancePhase) bool {
	return phase == d8v1alpha1.InstanceFailed || phase == d8v1alpha1.InstanceCrashLoopBackOff
}

func isCAPIDraining(machine *machineForInstance) (bool, *metav1.Time) {
	if machine == nil || !machine.IsCAPI {
		return false, nil
	}

	if found, draining, ts := capiDrainStatusFromCondition(machine.Conditions); found {
		return draining, ts
	}

	if cond := findCAPIConditionByPredicate(machine.Conditions, isCAPIConditionDraining); cond != nil {
		return true, &cond.LastTransitionTime
	}

	if machine.NodeDrainStartTime != nil {
		return true, machine.NodeDrainStartTime
	}

	return false, nil
}

func isMCMDraining(machine *machineForInstance) bool {
	// TODO(n-mcm-draining): implement MCM draining detection when requirements are clarified.
	return false
}

func buildCAPILastOperation(machine *machineForInstance, phase d8v1alpha1.InstancePhase) *d8v1alpha1.LastOperation {
	if machine == nil || !machine.IsCAPI {
		return nil
	}

	cond := selectCAPIMessageConditionByPriority(machine.Conditions)
	if cond == nil {
		return nil
	}

	description := cond.Message
	shortDescription := truncateInstanceMessage(description)
	operationType := d8v1alpha1.OperationHealthCheck
	state := d8v1alpha1.StateProcessing
	if phase == d8v1alpha1.InstanceDraining {
		operationType = d8v1alpha1.OperationDelete
	}
	if phase == d8v1alpha1.InstanceFailed {
		state = d8v1alpha1.StateFailed
	}

	return &d8v1alpha1.LastOperation{
		Description:      description,
		ShortDescription: shortDescription,
		LastUpdateTime:   cond.LastTransitionTime,
		State:            state,
		Type:             operationType,
	}
}

func truncateInstanceMessage(message string) string {
	if message == "" {
		return ""
	}

	runes := []rune(message)
	if len(runes) <= instanceMessageMaxRunes {
		return trimShortMessage(message)
	}

	return trimShortMessage(string(runes[:instanceMessageMaxRunes-3]) + "...")
}

func trimShortMessage(message string) string {
	trimmed := strings.TrimRight(message, " \t\r\n")
	if strings.HasSuffix(trimmed, "-") || strings.HasSuffix(trimmed, ":") {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return trimmed
}

func selectCAPIMessageConditionByPriority(conditions capi.Conditions) *capi.Condition {
	var selected *capi.Condition
	bestPriority := 1000

	for i := range conditions {
		cond := conditions[i]
		if cond.Message == "" {
			continue
		}
		priority := capiConditionPriority(cond.Type)
		if selected == nil || priority < bestPriority || (priority == bestPriority && isPreferableCAPICondition(cond, *selected)) {
			current := cond
			selected = &current
			bestPriority = priority
		}
	}

	return selected
}

func isPreferableCAPICondition(candidate, current capi.Condition) bool {
	if candidate.Message != "" && current.Message == "" {
		return true
	}
	if candidate.Message == "" && current.Message != "" {
		return false
	}
	return candidate.LastTransitionTime.After(current.LastTransitionTime.Time)
}

func capiConditionPriority(conditionType capi.ConditionType) int {
	if priority, ok := capiMachineConditionPriority[conditionType]; ok {
		return priority
	}
	return 100
}

func capiDrainStatusFromCondition(conditions capi.Conditions) (bool, bool, *metav1.Time) {
	for i := range conditions {
		cond := conditions[i]
		if cond.Type != capi.DrainingSucceededCondition {
			continue
		}

		switch cond.Status {
		case corev1.ConditionTrue:
			return true, false, &cond.LastTransitionTime
		case corev1.ConditionFalse, corev1.ConditionUnknown:
			return true, true, &cond.LastTransitionTime
		default:
			return true, true, &cond.LastTransitionTime
		}
	}

	return false, false, nil
}

func findCAPIConditionByPredicate(conditions capi.Conditions, predicate func(capi.Condition) bool) *capi.Condition {
	for i := range conditions {
		cond := conditions[i]
		if predicate(cond) {
			return &cond
		}
	}
	return nil
}

func isCAPIConditionDraining(cond capi.Condition) bool {
	if cond.Reason == capi.DrainingReason || cond.Reason == capi.DrainingFailedReason {
		return true
	}
	if strings.HasPrefix(cond.Reason, capi.DrainingReason) {
		return true
	}
	return strings.Contains(strings.ToLower(cond.Message), "drain")
}

func capiMachineErrorInfo(machine *clusterapi.Machine) *machineErrorInfo {
	if machine == nil {
		return nil
	}

	if cond := selectCAPIErrorCondition(capiMachineConditions(machine)); cond != nil {
		return &machineErrorInfo{
			Description:    formatCAPIConditionDescription(*cond),
			LastUpdateTime: cond.LastTransitionTime,
		}
	}

	description := formatCAPIFailureDescription(machine.Status.FailureReason, machine.Status.FailureMessage)
	if description == "" {
		return nil
	}

	lastUpdateTime := metav1.NewTime(time.Now().UTC())
	if machine.Status.LastUpdated != nil {
		lastUpdateTime = *machine.Status.LastUpdated
	}

	return &machineErrorInfo{
		Description:    description,
		LastUpdateTime: lastUpdateTime,
	}
}

func capiMachineConditions(machine *clusterapi.Machine) capi.Conditions {
	if machine == nil {
		return nil
	}

	return machine.Status.Conditions
}

func selectCAPIErrorCondition(conditions capi.Conditions) *capi.Condition {
	var selected *capi.Condition
	bestPriority := 1000

	for i := range conditions {
		cond := conditions[i]
		if cond.Status != corev1.ConditionFalse {
			continue
		}
		if cond.Severity != capi.ConditionSeverityError {
			continue
		}

		priority, ok := capiMachineConditionPriority[cond.Type]
		if !ok {
			priority = 100
		}
		if selected == nil || priority < bestPriority {
			current := cond
			selected = &current
			bestPriority = priority
		}
	}

	return selected
}

func formatCAPIConditionDescription(cond capi.Condition) string {
	return joinNonEmpty(string(cond.Type), cond.Reason, cond.Message)
}

func formatCAPIFailureDescription(reason, message *string) string {
	return joinNonEmpty(derefString(reason), derefString(message))
}

func joinNonEmpty(parts ...string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, ": ")
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type machineForInstance struct {
	APIVersion         string
	Kind               string
	NodeGroup          string
	NodeName           string
	Name               string
	CurrentStatus      mcmv1alpha1.CurrentStatus
	LastOperation      *mcmv1alpha1.LastOperation
	DeletionTimestamp  *metav1.Time
	IsCAPI             bool
	Conditions         capi.Conditions
	NodeDrainStartTime *metav1.Time
	ErrorInfo          *machineErrorInfo
}

type nodeGroupForInstance struct {
	Name           string
	UID            k8stypes.UID
	ClassReference d8v1.ClassReference
}

type instance struct {
	Name              string
	DeletionTimestamp *metav1.Time
	Status            d8v1alpha1.InstanceStatus
}

type machineErrorInfo struct {
	Description    string
	LastUpdateTime metav1.Time
}
