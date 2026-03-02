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
	"k8s.io/apimachinery/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

var conversionlog = logf.Log.WithName("nodegroup-conversion")

// ConvertV1alpha1NodeGroupSpecToV1NodeGroupSpec handles nodeType mapping
// v1alpha1: Cloud, Static, Hybrid
// v1: CloudEphemeral, CloudPermanent, CloudStatic, Static
func ConvertV1alpha1NodeGroupSpecToV1NodeGroupSpec(in *NodeGroupSpec, out *v1.NodeGroupSpec, s conversion.Scope) error {
	conversionlog.V(1).Info("converting NodeGroupSpec from v1alpha1 to v1", "nodeType", in.NodeType)

	// Map nodeType
	switch in.NodeType {
	case NodeTypeCloud:
		out.NodeType = v1.NodeTypeCloudEphemeral
	case NodeTypeStatic:
		out.NodeType = v1.NodeTypeStatic
	case NodeTypeHybrid:
		out.NodeType = v1.NodeTypeCloudStatic
	default:
		out.NodeType = v1.NodeType(in.NodeType)
	}

	// Convert CRI (handle Docker field from v1alpha1)
	if in.CRI != nil || in.Docker != nil {
		out.CRI = &v1.CRISpec{}

		if in.CRI != nil {
			out.CRI.Type = v1.CRIType(in.CRI.Type)

			if in.CRI.Containerd != nil {
				out.CRI.Containerd = &v1.ContainerdSpec{
					MaxConcurrentDownloads: in.CRI.Containerd.MaxConcurrentDownloads,
				}
			}
		}

		// Docker field in v1alpha1 maps to CRI.Docker in v1
		if in.Docker != nil {
			if out.CRI.Type == "" {
				out.CRI.Type = v1.CRITypeDocker
			}
			out.CRI.Docker = &v1.DockerSpec{
				MaxConcurrentDownloads: in.Docker.MaxConcurrentDownloads,
				Manage:                 in.Docker.Manage,
			}
		}
	}

	// Call auto-generated conversion for remaining fields
	return autoConvertV1alpha1NodeGroupSpecToV1NodeGroupSpec(in, out)
}

// ConvertV1NodeGroupSpecToV1alpha1NodeGroupSpec handles nodeType mapping (reverse)
func ConvertV1NodeGroupSpecToV1alpha1NodeGroupSpec(in *v1.NodeGroupSpec, out *NodeGroupSpec, s conversion.Scope) error {
	conversionlog.V(1).Info("converting NodeGroupSpec from v1 to v1alpha1", "nodeType", in.NodeType)

	// Map nodeType (reverse)
	switch in.NodeType {
	case v1.NodeTypeCloudEphemeral:
		out.NodeType = NodeTypeCloud
	case v1.NodeTypeStatic:
		out.NodeType = NodeTypeStatic
	case v1.NodeTypeCloudStatic, v1.NodeTypeCloudPermanent:
		out.NodeType = NodeTypeHybrid
	default:
		out.NodeType = NodeType(in.NodeType)
	}

	// Convert CRI (extract Docker field for v1alpha1)
	if in.CRI != nil {
		criType := in.CRI.Type

		// ContainerdV2 downgrades to Containerd in v1alpha1
		if criType == v1.CRITypeContainerdV2 {
			criType = v1.CRITypeContainerd
		}

		out.CRI = &CRISpec{
			Type: CRIType(criType),
		}

		if in.CRI.Containerd != nil {
			out.CRI.Containerd = &ContainerdSpec{
				MaxConcurrentDownloads: in.CRI.Containerd.MaxConcurrentDownloads,
			}
		}

		// ContainerdV2 settings also go to Containerd in v1alpha1
		if in.CRI.ContainerdV2 != nil && out.CRI.Containerd == nil {
			out.CRI.Containerd = &ContainerdSpec{
				MaxConcurrentDownloads: in.CRI.ContainerdV2.MaxConcurrentDownloads,
			}
		}

		// Extract Docker to separate field in v1alpha1
		if in.CRI.Docker != nil {
			out.Docker = &DockerSpec{
				MaxConcurrentDownloads: in.CRI.Docker.MaxConcurrentDownloads,
				Manage:                 in.CRI.Docker.Manage,
			}
		}
	}

	// Call auto-generated conversion for remaining fields
	return autoConvertV1NodeGroupSpecToV1alpha1NodeGroupSpec(in, out)
}

