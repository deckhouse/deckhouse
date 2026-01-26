package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/deckhouse/node-controller/api/v1"
)

// ConvertTo converts this NodeGroup (v1alpha1) to the Hub version (v1)
func (src *NodeGroup) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.NodeGroup)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Convert nodeType: Cloud -> CloudEphemeral, Static -> Static, Hybrid -> CloudStatic
	switch src.Spec.NodeType {
	case NodeTypeCloud:
		dst.Spec.NodeType = v1.NodeTypeCloudEphemeral
	case NodeTypeStatic:
		dst.Spec.NodeType = v1.NodeTypeStatic
	case NodeTypeHybrid:
		dst.Spec.NodeType = v1.NodeTypeCloudStatic
	default:
		dst.Spec.NodeType = v1.NodeType(src.Spec.NodeType)
	}

	// CRI
	if src.Spec.CRI != nil {
		dst.Spec.CRI = &v1.CRISpec{
			Type: v1.CRIType(src.Spec.CRI.Type),
		}
		if src.Spec.CRI.Containerd != nil {
			dst.Spec.CRI.Containerd = &v1.ContainerdSpec{
				MaxConcurrentDownloads: src.Spec.CRI.Containerd.MaxConcurrentDownloads,
			}
		}
	}

	// Docker -> CRI.Docker (deprecated field in v1alpha1)
	if src.Spec.Docker != nil && dst.Spec.CRI == nil {
		dst.Spec.CRI = &v1.CRISpec{
			Type: v1.CRITypeDocker,
			Docker: &v1.DockerSpec{
				MaxConcurrentDownloads: src.Spec.Docker.MaxConcurrentDownloads,
				Manage:                 src.Spec.Docker.Manage,
			},
		}
	}

	// CloudInstances
	if src.Spec.CloudInstances != nil {
		dst.Spec.CloudInstances = &v1.CloudInstancesSpec{
			Zones:                 src.Spec.CloudInstances.Zones,
			MinPerZone:            src.Spec.CloudInstances.MinPerZone,
			MaxPerZone:            src.Spec.CloudInstances.MaxPerZone,
			MaxUnavailablePerZone: src.Spec.CloudInstances.MaxUnavailablePerZone,
			MaxSurgePerZone:       src.Spec.CloudInstances.MaxSurgePerZone,
			Standby:               src.Spec.CloudInstances.Standby,
			ClassReference: v1.ClassReference{
				Kind: src.Spec.CloudInstances.ClassReference.Kind,
				Name: src.Spec.CloudInstances.ClassReference.Name,
			},
		}
		if src.Spec.CloudInstances.StandbyHolder != nil {
			dst.Spec.CloudInstances.StandbyHolder = &v1.StandbyHolderSpec{}
			if src.Spec.CloudInstances.StandbyHolder.NotHeldResources != nil {
				dst.Spec.CloudInstances.StandbyHolder.NotHeldResources = &v1.NotHeldResourcesSpec{
					CPU:    src.Spec.CloudInstances.StandbyHolder.NotHeldResources.CPU,
					Memory: src.Spec.CloudInstances.StandbyHolder.NotHeldResources.Memory,
				}
			}
		}
	}

	// NodeTemplate
	if src.Spec.NodeTemplate != nil {
		dst.Spec.NodeTemplate = &v1.NodeTemplate{
			Labels:      src.Spec.NodeTemplate.Labels,
			Annotations: src.Spec.NodeTemplate.Annotations,
			Taints:      src.Spec.NodeTemplate.Taints,
		}
	}

	// Chaos
	if src.Spec.Chaos != nil {
		dst.Spec.Chaos = &v1.ChaosSpec{
			Mode:   v1.ChaosMode(src.Spec.Chaos.Mode),
			Period: src.Spec.Chaos.Period,
		}
	}

	// OperatingSystem
	if src.Spec.OperatingSystem != nil {
		dst.Spec.OperatingSystem = &v1.OperatingSystemSpec{
			ManageKernel: src.Spec.OperatingSystem.ManageKernel,
		}
	}

	// Disruptions
	if src.Spec.Disruptions != nil {
		dst.Spec.Disruptions = &v1.DisruptionsSpec{
			ApprovalMode: v1.DisruptionApprovalMode(src.Spec.Disruptions.ApprovalMode),
		}
		if src.Spec.Disruptions.Automatic != nil {
			dst.Spec.Disruptions.Automatic = &v1.AutomaticDisruptionSpec{
				DrainBeforeApproval: src.Spec.Disruptions.Automatic.DrainBeforeApproval,
			}
			for _, w := range src.Spec.Disruptions.Automatic.Windows {
				dst.Spec.Disruptions.Automatic.Windows = append(dst.Spec.Disruptions.Automatic.Windows,
					v1.DisruptionWindow{
						From: w.From,
						To:   w.To,
						Days: w.Days,
					})
			}
		}
		if src.Spec.Disruptions.RollingUpdate != nil {
			dst.Spec.Disruptions.RollingUpdate = &v1.RollingUpdateDisruptionSpec{}
			for _, w := range src.Spec.Disruptions.RollingUpdate.Windows {
				dst.Spec.Disruptions.RollingUpdate.Windows = append(dst.Spec.Disruptions.RollingUpdate.Windows,
					v1.DisruptionWindow{
						From: w.From,
						To:   w.To,
						Days: w.Days,
					})
			}
		}
	}

	// Kubelet
	if src.Spec.Kubelet != nil {
		dst.Spec.Kubelet = &v1.KubeletSpec{
			MaxPods:              src.Spec.Kubelet.MaxPods,
			RootDir:              src.Spec.Kubelet.RootDir,
			ContainerLogMaxSize:  src.Spec.Kubelet.ContainerLogMaxSize,
			ContainerLogMaxFiles: src.Spec.Kubelet.ContainerLogMaxFiles,
		}
	}

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Nodes = src.Status.Nodes
	dst.Status.Instances = src.Status.Instances
	dst.Status.Desired = src.Status.Desired
	dst.Status.Min = src.Status.Min
	dst.Status.Max = src.Status.Max
	dst.Status.UpToDate = src.Status.UpToDate
	dst.Status.Standby = src.Status.Standby
	dst.Status.Error = src.Status.Error
	dst.Status.KubernetesVersion = src.Status.KubernetesVersion

	if src.Status.ConditionSummary != nil {
		dst.Status.ConditionSummary = &v1.ConditionSummary{
			StatusMessage: src.Status.ConditionSummary.StatusMessage,
			Ready:         src.Status.ConditionSummary.Ready,
		}
	}

	for _, mf := range src.Status.LastMachineFailures {
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
		dst.Status.LastMachineFailures = append(dst.Status.LastMachineFailures, failure)
	}

	return nil
}

