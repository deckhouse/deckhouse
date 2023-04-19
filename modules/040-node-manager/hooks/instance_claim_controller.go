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
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	mcmv1alpha1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/mcm/v1alpha1"
	d8v1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
	d8v1alpha1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:       "/modules/node-manager/instance_claim_controller",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "instances",
			Kind:       "InstanceClaim",
			ApiVersion: "deckhouse.io/v1alpha1",
			FilterFunc: instanceClaimFilter,
		},
		{
			Name:       "ngs",
			Kind:       "NodeGroup",
			ApiVersion: "deckhouse.io/v1",
			FilterFunc: instanceClaimNodeGroupFilter,
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
			FilterFunc: instanceClaimMachineFilter,
		},
	},
}, instanceClaimController)

func instanceClaimMachineFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var machine mcmv1alpha1.Machine

	err := sdk.FromUnstructured(obj, &machine)
	if err != nil {
		return nil, err
	}

	return &machineForInstanceClaim{
		NodeGroup:         machine.Spec.NodeTemplateSpec.Labels["node.deckhouse.io/group"],
		NodeName:          machine.GetLabels()["node"],
		Name:              machine.GetName(),
		CurrentStatus:     machine.Status.CurrentStatus,
		LastOperation:     machine.Status.LastOperation,
		DeletionTimestamp: machine.GetDeletionTimestamp(),
	}, nil
}

func instanceClaimNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng d8v1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	return &nodeGroupForInstanceClaim{
		Name: ng.GetName(),
		UID:  ng.GetUID(),
	}, nil
}

func instanceClaimFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ic d8v1alpha1.InstanceClaim

	err := sdk.FromUnstructured(obj, &ic)
	if err != nil {
		return nil, err
	}

	return &ic, nil
}

func instanceClaimController(input *go_hook.HookInput) error {
	instanceClaims := make(map[string]*d8v1alpha1.InstanceClaim)
	machines := make(map[string]*machineForInstanceClaim)
	nodeGroups := make(map[string]*nodeGroupForInstanceClaim)

	for _, i := range input.Snapshots["instances"] {
		ins := i.(*d8v1alpha1.InstanceClaim)
		instanceClaims[ins.GetName()] = ins
	}

	for _, m := range input.Snapshots["machines"] {
		mc := m.(*machineForInstanceClaim)
		machines[mc.Name] = mc
	}

	for _, m := range input.Snapshots["ngs"] {
		ng := m.(*nodeGroupForInstanceClaim)
		nodeGroups[ng.Name] = ng
	}

	// first, check mapping from machines to instance claims
	// here we handle two cases:
	//   1. if instance claim exists, then we have 2 subcases:
	//  	a. first, we should compare machines fields and instance claim status and if we have changes path status
	//		b. next, we should check metadata.deletionTimestamp for instance claim
	//         if we have non-zero timestamp this means that client deletes instance claim, and we should delete machine.
	//         Before deletion, we should check metadata.deletionTimestamp in the machine for prevent multiple deletion
	//   2. if instance claim does not exist, create instance claim for machine
	for name, machine := range machines {
		if ic, ok := instanceClaims[name]; ok {
			statusPatch := getInstanceClaimStatusPatch(ic, machine)
			if len(statusPatch) > 0 {
				patch := map[string]interface{}{
					"status": statusPatch,
				}
				input.PatchCollector.MergePatch(patch, "deckhouse.io/v1alpha1", "InstanceClaim", "", ic.Name, object_patch.WithSubresource("/status"))
			}

			if ic.DeletionTimestamp != nil && !ic.DeletionTimestamp.IsZero() {
				if machine.DeletionTimestamp == nil || machine.DeletionTimestamp.IsZero() {
					// delete in background, because machine has finalizer
					input.PatchCollector.Delete("machine.sapcloud.io/v1alpha1", "Machine", "d8-cloud-instance-manager", machine.Name, object_patch.InBackground())
				}
			}
		} else {
			ng, ok := nodeGroups[machine.NodeGroup]
			if !ok {
				return fmt.Errorf("NodeGroup %s not found", machine.NodeGroup)
			}
			newIc := newInstanceClaim(machine, ng)
			input.PatchCollector.Create(newIc, object_patch.IgnoreIfExists())
		}
	}

	// next, check mapping from instance claims to machines
	// here we handle next cases:
	//   1. if machine exists, then skip it, we handle it above
	//   2. if machine does not exist:
	//  	a. we should delete finalizers only if metadata.deletionTimestamp is non-zero
	//		b. we should delete finalizers and delete instance claim if metadata.deletionTimestamp is zero
	for _, ic := range instanceClaims {
		if _, ok := machines[ic.GetName()]; !ok {
			deleteFinalizersPatch := map[string]interface{}{
				"metadata": map[string]interface{}{
					"finalizers": nil,
				},
			}
			input.PatchCollector.MergePatch(deleteFinalizersPatch, "deckhouse.io/v1alpha1", "InstanceClaim", "", ic.Name)

			ds := ic.GetDeletionTimestamp()
			if ds == nil || ds.IsZero() {
				input.PatchCollector.Delete("deckhouse.io/v1alpha1", "InstanceClaim", "", ic.Name)
			}
		}
	}

	return nil
}

