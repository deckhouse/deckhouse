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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"github.com/deckhouse/deckhouse/modules/490-virtualization/hooks/internal/v1alpha1"
)

const (
	disksSnapshot            = "diskHandlerVirtualMachineDisk"
	clusterImagesSnapshot    = "diskHandlerClusterVirtualMachineImage"
	dataVolumesSnapshot      = "diskHandlerDataVolume"
	pvcsSnapshot             = "diskHandlerPVC"
	cdiDataVolumeCRDSnapshot = "diskHandlerCDIDataVolumeCRD"
)

var diskHandlerHookConfig = &go_hook.HookConfig{
	Queue: "/modules/virtualization/disk-handler",
	Kubernetes: []go_hook.KubernetesConfig{
		// A binding with dynamic kind has index 0 for simplicity.
		{
			Name:       dataVolumesSnapshot,
			ApiVersion: "",
			Kind:       "",
			FilterFunc: applyDataVolumeFilter,
		},
		{
			Name:       pvcsSnapshot,
			ApiVersion: "v1",
			Kind:       "PersistentVolumeClaim",
			FilterFunc: applyPVCFilter,
		},
		{
			Name:       disksSnapshot,
			ApiVersion: gv,
			Kind:       "VirtualMachineDisk",
			FilterFunc: applyVirtualMachineDiskFilter,
		},
		{
			Name:       clusterImagesSnapshot,
			ApiVersion: gv,
			Kind:       "ClusterVirtualMachineImage",
			FilterFunc: applyClusterVirtualMachineImageFilter,
		},
		{
			Name:       cdiDataVolumeCRDSnapshot,
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"datavolumes.cdi.kubevirt.io"},
			},
			FilterFunc: applyCRDExistenseFilter,
		},
	},
}

var _ = sdk.RegisterFunc(diskHandlerHookConfig, handleVirtualMachineDisks)

type VirtualMachineDiskSnapshot struct {
	Name             string
	Namespace        string
	UID              ktypes.UID
	StorageClassName string
	Size             resource.Quantity
	Source           *corev1.TypedLocalObjectReference
	VMName           string
	Ephemeral        bool
}

type ClusterVirtualMachineImageSnapshot struct {
	Name             string
	Namespace        string
	UID              ktypes.UID
	StorageClassName string
	Size             resource.Quantity
	Remote           *cdiv1.DataVolumeSource
	Source           *v1alpha1.TypedObjectReference
}

type DataVolumeSnapshot struct {
	Name      string
	Namespace string
}

type PVCSnapshot struct {
	Name      string
	Namespace string
	Size      resource.Quantity
}

func applyVirtualMachineDiskFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	disk := &v1alpha1.VirtualMachineDisk{}
	err := sdk.FromUnstructured(obj, disk)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to VirtualMachineDisk: %v", err)
	}

	return &VirtualMachineDiskSnapshot{
		Name:             disk.Name,
		Namespace:        disk.Namespace,
		UID:              disk.UID,
		StorageClassName: disk.Spec.StorageClassName,
		Size:             disk.Spec.Size,
		Source:           disk.Spec.Source,
		Ephemeral:        disk.Status.Ephemeral,
	}, nil
}

func applyClusterVirtualMachineImageFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	clusterImage := &v1alpha1.ClusterVirtualMachineImage{}
	err := sdk.FromUnstructured(obj, clusterImage)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to DataVolume: %v", err)
	}

	return &ClusterVirtualMachineImageSnapshot{
		Name:      clusterImage.Name,
		Namespace: clusterImage.Namespace,
		UID:       clusterImage.UID,
		Source:    clusterImage.Spec.Source,
		Remote:    reducedDataVolumeSource2cdiDataVolumeSource(&clusterImage.Spec.Remote),
	}, nil
}

func applyDataVolumeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	volume := &cdiv1.DataVolume{}
	err := sdk.FromUnstructured(obj, volume)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to DataVolume: %v", err)
	}

	return &DataVolumeSnapshot{
		Name:      volume.Name,
		Namespace: volume.Namespace,
	}, nil
}

func applyPVCFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := sdk.FromUnstructured(obj, pvc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to PVC: %v", err)
	}

	var size resource.Quantity
	if s := pvc.Spec.Resources.Requests.Storage(); s != nil {
		size = *s
	}

	return &PVCSnapshot{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
		Size:      size,
	}, nil
}

// handleVirtualMachineDisks
//
// synopsis:
//   This hook converts Deckhouse VirtualMachineDisks (top-level abstraction) to CDI DataVolumes.
//   Every Deckhouse VirtualMachineDisk represents DataVolume with specified data source.

