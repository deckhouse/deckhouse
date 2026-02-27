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

package v1alpha2

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

var conversionlog = logf.Log.WithName("nodegroup-conversion-v1alpha2")

// ConvertTo converts this NodeGroup (v1alpha2) to the Hub version (v1).
func (ng *NodeGroup) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.NodeGroup)
	conversionlog.V(1).Info("converting NodeGroup from v1alpha2 to v1", "name", ng.Name)

	// Convert ObjectMeta
	dst.ObjectMeta = ng.ObjectMeta

	// Map nodeType
	switch ng.Spec.NodeType {
	case NodeTypeCloud:
		dst.Spec.NodeType = v1.NodeTypeCloudEphemeral
	case NodeTypeStatic:
		dst.Spec.NodeType = v1.NodeTypeStatic
	case NodeTypeHybrid:
		dst.Spec.NodeType = v1.NodeTypeCloudStatic
	default:
		dst.Spec.NodeType = v1.NodeType(ng.Spec.NodeType)
	}

	// Convert CRI
	if ng.Spec.CRI != nil {
		dst.Spec.CRI = &v1.CRISpec{
			Type: v1.CRIType(ng.Spec.CRI.Type),
		}
		if ng.Spec.CRI.Containerd != nil {
			dst.Spec.CRI.Containerd = &v1.ContainerdSpec{
				MaxConcurrentDownloads: ng.Spec.CRI.Containerd.MaxConcurrentDownloads,
			}
		}
		if ng.Spec.CRI.Docker != nil {
			dst.Spec.CRI.Docker = &v1.DockerSpec{
				MaxConcurrentDownloads: ng.Spec.CRI.Docker.MaxConcurrentDownloads,
				Manage:                 ng.Spec.CRI.Docker.Manage,
			}
		}
		if ng.Spec.CRI.NotManaged != nil {
			dst.Spec.CRI.NotManaged = &v1.NotManagedCRISpec{
				CRISocketPath: ng.Spec.CRI.NotManaged.CRISocketPath,
			}
		}
	}

	// Convert CloudInstances
	if ng.Spec.CloudInstances != nil {
		dst.Spec.CloudInstances = &v1.CloudInstancesSpec{
			Zones:                 ng.Spec.CloudInstances.Zones,
			MinPerZone:            ng.Spec.CloudInstances.MinPerZone,
			MaxPerZone:            ng.Spec.CloudInstances.MaxPerZone,
			MaxUnavailablePerZone: ng.Spec.CloudInstances.MaxUnavailablePerZone,
			MaxSurgePerZone:       ng.Spec.CloudInstances.MaxSurgePerZone,
			Standby:               ng.Spec.CloudInstances.Standby,
			ClassReference: v1.ClassReference{
				Kind: ng.Spec.CloudInstances.ClassReference.Kind,
				Name: ng.Spec.CloudInstances.ClassReference.Name,
			},
		}
		if ng.Spec.CloudInstances.StandbyHolder != nil {
			dst.Spec.CloudInstances.StandbyHolder = &v1.StandbyHolderSpec{}
			if ng.Spec.CloudInstances.StandbyHolder.NotHeldResources != nil {
				dst.Spec.CloudInstances.StandbyHolder.NotHeldResources = &v1.NotHeldResourcesSpec{
					CPU:    ng.Spec.CloudInstances.StandbyHolder.NotHeldResources.CPU,
					Memory: ng.Spec.CloudInstances.StandbyHolder.NotHeldResources.Memory,
				}
			}
		}
	}

	// Convert NodeTemplate
	if ng.Spec.NodeTemplate != nil {
		dst.Spec.NodeTemplate = &v1.NodeTemplate{
			Labels:      ng.Spec.NodeTemplate.Labels,
			Annotations: ng.Spec.NodeTemplate.Annotations,
			Taints:      ng.Spec.NodeTemplate.Taints,
		}
	}

	// Convert Chaos
	if ng.Spec.Chaos != nil {
		dst.Spec.Chaos = &v1.ChaosSpec{
			Mode:   v1.ChaosMode(ng.Spec.Chaos.Mode),
			Period: ng.Spec.Chaos.Period,
		}
	}

	// Convert OperatingSystem
	if ng.Spec.OperatingSystem != nil {
		dst.Spec.OperatingSystem = &v1.OperatingSystemSpec{
			ManageKernel: ng.Spec.OperatingSystem.ManageKernel,
		}
	}

	// Convert Disruptions
	if ng.Spec.Disruptions != nil {
		dst.Spec.Disruptions = &v1.DisruptionsSpec{
			ApprovalMode: v1.DisruptionApprovalMode(ng.Spec.Disruptions.ApprovalMode),
		}
		if ng.Spec.Disruptions.Automatic != nil {
			dst.Spec.Disruptions.Automatic = &v1.AutomaticDisruptionSpec{
				DrainBeforeApproval: ng.Spec.Disruptions.Automatic.DrainBeforeApproval,
			}
			for _, w := range ng.Spec.Disruptions.Automatic.Windows {
				dst.Spec.Disruptions.Automatic.Windows = append(dst.Spec.Disruptions.Automatic.Windows,
					v1.DisruptionWindow{From: w.From, To: w.To, Days: w.Days})
			}
		}
		if ng.Spec.Disruptions.RollingUpdate != nil {
			dst.Spec.Disruptions.RollingUpdate = &v1.RollingUpdateDisruptionSpec{}
			for _, w := range ng.Spec.Disruptions.RollingUpdate.Windows {
				dst.Spec.Disruptions.RollingUpdate.Windows = append(dst.Spec.Disruptions.RollingUpdate.Windows,
					v1.DisruptionWindow{From: w.From, To: w.To, Days: w.Days})
			}
		}
	}

	// Convert Kubelet
	if ng.Spec.Kubelet != nil {
		dst.Spec.Kubelet = &v1.KubeletSpec{
			MaxPods:              ng.Spec.Kubelet.MaxPods,
			RootDir:              ng.Spec.Kubelet.RootDir,
			ContainerLogMaxSize:  ng.Spec.Kubelet.ContainerLogMaxSize,
			ContainerLogMaxFiles: ng.Spec.Kubelet.ContainerLogMaxFiles,
		}
	}

	// Convert Status
	if err := convertStatusTo(&ng.Status, &dst.Status); err != nil {
		return err
	}

	return nil
}