func newInstanceClaim(machine *machineForInstanceClaim, ng *nodeGroupForInstanceClaim) *d8v1alpha1.InstanceClaim {
	return &d8v1alpha1.InstanceClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "InstanceClaim",
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
					BlockOwnerDeletion: pointer.Bool(false),
					Controller:         pointer.Bool(false),
					Kind:               "NodeGroup",
					Name:               ng.Name,
					UID:                ng.UID,
				},
			},

			Finalizers: []string{
				"node-manager.hooks.deckhouse.io/instance-claim-controller",
			},
		},

		Status: d8v1alpha1.InstanceClaimStatus{
			MachineRef: d8v1alpha1.MachineRef{
				APIVersion: "machine.sapcloud.io/v1alpha1",
				Kind:       "Machine",
				Name:       machine.Name,
				Namespace:  "d8-cloud-instance-manager",
			},
			CurrentStatus: d8v1alpha1.CurrentStatus{
				Phase:          d8v1alpha1.InstanceClaimPhase(machine.CurrentStatus.Phase),
				LastUpdateTime: machine.CurrentStatus.LastUpdateTime,
			},
			LastOperation: d8v1alpha1.LastOperation{
				LastUpdateTime: machine.LastOperation.LastUpdateTime,
				Description:    machine.LastOperation.Description,
				State:          d8v1alpha1.State(machine.LastOperation.State),
				Type:           d8v1alpha1.OperationType(machine.LastOperation.Type),
			},
		},
	}
}

func instanceClaimLastOpMap(s map[string]interface{}) map[string]interface{} {
	m, ok := s["lastOperation"]
	if !ok {
		m = make(map[string]interface{})
		s["lastOperation"] = m
	}

	return m.(map[string]interface{})
}

func getInstanceClaimStatusPatch(ic *d8v1alpha1.InstanceClaim, machine *machineForInstanceClaim) map[string]interface{} {
	status := make(map[string]interface{})

	if ic.Status.NodeRef.Name != machine.NodeName {
		status["nodeRef"] = map[string]interface{}{
			"name": machine.NodeName,
		}
	}

	if string(ic.Status.CurrentStatus.Phase) != string(machine.CurrentStatus.Phase) {
		status["currentStatus"] = map[string]interface{}{
			"phase":          string(machine.CurrentStatus.Phase),
			"lastUpdateTime": machine.CurrentStatus.LastUpdateTime.Format(time.RFC3339),
		}
	}

	if ic.Status.LastOperation.Description != machine.LastOperation.Description {
		m := instanceClaimLastOpMap(status)
		m["description"] = machine.LastOperation.Description
	}

	if string(ic.Status.LastOperation.Type) != string(machine.LastOperation.Type) {
		m := instanceClaimLastOpMap(status)
		m["type"] = string(machine.LastOperation.Type)
	}

	if string(ic.Status.LastOperation.State) != string(machine.LastOperation.State) {
		m := instanceClaimLastOpMap(status)
		m["state"] = string(machine.LastOperation.State)
	}

	if !ic.Status.LastOperation.LastUpdateTime.Equal(&machine.LastOperation.LastUpdateTime) {
		m := instanceClaimLastOpMap(status)
		m["lastUpdateTime"] = machine.LastOperation.LastUpdateTime.Format(time.RFC3339)
	}

	return status
}

type machineForInstanceClaim struct {
	NodeGroup         string
	NodeName          string
	Name              string
	CurrentStatus     mcmv1alpha1.CurrentStatus
	LastOperation     mcmv1alpha1.LastOperation
	DeletionTimestamp *metav1.Time
}

type nodeGroupForInstanceClaim struct {
	Name string
	UID  k8stypes.UID
}