// Stub functions that will be replaced by auto-generated ones
// These are needed for compilation before running conversion-gen

func autoConvertV1alpha1NodeGroupSpecToV1NodeGroupSpec(in *NodeGroupSpec, out *v1.NodeGroupSpec) error {
	// CloudInstances
	if in.CloudInstances != nil {
		out.CloudInstances = &v1.CloudInstancesSpec{
			Zones:                 in.CloudInstances.Zones,
			MinPerZone:            in.CloudInstances.MinPerZone,
			MaxPerZone:            in.CloudInstances.MaxPerZone,
			MaxUnavailablePerZone: in.CloudInstances.MaxUnavailablePerZone,
			MaxSurgePerZone:       in.CloudInstances.MaxSurgePerZone,
			Standby:               in.CloudInstances.Standby,
			ClassReference: v1.ClassReference{
				Kind: in.CloudInstances.ClassReference.Kind,
				Name: in.CloudInstances.ClassReference.Name,
			},
		}
		if in.CloudInstances.StandbyHolder != nil {
			out.CloudInstances.StandbyHolder = &v1.StandbyHolderSpec{}
			if in.CloudInstances.StandbyHolder.NotHeldResources != nil {
				out.CloudInstances.StandbyHolder.NotHeldResources = &v1.NotHeldResourcesSpec{
					CPU:    in.CloudInstances.StandbyHolder.NotHeldResources.CPU,
					Memory: in.CloudInstances.StandbyHolder.NotHeldResources.Memory,
				}
			}
		}
	}

	// NodeTemplate
	if in.NodeTemplate != nil {
		out.NodeTemplate = &v1.NodeTemplate{
			Labels:      in.NodeTemplate.Labels,
			Annotations: in.NodeTemplate.Annotations,
			Taints:      in.NodeTemplate.Taints,
		}
	}

	// Chaos
	if in.Chaos != nil {
		out.Chaos = &v1.ChaosSpec{
			Mode:   v1.ChaosMode(in.Chaos.Mode),
			Period: in.Chaos.Period,
		}
	}

	// OperatingSystem
	if in.OperatingSystem != nil {
		out.OperatingSystem = &v1.OperatingSystemSpec{
			ManageKernel: in.OperatingSystem.ManageKernel,
		}
	}

	// Disruptions
	if in.Disruptions != nil {
		out.Disruptions = &v1.DisruptionsSpec{
			ApprovalMode: v1.DisruptionApprovalMode(in.Disruptions.ApprovalMode),
		}
		if in.Disruptions.Automatic != nil {
			out.Disruptions.Automatic = &v1.AutomaticDisruptionSpec{
				DrainBeforeApproval: in.Disruptions.Automatic.DrainBeforeApproval,
			}
			for _, w := range in.Disruptions.Automatic.Windows {
				out.Disruptions.Automatic.Windows = append(out.Disruptions.Automatic.Windows,
					v1.DisruptionWindow{From: w.From, To: w.To, Days: w.Days})
			}
		}
		if in.Disruptions.RollingUpdate != nil {
			out.Disruptions.RollingUpdate = &v1.RollingUpdateDisruptionSpec{}
			for _, w := range in.Disruptions.RollingUpdate.Windows {
				out.Disruptions.RollingUpdate.Windows = append(out.Disruptions.RollingUpdate.Windows,
					v1.DisruptionWindow{From: w.From, To: w.To, Days: w.Days})
			}
		}
	}

	// Kubelet
	if in.Kubelet != nil {
		out.Kubelet = &v1.KubeletSpec{
			MaxPods:              in.Kubelet.MaxPods,
			RootDir:              in.Kubelet.RootDir,
			ContainerLogMaxSize:  in.Kubelet.ContainerLogMaxSize,
			ContainerLogMaxFiles: in.Kubelet.ContainerLogMaxFiles,
		}
	}

	// Note: Static field from v1alpha1 has no equivalent in v1 (lost on conversion)
	// Note: KubernetesVersion from v1alpha1 has no equivalent in v1.Spec (lost on conversion)

	return nil
}