// ConvertFrom converts the Hub version (v1) to this NodeGroup (v1alpha1)
func (dst *NodeGroup) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.NodeGroup)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Convert nodeType: CloudEphemeral -> Cloud, Static -> Static, CloudStatic/CloudPermanent -> Hybrid
	switch src.Spec.NodeType {
	case v1.NodeTypeCloudEphemeral:
		dst.Spec.NodeType = NodeTypeCloud
	case v1.NodeTypeStatic:
		dst.Spec.NodeType = NodeTypeStatic
	case v1.NodeTypeCloudStatic, v1.NodeTypeCloudPermanent:
		dst.Spec.NodeType = NodeTypeHybrid
	default:
		dst.Spec.NodeType = NodeType(src.Spec.NodeType)
	}

	// CRI
	if src.Spec.CRI != nil {
		// v1alpha1 doesn't support ContainerdV2, convert to Containerd
		criType := src.Spec.CRI.Type
		if criType == v1.CRITypeContainerdV2 {
			criType = v1.CRITypeContainerd
		}
		dst.Spec.CRI = &CRISpec{
			Type: CRIType(criType),
		}
		if src.Spec.CRI.Containerd != nil {
			dst.Spec.CRI.Containerd = &ContainerdSpec{
				MaxConcurrentDownloads: src.Spec.CRI.Containerd.MaxConcurrentDownloads,
			}
		}
		// Also handle ContainerdV2 settings
		if src.Spec.CRI.ContainerdV2 != nil {
			dst.Spec.CRI.Containerd = &ContainerdSpec{
				MaxConcurrentDownloads: src.Spec.CRI.ContainerdV2.MaxConcurrentDownloads,
			}
		}
		if src.Spec.CRI.Docker != nil {
			dst.Spec.Docker = &DockerSpec{
				MaxConcurrentDownloads: src.Spec.CRI.Docker.MaxConcurrentDownloads,
				Manage:                 src.Spec.CRI.Docker.Manage,
			}
		}
	}

	// CloudInstances
	if src.Spec.CloudInstances != nil {
		dst.Spec.CloudInstances = &CloudInstancesSpec{
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
		if src.Spec.CloudInstances.StandbyHolder != nil && src.Spec.CloudInstances.StandbyHolder.NotHeldResources != nil {
			dst.Spec.CloudInstances.StandbyHolder = &StandbyHolderSpec{
				NotHeldResources: &NotHeldResourcesSpec{
					CPU:    src.Spec.CloudInstances.StandbyHolder.NotHeldResources.CPU,
					Memory: src.Spec.CloudInstances.StandbyHolder.NotHeldResources.Memory,
				},
			}
		}
	}

	// NodeTemplate
	if src.Spec.NodeTemplate != nil {
		dst.Spec.NodeTemplate = &NodeTemplate{
			Labels:      src.Spec.NodeTemplate.Labels,
			Annotations: src.Spec.NodeTemplate.Annotations,
			Taints:      src.Spec.NodeTemplate.Taints,
		}
	}

	// Chaos
	if src.Spec.Chaos != nil {
		dst.Spec.Chaos = &ChaosSpec{
			Mode:   ChaosMode(src.Spec.Chaos.Mode),
			Period: src.Spec.Chaos.Period,
		}
	}

	// OperatingSystem
	if src.Spec.OperatingSystem != nil {
		dst.Spec.OperatingSystem = &OperatingSystemSpec{
			ManageKernel: src.Spec.OperatingSystem.ManageKernel,
		}
	}

	// Disruptions
	if src.Spec.Disruptions != nil {
		dst.Spec.Disruptions = &DisruptionsSpec{
			ApprovalMode: DisruptionApprovalMode(src.Spec.Disruptions.ApprovalMode),
		}
		if src.Spec.Disruptions.Automatic != nil {
			dst.Spec.Disruptions.Automatic = &AutomaticDisruptionSpec{
				DrainBeforeApproval: src.Spec.Disruptions.Automatic.DrainBeforeApproval,
			}
			for _, w := range src.Spec.Disruptions.Automatic.Windows {
				dst.Spec.Disruptions.Automatic.Windows = append(dst.Spec.Disruptions.Automatic.Windows,
					DisruptionWindow{
						From: w.From,
						To:   w.To,
						Days: w.Days,
					})
			}
		}
		if src.Spec.Disruptions.RollingUpdate != nil {
			dst.Spec.Disruptions.RollingUpdate = &RollingUpdateDisruptionSpec{}
			for _, w := range src.Spec.Disruptions.RollingUpdate.Windows {
				dst.Spec.Disruptions.RollingUpdate.Windows = append(dst.Spec.Disruptions.RollingUpdate.Windows,
					DisruptionWindow{
						From: w.From,
						To:   w.To,
						Days: w.Days,
					})
			}
		}
	}

	// Kubelet
	if src.Spec.Kubelet != nil {
		dst.Spec.Kubelet = &KubeletSpec{
			MaxPods:              src.Spec.Kubelet.MaxPods,
			RootDir:              src.Spec.Kubelet.RootDir,
			ContainerLogMaxSize:  src.Spec.Kubelet.ContainerLogMaxSize,
			ContainerLogMaxFiles: src.Spec.Kubelet.ContainerLogMaxFiles,
		}
	}

	// Status
	dst.Status.Ready = src.Status.Ready
	dst.Status.Nodes = src.Status.Nodes
	dst.Status.Instances = src.Status.Instances
	dst.Status.Desired = src.Status.Desired
	dst.Status.Min = src.Status.Min
	dst.Status.Max = src.Status.Max
	dst.Status.UpToDate = src.Status.UpToDate
	dst.Status.Standby = src.Status.Standby
	dst.Status.Error = src.Status.Error
	dst.Status.KubernetesVersion = src.Status.KubernetesVersion

	if src.Status.ConditionSummary != nil {
		dst.Status.ConditionSummary = &ConditionSummary{
			StatusMessage: src.Status.ConditionSummary.StatusMessage,
			Ready:         src.Status.ConditionSummary.Ready,
		}
	}

	for _, mf := range src.Status.LastMachineFailures {
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
		dst.Status.LastMachineFailures = append(dst.Status.LastMachineFailures, failure)
	}

	return nil
}