// ConvertFrom converts the Hub version (v1) to this NodeGroup (v1alpha2).
func (ng *NodeGroup) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.NodeGroup)
	conversionlog.V(1).Info("converting NodeGroup from v1 to v1alpha2", "name", src.Name)

	// Convert ObjectMeta
	ng.ObjectMeta = src.ObjectMeta

	// Map nodeType (reverse)
	switch src.Spec.NodeType {
	case v1.NodeTypeCloudEphemeral:
		ng.Spec.NodeType = NodeTypeCloud
	case v1.NodeTypeStatic:
		ng.Spec.NodeType = NodeTypeStatic
	case v1.NodeTypeCloudStatic, v1.NodeTypeCloudPermanent:
		ng.Spec.NodeType = NodeTypeHybrid
	default:
		ng.Spec.NodeType = NodeType(src.Spec.NodeType)
	}

	// Convert CRI
	if src.Spec.CRI != nil {
		criType := src.Spec.CRI.Type
		// ContainerdV2 downgrades to Containerd
		if criType == v1.CRITypeContainerdV2 {
			criType = v1.CRITypeContainerd
		}

		ng.Spec.CRI = &CRISpec{
			Type: CRIType(criType),
		}
		if src.Spec.CRI.Containerd != nil {
			ng.Spec.CRI.Containerd = &ContainerdSpec{
				MaxConcurrentDownloads: src.Spec.CRI.Containerd.MaxConcurrentDownloads,
			}
		}
		// ContainerdV2 settings go to Containerd
		if src.Spec.CRI.ContainerdV2 != nil && ng.Spec.CRI.Containerd == nil {
			ng.Spec.CRI.Containerd = &ContainerdSpec{
				MaxConcurrentDownloads: src.Spec.CRI.ContainerdV2.MaxConcurrentDownloads,
			}
		}
		if src.Spec.CRI.Docker != nil {
			ng.Spec.CRI.Docker = &DockerSpec{
				MaxConcurrentDownloads: src.Spec.CRI.Docker.MaxConcurrentDownloads,
				Manage:                 src.Spec.CRI.Docker.Manage,
			}
		}
		if src.Spec.CRI.NotManaged != nil {
			ng.Spec.CRI.NotManaged = &NotManagedCRISpec{
				CRISocketPath: src.Spec.CRI.NotManaged.CRISocketPath,
			}
		}
	}

	// Convert CloudInstances
	if src.Spec.CloudInstances != nil {
		ng.Spec.CloudInstances = &CloudInstancesSpec{
			Zones:                 src.Spec.CloudInstances.Zones,
			MinPerZone:            src.Spec.CloudInstances.MinPerZone,
			MaxPerZone:            src.Spec.CloudInstances.MaxPerZone,
			MaxUnavailablePerZone: src.Spec.CloudInstances.MaxUnavailablePerZone,
			MaxSurgePerZone:       src.Spec.CloudInstances.MaxSurgePerZone,
			Standby:               src.Spec.CloudInstances.Standby,
			ClassReference: ClassReference{
				Kind: src.Spec.CloudInstances.ClassReference.Kind,
				Name: src.Spec.CloudInstances.ClassReference.Name,
			},
		}
		if src.Spec.CloudInstances.StandbyHolder != nil {
			ng.Spec.CloudInstances.StandbyHolder = &StandbyHolderSpec{}
			if src.Spec.CloudInstances.StandbyHolder.NotHeldResources != nil {
				ng.Spec.CloudInstances.StandbyHolder.NotHeldResources = &NotHeldResourcesSpec{
					CPU:    src.Spec.CloudInstances.StandbyHolder.NotHeldResources.CPU,
					Memory: src.Spec.CloudInstances.StandbyHolder.NotHeldResources.Memory,
				}
			}
		}
	}

	// Convert NodeTemplate
	if src.Spec.NodeTemplate != nil {
		ng.Spec.NodeTemplate = &NodeTemplate{
			Labels:      src.Spec.NodeTemplate.Labels,
			Annotations: src.Spec.NodeTemplate.Annotations,
			Taints:      src.Spec.NodeTemplate.Taints,
		}
	}

	// Convert Chaos
	if src.Spec.Chaos != nil {
		ng.Spec.Chaos = &ChaosSpec{
			Mode:   ChaosMode(src.Spec.Chaos.Mode),
			Period: src.Spec.Chaos.Period,
		}
	}

	// Convert OperatingSystem
	if src.Spec.OperatingSystem != nil {
		ng.Spec.OperatingSystem = &OperatingSystemSpec{
			ManageKernel: src.Spec.OperatingSystem.ManageKernel,
		}
	}

	// Convert Disruptions
	if src.Spec.Disruptions != nil {
		ng.Spec.Disruptions = &DisruptionsSpec{
			ApprovalMode: DisruptionApprovalMode(src.Spec.Disruptions.ApprovalMode),
		}
		if src.Spec.Disruptions.Automatic != nil {
			ng.Spec.Disruptions.Automatic = &AutomaticDisruptionSpec{
				DrainBeforeApproval: src.Spec.Disruptions.Automatic.DrainBeforeApproval,
			}
			for _, w := range src.Spec.Disruptions.Automatic.Windows {
				ng.Spec.Disruptions.Automatic.Windows = append(ng.Spec.Disruptions.Automatic.Windows,
					DisruptionWindow{From: w.From, To: w.To, Days: w.Days})
			}
		}
		if src.Spec.Disruptions.RollingUpdate != nil {
			ng.Spec.Disruptions.RollingUpdate = &RollingUpdateDisruptionSpec{}
			for _, w := range src.Spec.Disruptions.RollingUpdate.Windows {
				ng.Spec.Disruptions.RollingUpdate.Windows = append(ng.Spec.Disruptions.RollingUpdate.Windows,
					DisruptionWindow{From: w.From, To: w.To, Days: w.Days})
			}
		}
	}

	// Convert Kubelet
	if src.Spec.Kubelet != nil {
		ng.Spec.Kubelet = &KubeletSpec{
			MaxPods:              src.Spec.Kubelet.MaxPods,
			RootDir:              src.Spec.Kubelet.RootDir,
			ContainerLogMaxSize:  src.Spec.Kubelet.ContainerLogMaxSize,
			ContainerLogMaxFiles: src.Spec.Kubelet.ContainerLogMaxFiles,
		}
	}

	// Convert Status
	if err := convertStatusFrom(&src.Status, &ng.Status); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts NodeGroupList (v1alpha2) to the Hub version (v1).
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

// ConvertFrom converts NodeGroupList from the Hub version (v1) to v1alpha2.
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

// convertStatusTo converts v1alpha2.NodeGroupStatus to v1.NodeGroupStatus
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

// convertStatusFrom converts v1.NodeGroupStatus to v1alpha2.NodeGroupStatus
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

	return nil
}
