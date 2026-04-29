/*
Copyright 2026 Flant JSC

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

package nodegroupfilter

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	nm "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/pkg/schema"
)

type NodeType string

type NodeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NodeGroupSpec `json:"spec"`
}

type NodeGroupSpec struct {
	NodeType NodeType `json:"nodeType,omitempty"`

	NodeDrainTimeoutSecond *int64 `json:"nodeDrainTimeoutSecond,omitempty"`

	CRI CRI `json:"cri,omitempty"`

	GPU GPU `json:"gpu,omitempty"`

	StaticInstances *StaticInstances `json:"staticInstances,omitempty"`

	CloudInstances CloudInstances `json:"cloudInstances,omitempty"`

	NodeTemplate nm.NodeTemplate `json:"nodeTemplate,omitempty"`

	Chaos Chaos `json:"chaos,omitempty"`

	OperatingSystem OperatingSystem `json:"operatingSystem,omitempty"`

	Disruptions Disruptions `json:"disruptions,omitempty"`

	Update Update `json:"update,omitempty"`

	Kubelet Kubelet `json:"kubelet,omitempty"`
}

type GPU struct {
	Sharing string `json:"sharing,omitempty"`

	TimeSlicing *TimeSlicing `json:"timeSlicing,omitempty"`

	Mig *Mig `json:"mig,omitempty"`
}

type CRI struct {
	Type string `json:"type,omitempty"`

	Containerd *Containerd `json:"containerd,omitempty"`

	ContainerdV2 *Containerd `json:"containerdV2,omitempty"`

	Docker *Docker `json:"docker,omitempty"`

	NotManaged *NotManaged `json:"notManaged,omitempty"`
}

type TimeSlicing struct {
	GpuPartitionCount *int32 `json:"partitionCount,omitempty"`
}

type Mig struct {
	PartedConfig *string `json:"partedConfig,omitempty"`

	CustomConfigs []MigCustomConfig `json:"customConfigs,omitempty"`
}

type MigCustomConfig struct {
	Index  int32      `json:"index"`
	Slices []MigSlice `json:"slices,omitempty"`
}

type MigSlice struct {
	Profile string `json:"profile,omitempty"`
	Count   *int32 `json:"count,omitempty"`
}

type Containerd struct {
	MaxConcurrentDownloads *int32 `json:"maxConcurrentDownloads,omitempty"`
}

type Docker struct {
	MaxConcurrentDownloads *int32 `json:"maxConcurrentDownloads,omitempty"`
	Manage                 *bool  `json:"manage,omitempty"`
}

type NotManaged struct {
	CriSocketPath *string `json:"criSocketPath,omitempty"`
}

type StaticInstances struct {
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	Count         int32                 `json:"count"`
}

type CloudInstances struct {
	QuickShutdown *bool `json:"quickShutdown,omitempty"`

	Zones []string `json:"zones"`

	MinPerZone *int32 `json:"minPerZone,omitempty"`

	MaxPerZone *int32 `json:"maxPerZone,omitempty"`

	MaxUnavailablePerZone *int32 `json:"maxUnavailablePerZone,omitempty"`

	MaxSurgePerZone *int32 `json:"maxSurgePerZone,omitempty"`

	Standby *intstr.IntOrString `json:"standby,omitempty"`

	StandbyHolder StandbyHolder `json:"standbyHolder,omitempty"`

	ClassReference ClassReference `json:"classReference"`

	Priority *int32 `json:"priority,omitempty"`
}

type StandbyHolder struct {
	OverprovisioningRate *int64    `json:"overprovisioningRate,omitempty"`
	NotHeldResources     Resources `json:"notHeldResources,omitempty"`
}

type Resources struct {
	CPU    intstr.IntOrString `json:"cpu,omitempty"`
	Memory intstr.IntOrString `json:"memory,omitempty"`
}

type ClassReference struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
}

type Chaos struct {
	Mode   string `json:"mode,omitempty"`
	Period string `json:"period,omitempty"`
}

type OperatingSystem struct {
	ManageKernel *bool `json:"manageKernel,omitempty"`
}

type Disruptions struct {
	ApprovalMode string `json:"approvalMode"`

	Automatic     AutomaticDisruptions     `json:"automatic,omitempty"`
	RollingUpdate RollingUpdateDisruptions `json:"rollingUpdate,omitempty"`
}

type Update struct {
	MaxConcurrent *intstr.IntOrString `json:"maxConcurrent,omitempty"`
}

type AutomaticDisruptions struct {
	DrainBeforeApproval *bool          `json:"drainBeforeApproval,omitempty"`
	Windows             update.Windows `json:"windows,omitempty"`
}

type RollingUpdateDisruptions struct {
	Windows update.Windows `json:"windows,omitempty"`
}

type Kubelet struct {
	MaxPods *int32 `json:"maxPods,omitempty"`

	RootDir string `json:"rootDir,omitempty"`

	ContainerLogMaxSize string `json:"containerLogMaxSize,omitempty"`

	ContainerLogMaxFiles int `json:"containerLogMaxFiles,omitempty"`

	ResourceReservation KubeletResourceReservation `json:"resourceReservation"`

	TopologyManager KubeletTopologyManager `json:"topologyManager"`

	MemorySwap *KubeletMemorySwap `json:"memorySwap,omitempty"`
}

type KubeletTopologyManager struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Scope   string `json:"scope,omitempty"`
	Policy  string `json:"policy,omitempty"`
}

type KubeletResourceReservation struct {
	Mode   string                            `json:"mode"`
	Static *KubeletStaticResourceReservation `json:"static,omitempty"`
}

type KubeletStaticResourceReservation struct {
	CPU              resource.Quantity `json:"cpu,omitempty"`
	Memory           resource.Quantity `json:"memory,omitempty"`
	EphemeralStorage resource.Quantity `json:"ephemeralStorage,omitempty"`
}

type KubeletMemorySwap struct {
	SwapBehavior string              `json:"swapBehavior"`
	LimitedSwap  *KubeletLimitedSwap `json:"limitedSwap,omitempty"`
	Swappiness   *int                `json:"swappiness,omitempty"`
}

type KubeletLimitedSwap struct {
	Size string `json:"size"`
}