func autoConvertV1NodeGroupSpecToV1alpha1NodeGroupSpec(in *v1.NodeGroupSpec, out *NodeGroupSpec) error {
	// CloudInstances
	if in.CloudInstances != nil {
		out.CloudInstances = &CloudInstancesSpec{
			Zones:                 in.CloudInstances.Zones,
			MinPerZone:            in.CloudInstances.MinPerZone,
			MaxPerZone:            in.CloudInstances.MaxPerZone,
			MaxUnavailablePerZone: in.CloudInstances.MaxUnavailablePerZone,
			MaxSurgePerZone:       in.CloudInstances.MaxSurgePerZone,
			Standby:               in.CloudInstances.Standby,
			ClassReference: ClassReference{
				Kind: in.CloudInstances.ClassReference.Kind,
				Name: in.CloudInstances.ClassReference.Name,
			},
		}
		if in.CloudInstances.StandbyHolder != nil {
			out.CloudInstances.StandbyHolder = &StandbyHolderSpec{}
			if in.CloudInstances.StandbyHolder.NotHeldResources != nil {
				out.CloudInstances.StandbyHolder.NotHeldResources = &NotHeldResourcesSpec{
					CPU:    in.CloudInstances.StandbyHolder.NotHeldResources.CPU,
					Memory: in.CloudInstances.StandbyHolder.NotHeldResources.Memory,
				}
			}
		}
	}

	// NodeTemplate
	if in.NodeTemplate != nil {
		out.NodeTemplate = &NodeTemplate{
			Labels:      in.NodeTemplate.Labels,
			Annotations: in.NodeTemplate.Annotations,
			Taints:      in.NodeTemplate.Taints,
		}
	}

	// Chaos
	if in.Chaos != nil {
		out.Chaos = &ChaosSpec{
			Mode:   ChaosMode(in.Chaos.Mode),
			Period: in.Chaos.Period,
		}
	}

	// OperatingSystem
	if in.OperatingSystem != nil {
		out.OperatingSystem = &OperatingSystemSpec{
			ManageKernel: in.OperatingSystem.ManageKernel,
		}
	}

	// Disruptions
	if in.Disruptions != nil {
		out.Disruptions = &DisruptionsSpec{
			ApprovalMode: DisruptionApprovalMode(in.Disruptions.ApprovalMode),
		}
		if in.Disruptions.Automatic != nil {
			out.Disruptions.Automatic = &AutomaticDisruptionSpec{
				DrainBeforeApproval: in.Disruptions.Automatic.DrainBeforeApproval,
			}
			for _, w := range in.Disruptions.Automatic.Windows {
				out.Disruptions.Automatic.Windows = append(out.Disruptions.Automatic.Windows,
					DisruptionWindow{From: w.From, To: w.To, Days: w.Days})
			}
		}
		if in.Disruptions.RollingUpdate != nil {
			out.Disruptions.RollingUpdate = &RollingUpdateDisruptionSpec{}
			for _, w := range in.Disruptions.RollingUpdate.Windows {
				out.Disruptions.RollingUpdate.Windows = append(out.Disruptions.RollingUpdate.Windows,
					DisruptionWindow{From: w.From, To: w.To, Days: w.Days})
			}
		}
	}

	// Kubelet
	if in.Kubelet != nil {
		out.Kubelet = &KubeletSpec{
			MaxPods:              in.Kubelet.MaxPods,
			RootDir:              in.Kubelet.RootDir,
			ContainerLogMaxSize:  in.Kubelet.ContainerLogMaxSize,
			ContainerLogMaxFiles: in.Kubelet.ContainerLogMaxFiles,
		}
	}

	// Note: v1 fields not in v1alpha1 are lost:
	// - StaticInstances
	// - Update
	// - Fencing
	// - GPU
	// - NodeDrainTimeoutSecond
	// - Kubelet.ResourceReservation
	// - Kubelet.TopologyManager
	// - Kubelet.MemorySwap

	return nil
}