func handleVirtualMachineDisks(input *go_hook.HookInput) error {
	// CDI manages it's own CRDs, so we need to wait for them before starting the watch
	if diskHandlerHookConfig.Kubernetes[0].Kind == "" {
		if len(input.Snapshots[cdiDataVolumeCRDSnapshot]) > 0 {
			// CDI installed
			input.LogEntry.Infof("CDI DataVolume CRD installed, update kind for binding datavolumes.cdi.kubevirt.io")
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name:       dataVolumesSnapshot,
				Action:     "UpdateKind",
				ApiVersion: "cdi.kubevirt.io/v1beta1",
				Kind:       "DataVolume",
			})
			// Save new kind as current kind.
			diskHandlerHookConfig.Kubernetes[0].Kind = "DataVolume"
			diskHandlerHookConfig.Kubernetes[0].ApiVersion = "cdi.kubevirt.io/v1beta1"
			// Binding changed, hook will be restarted with new objects in snapshot.
			return nil
		}
		// CDI is not yet installed, do nothing
		return nil
	}

	// Start main hook logic
	diskSnap := input.Snapshots[disksSnapshot]
	clusterImageSnap := input.Snapshots[clusterImagesSnapshot]
	dataVolumeSnap := input.Snapshots[dataVolumesSnapshot]
	pvcSnap := input.Snapshots[pvcsSnapshot]

	if len(diskSnap) == 0 {
		input.LogEntry.Warnln("VirtualMachineDisk not found. Skip")
		return nil
	}

	for _, sRaw := range diskSnap {
		disk := sRaw.(*VirtualMachineDiskSnapshot)
		if getDataVolume(&dataVolumeSnap, disk.Namespace, "disk-"+disk.Name) != nil {
			// DataVolume found, check and resize PVC
			if pvc := getPVC(&pvcSnap, disk.Namespace, "disk-"+disk.Name); pvc != nil {
				if disk.Size.CmpInt64(pvc.Size.Value()) == 1 {
					patch := map[string]interface{}{"spec": map[string]interface{}{"resources": map[string]interface{}{"requests": corev1.ResourceList{
						corev1.ResourceStorage: disk.Size,
					}}}}
					input.PatchCollector.MergePatch(patch, "v1", "PersistentVolumeClaim", disk.Namespace, "disk-"+disk.Name)
				}
			}
			continue
		}

		// DataVolume not found, needs to create a new one

		source := &v1alpha1.TypedObjectReference{}
		source.APIGroup = disk.Source.APIGroup
		source.Kind = disk.Source.Kind
		source.Name = disk.Source.Name

		dataVolumeSource, err := resolveDataVolumeSource(&diskSnap, &clusterImageSnap, source)
		if err != nil {
			input.LogEntry.Warnf("%s. Skip", err)
			return nil
		}

		storage := &cdiv1.StorageSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: disk.Size,
				},
			},
		}
		if disk.StorageClassName != "" {
			storage.StorageClassName = &disk.StorageClassName
		}

		dataVolume := &cdiv1.DataVolume{
			TypeMeta: metav1.TypeMeta{
				Kind:       "DataVolume",
				APIVersion: "cdi.kubevirt.io/v1beta1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      "disk-" + disk.Name,
				Namespace: disk.Namespace,
				OwnerReferences: []v1.OwnerReference{{
					APIVersion:         gv,
					BlockOwnerDeletion: pointer.Bool(true),
					Controller:         pointer.Bool(true),
					Kind:               "VirtualMachineDisk",
					Name:               disk.Name,
					UID:                disk.UID,
				}},
			},
			Spec: cdiv1.DataVolumeSpec{
				Source:  dataVolumeSource,
				Storage: storage,
			},
		}
		input.PatchCollector.Create(dataVolume)
	}

	return nil
}

func resolveDataVolumeSource(diskSnap, clusterImageSnap *[]go_hook.FilterResult, source *v1alpha1.TypedObjectReference) (*cdiv1.DataVolumeSource, error) {
	if source == nil {
		return &cdiv1.DataVolumeSource{Blank: &cdiv1.DataVolumeBlankImage{}}, nil
	}
	switch source.Kind {
	case "VirtualMachineDisk":
		disk := getDisk(diskSnap, source.Namespace, source.Name)
		if disk == nil {
			return nil, fmt.Errorf("VirtualMachineDisk not found")
		}
		return &cdiv1.DataVolumeSource{PVC: &cdiv1.DataVolumeSourcePVC{Namespace: disk.Namespace, Name: "disk-" + disk.Name}}, nil
	case "ClusterVirtualMachineImage":
		clusterImage := getClusterImage(clusterImageSnap, source.Name)
		if clusterImage == nil {
			return nil, fmt.Errorf("ClusterVirtualMachineImage not found")
		}
		if clusterImage.Remote != nil {
			return clusterImage.Remote, nil
		}
		if clusterImage.Source != nil {
			return resolveDataVolumeSource(diskSnap, clusterImageSnap, clusterImage.Source)
		}
		return nil, fmt.Errorf("Neither source and remote specified")
	case "VirtualMachineImage":
		// TODO handle namespaced VirtualMachineImage
		return nil, fmt.Errorf("Not implemented")
	}
	return nil, fmt.Errorf("Unknown type of source")
}
