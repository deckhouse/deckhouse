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

package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// ConvertTo converts this NodeGroup (v1alpha1) to the Hub version (v1).
func (ng *NodeGroup) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.NodeGroup)

	// Convert ObjectMeta
	dst.ObjectMeta = ng.ObjectMeta

	// Convert Spec using custom conversion function
	if err := ConvertV1alpha1NodeGroupSpecToV1NodeGroupSpec(&ng.Spec, &dst.Spec, nil); err != nil {
		return err
	}

	// Convert Status
	if err := convertStatusTo(&ng.Status, &dst.Status); err != nil {
		return err
	}

	return nil
}

// ConvertFrom converts the Hub version (v1) to this NodeGroup (v1alpha1).
func (ng *NodeGroup) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.NodeGroup)

	// Convert ObjectMeta
	ng.ObjectMeta = src.ObjectMeta

	// Convert Spec using custom conversion function
	if err := ConvertV1NodeGroupSpecToV1alpha1NodeGroupSpec(&src.Spec, &ng.Spec, nil); err != nil {
		return err
	}

	// Convert Status
	if err := convertStatusFrom(&src.Status, &ng.Status); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts NodeGroupList (v1alpha1) to the Hub version (v1).
func (ng *NodeGroupList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.NodeGroupList)

	dst.ListMeta = ng.ListMeta
	dst.Items = make([]v1.NodeGroup, len(ng.Items))

	for i := range ng.Items {
		if err := ng.Items[i].ConvertTo(&dst.Items[i]); err != nil {
			return err
		}
	}

	return nil
}

// ConvertFrom converts NodeGroupList from the Hub version (v1) to v1alpha1.
func (ng *NodeGroupList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.NodeGroupList)

	ng.ListMeta = src.ListMeta
	ng.Items = make([]NodeGroup, len(src.Items))

	for i := range src.Items {
		if err := ng.Items[i].ConvertFrom(&src.Items[i]); err != nil {
			return err
		}
	}

	return nil
}

// convertStatusTo converts v1alpha1.NodeGroupStatus to v1.NodeGroupStatus
func convertStatusTo(in *NodeGroupStatus, out *v1.NodeGroupStatus) error {
	out.Ready = in.Ready
	out.Nodes = in.Nodes
	out.Instances = in.Instances
	out.Desired = in.Desired
	out.Min = in.Min
	out.Max = in.Max
	out.UpToDate = in.UpToDate
	out.Standby = in.Standby
	out.Error = in.Error
	out.KubernetesVersion = in.KubernetesVersion

	if in.ConditionSummary != nil {
		out.ConditionSummary = &v1.ConditionSummary{
			StatusMessage: in.ConditionSummary.StatusMessage,
			Ready:         in.ConditionSummary.Ready,
		}
	}

	for _, mf := range in.LastMachineFailures {
		failure := v1.MachineFailure{
			Name:       mf.Name,
			ProviderID: mf.ProviderID,
			OwnerRef:   mf.OwnerRef,
		}
		if mf.LastOperation != nil {
			failure.LastOperation = &v1.MachineLastOperation{
				Description:    mf.LastOperation.Description,
				LastUpdateTime: mf.LastOperation.LastUpdateTime,
				State:          mf.LastOperation.State,
				Type:           mf.LastOperation.Type,
			}
		}
		out.LastMachineFailures = append(out.LastMachineFailures, failure)
	}

	return nil
}

// convertStatusFrom converts v1.NodeGroupStatus to v1alpha1.NodeGroupStatus
func convertStatusFrom(in *v1.NodeGroupStatus, out *NodeGroupStatus) error {
	out.Ready = in.Ready
	out.Nodes = in.Nodes
	out.Instances = in.Instances
	out.Desired = in.Desired
	out.Min = in.Min
	out.Max = in.Max
	out.UpToDate = in.UpToDate
	out.Standby = in.Standby
	out.Error = in.Error
	out.KubernetesVersion = in.KubernetesVersion

	if in.ConditionSummary != nil {
		out.ConditionSummary = &ConditionSummary{
			StatusMessage: in.ConditionSummary.StatusMessage,
			Ready:         in.ConditionSummary.Ready,
		}
	}

	for _, mf := range in.LastMachineFailures {
		failure := MachineFailure{
			Name:       mf.Name,
			ProviderID: mf.ProviderID,
			OwnerRef:   mf.OwnerRef,
		}
		if mf.LastOperation != nil {
			failure.LastOperation = &MachineLastOperation{
				Description:    mf.LastOperation.Description,
				LastUpdateTime: mf.LastOperation.LastUpdateTime,
				State:          mf.LastOperation.State,
				Type:           mf.LastOperation.Type,
			}
		}
		out.LastMachineFailures = append(out.LastMachineFailures, failure)
	}

	// Note: v1.Status.Conditions is lost (not in v1alpha1)

	return nil
}
