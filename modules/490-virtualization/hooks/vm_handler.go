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
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	virtv1 "kubevirt.io/api/core/v1"

	"github.com/deckhouse/deckhouse/modules/490-virtualization/hooks/internal/v1alpha1"
)

const (
	deckhouseVMSnapshot    = "vmHandlerDeckhouseVM"
	kubevirtVMSnapshot     = "vmHandlerKubevirtVM"
	kubevirtVMsCRDSnapshot = "vmHandlerKubevirtVMCRD"
	vmDisksSnapshot        = "vmDisksSnapshot"
)

var vmHandlerHookConfig = &go_hook.HookConfig{
	Queue: "/modules/virtualization/vm-handler",
	Kubernetes: []go_hook.KubernetesConfig{
		// A binding with dynamic kind has index 0 for simplicity.
		{
			Name:       kubevirtVMSnapshot,
			ApiVersion: "",
			Kind:       "",
			FilterFunc: applyKubevirtVMFilter,
		},
		{
			Name:       deckhouseVMSnapshot,
			ApiVersion: gv,
			Kind:       "VirtualMachine",
			FilterFunc: applyDeckhouseVMFilter,
		},
		{
			Name:       ipClaimsSnapshot,
			ApiVersion: gv,
			Kind:       "VirtualMachineIPAddressClaim",
			FilterFunc: applyVirtualMachineIPAddressClaimFilter,
		},
		{
			Name:       vmDisksSnapshot,
			ApiVersion: gv,
			Kind:       "VirtualMachineDisk",
			FilterFunc: applyVirtualMachineDisksFilter,
		},
		{
			Name:       kubevirtVMsCRDSnapshot,
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"virtualmachines.kubevirt.io"},
			},
			FilterFunc: applyCRDExistenseFilter,
		},
	},
}

var _ = sdk.RegisterFunc(vmHandlerHookConfig, handleVMs)

type InstanceClassCrdInfo struct {
	Name string
	Spec interface{}
}

func applyKubevirtVMFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	vm := &virtv1.VirtualMachine{}
	err := sdk.FromUnstructured(obj, vm)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to VirtualMachine: %v", err)
	}

	return vm, nil
}

func applyDeckhouseVMFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	vm := &v1alpha1.VirtualMachine{}
	err := sdk.FromUnstructured(obj, vm)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to VirtualMachine: %v", err)
	}

	return vm, nil
}

func applyVirtualMachineDisksFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	disk := &v1alpha1.VirtualMachineDisk{}
	err := sdk.FromUnstructured(obj, disk)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to VirtualMachineDisk: %v", err)
	}

	return &VirtualMachineDiskSnapshot{
		Name:      disk.Name,
		Namespace: disk.Namespace,
		VMName:    disk.Status.VMName,
		Ephemeral: disk.Status.Ephemeral,
		Size:      disk.Spec.Size,
	}, nil
}

// handleVMs
//
// synopsis:
//
//	This hook converts Deckhouse VirtualMachines (top-level abstraction) to KubeVirt VirtualMachines.
//	Every Deckhouse VirtualMachine represents KubeVirt VirtualMachne with attached DataVolumes.
//	For the boot disk this hook creates VirtualMachineDisk to provision DataVolume out of specific image source.
func handleVMs(input *go_hook.HookInput) error {
	// KubeVirt manages it's own CRDs, so we need to wait for them before starting the watch
	if vmHandlerHookConfig.Kubernetes[0].Kind == "" {
		if len(input.Snapshots[kubevirtVMsCRDSnapshot]) > 0 {
			// KubeVirt installed
			input.LogEntry.Infof("KubeVirt VirtualMachine CRD installed, update kind for binding VirtualMachines.kubevirt.io")
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name:       kubevirtVMSnapshot,
				Action:     "UpdateKind",
				ApiVersion: "kubevirt.io/v1",
				Kind:       "VirtualMachine",
			})
			// Save new kind as current kind.
			vmHandlerHookConfig.Kubernetes[0].Kind = "VirtualMachine"
			vmHandlerHookConfig.Kubernetes[0].ApiVersion = "kubevirt.io/v1"
			// Binding changed, hook will be restarted with new objects in snapshot.
			return nil
		}
		// KubeVirt is not yet installed, do nothing
		return nil
	}

	// Start main hook logic
	kubevirtVMSnap := input.Snapshots[kubevirtVMSnapshot]
	deckhouseVMSnap := input.Snapshots[deckhouseVMSnapshot]
	ipClaimSnap := input.Snapshots[ipClaimsSnapshot]
	diskSnap := input.Snapshots[vmDisksSnapshot]

	if len(kubevirtVMSnap) == 0 && len(deckhouseVMSnap) == 0 && len(diskSnap) == 0 {
		input.LogEntry.Warnln("VirtualMachines and VirtualMachineDisks not found. Skip")
		return nil
	}

	for _, sRaw := range deckhouseVMSnap {
		d8vm := sRaw.(*v1alpha1.VirtualMachine)
		if err := processD8VM(input, d8vm); err != nil {
			return err
		}
	}

	// Disks cleanup loop
	for _, sRaw := range diskSnap {
		disk := sRaw.(*VirtualMachineDiskSnapshot)
		checkAndCleanupDisk(input, disk)
	}

	// IPAddressClaims cleanup loop
	for _, sRaw := range ipClaimSnap {
		ipClaim := sRaw.(*VirtualMachineIPAddressClaimSnapshot)
		checkAndCleanupIPAddressClaim(input, ipClaim)
	}

	return nil
}

func setVMFields(d8vm *v1alpha1.VirtualMachine, vm *virtv1.VirtualMachine, ipAddress string) error {
	vm.TypeMeta = metav1.TypeMeta{
		Kind:       "VirtualMachine",
		APIVersion: "kubevirt.io/v1",
	}
	vm.SetName(d8vm.Name)
	vm.SetNamespace(d8vm.Namespace)
	vm.SetOwnerReferences([]v1.OwnerReference{{
		APIVersion:         gv,
		BlockOwnerDeletion: pointer.Bool(true),
		Controller:         pointer.Bool(true),
		Kind:               "VirtualMachine",
		Name:               d8vm.Name,
		UID:                d8vm.UID,
	}})

	cloudInit := make(map[string]interface{})
	if d8vm.Spec.CloudInit != nil {
		err := yaml.Unmarshal([]byte(d8vm.Spec.CloudInit.UserData), &cloudInit)
		if err != nil {
			return fmt.Errorf("cannot parse cloudInit config for VirtualMachine: %v", err)
		}
	}
	if d8vm.Spec.SSHPublicKey != nil {
		cloudInit["ssh_authorized_keys"] = []string{*d8vm.Spec.SSHPublicKey}
	}
	if d8vm.Spec.UserName != nil {
		cloudInit["user"] = *d8vm.Spec.UserName
	}
	cloudInitRaw, _ := yaml.Marshal(cloudInit)

	annotations := d8vm.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["cni.cilium.io/ipAddress"] = ipAddress
	annotations["cni.cilium.io/macAddress"] = "f6:e1:74:94:b8:1a"

	vm.Spec.Running = d8vm.Spec.Running
	vm.Spec.Template = &virtv1.VirtualMachineInstanceTemplateSpec{
		ObjectMeta: v1.ObjectMeta{
			Labels:      d8vm.Labels,
			Annotations: annotations,
		},
		Spec: virtv1.VirtualMachineInstanceSpec{
			Domain: virtv1.DomainSpec{
				Devices: virtv1.Devices{
					Interfaces: []virtv1.Interface{{
						Name:  "default",
						Model: "virtio",
						InterfaceBindingMethod: virtv1.InterfaceBindingMethod{
							Macvtap: &virtv1.InterfaceMacvtap{},
						},
					}},
				},
				Resources: virtv1.ResourceRequirements{
					Requests: d8vm.Spec.Resources,
				},
			},
			Networks: []virtv1.Network{{
				Name: "default",
				NetworkSource: virtv1.NetworkSource{
					Pod: &virtv1.PodNetwork{},
				}}},
			NodeSelector: d8vm.Spec.NodeSelector,
			Tolerations:  d8vm.Spec.Tolerations,
		},
	}

	// attach boot disk
	if d8vm.Spec.BootDisk != nil {
		bootVirtualMachineDiskName := d8vm.Spec.BootDisk.Name
		if bootVirtualMachineDiskName == "" {
			bootVirtualMachineDiskName = d8vm.Name + "-boot"
		}
		vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, virtv1.Disk{
			Name: "boot",
			DiskDevice: virtv1.DiskDevice{
				Disk: &virtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		})
		vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, virtv1.Volume{
			Name: "boot",
			VolumeSource: virtv1.VolumeSource{
				DataVolume: &virtv1.DataVolumeSource{
					Name:         "disk-" + bootVirtualMachineDiskName,
					Hotpluggable: false,
				},
			},
		})
	}

	// attach cloud-init
	if len(cloudInit) != 0 {
		vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, virtv1.Disk{
			Name: "cloudinit",
			DiskDevice: virtv1.DiskDevice{
				Disk: &virtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		})
		vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, virtv1.Volume{
			Name: "cloudinit",
			VolumeSource: virtv1.VolumeSource{
				CloudInitNoCloud: &virtv1.CloudInitNoCloudSource{
					// TODO handle cloudinit from secret
					UserData: fmt.Sprintf("#cloud-config\n%s", cloudInitRaw),
				},
			},
		})
	}

	// attach extra disks
	if d8vm.Spec.DiskAttachments != nil {
		for i, disk := range *d8vm.Spec.DiskAttachments {
			diskName := "disk-" + strconv.Itoa(i+1)
			vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, virtv1.Disk{
				Name: diskName,
				DiskDevice: virtv1.DiskDevice{
					Disk: &virtv1.DiskTarget{
						Bus: disk.Bus,
					},
				},
			})
			vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, virtv1.Volume{
				Name: diskName,
				VolumeSource: virtv1.VolumeSource{
					DataVolume: &virtv1.DataVolumeSource{
						Name:         "disk-" + disk.Name,
						Hotpluggable: disk.Hotpluggable,
					},
				},
			})
		}
	}
	return nil
}

func processD8VM(input *go_hook.HookInput, d8vm *v1alpha1.VirtualMachine) error {
	diskSnap := input.Snapshots[vmDisksSnapshot]

	// Handle boot disk
	if err := processVMBootDisk(input, d8vm); err != nil {
		return err
	}

	// handle other disks
	if d8vm.Spec.DiskAttachments != nil {
		for _, diskSource := range *d8vm.Spec.DiskAttachments {
			disk := getDisk(&diskSnap, d8vm.Namespace, diskSource.Name)
			if disk == nil {
				return fmt.Errorf("disk not found: %v", disk.Name)
			}
			if disk.VMName != "" && disk.VMName != d8vm.Name {
				return fmt.Errorf("disk already attached to another VirtualMachine: %v", disk.VMName)
			}
			if disk.VMName != d8vm.Name {
				patch := map[string]interface{}{"spec": map[string]string{"vmName": d8vm.Name}}
				input.PatchCollector.MergePatch(patch, gv, "VirtualMachineDisk", disk.Namespace, disk.Name)
			}
		}
	}

	// Handle IPAddressClaim
	ipAddress := processIPAddressClaim(input, d8vm)
	if ipAddress == "" {
		// IP is not assigned yet
		return nil
	}

	// handle KubeVirt VirtualMachine
	if err := processKubevirtVM(input, d8vm, ipAddress); err != nil {
		return err
	}

	return nil
}

func checkAndCleanupDisk(input *go_hook.HookInput, disk *VirtualMachineDiskSnapshot) {
	deckhouseVMSnap := input.Snapshots[deckhouseVMSnapshot]
	if !disk.Ephemeral && disk.VMName != "" && getD8VM(&deckhouseVMSnap, disk.Namespace, disk.VMName) == nil {
		// Remove vmName
		patch := map[string]interface{}{"status": map[string]interface{}{"vmName": nil}}
		input.PatchCollector.MergePatch(patch, gv, "VirtualMachineDisk", disk.Namespace, disk.Name, object_patch.WithSubresource("/status"))
	}
}

func checkAndCleanupIPAddressClaim(input *go_hook.HookInput, ipClaim *VirtualMachineIPAddressClaimSnapshot) {
	deckhouseVMSnap := input.Snapshots[deckhouseVMSnapshot]
	if ipClaim.VMName != "" && getD8VM(&deckhouseVMSnap, ipClaim.Namespace, ipClaim.VMName) == nil {
		// Remove vmName
		patch := map[string]interface{}{"status": map[string]interface{}{"vmName": nil}}
		input.PatchCollector.MergePatch(patch, gv, "VirtualMachineIPAddressClaim", ipClaim.Namespace, ipClaim.Name, object_patch.WithSubresource("/status"))
	}

	// Handle OwnerReference
	if ipClaim.Static != nil && *ipClaim.Static && ipClaim.OwnerReferenceIsSet {
		patch := map[string]interface{}{"metadata": map[string]interface{}{"ownerReferences": nil}}
		input.PatchCollector.MergePatch(patch, gv, "VirtualMachineIPAddressClaim", ipClaim.Namespace, ipClaim.Name)
	}
	if ipClaim.Static != nil && !*ipClaim.Static && !ipClaim.OwnerReferenceIsSet && ipClaim.VMName != "" {
		// TODO: should we remove non-static ip automatically?
		if d8vm := getD8VM(&deckhouseVMSnap, ipClaim.Namespace, ipClaim.VMName); d8vm != nil {
			ownerReferences := []v1.OwnerReference{{
				APIVersion:         gv,
				BlockOwnerDeletion: pointer.Bool(true),
				Controller:         pointer.Bool(true),
				Kind:               "VirtualMachine",
				Name:               d8vm.Name,
				UID:                d8vm.UID,
			}}
			patch := map[string]interface{}{"metadata": map[string]interface{}{"ownerReferences": ownerReferences}}
			input.PatchCollector.MergePatch(patch, gv, "VirtualMachineIPAddressClaim", ipClaim.Namespace, ipClaim.Name)
		}
	}
}

func processVMBootDisk(input *go_hook.HookInput, d8vm *v1alpha1.VirtualMachine) error {
	if d8vm.Spec.BootDisk == nil {
		return nil
	}
	if d8vm.Spec.BootDisk.Source == nil {
		return nil
	}

	diskSnap := input.Snapshots[vmDisksSnapshot]
	var disk *VirtualMachineDiskSnapshot

	bootVirtualMachineDiskName := d8vm.Spec.BootDisk.Name
	if bootVirtualMachineDiskName == "" {
		bootVirtualMachineDiskName = d8vm.Name + "-boot"
	}
	disk = getDisk(&diskSnap, d8vm.Namespace, bootVirtualMachineDiskName)

	switch d8vm.Spec.BootDisk.Source.Kind {
	case "ClusterVirtualMachineImage":
		ownerReferences := []v1.OwnerReference{}
		if d8vm.Spec.BootDisk.AutoDelete {
			ownerReferences = []v1.OwnerReference{{
				APIVersion:         gv,
				BlockOwnerDeletion: pointer.Bool(true),
				Controller:         pointer.Bool(true),
				Kind:               "VirtualMachine",
				Name:               d8vm.Name,
				UID:                d8vm.UID,
			}}
		}

		if disk != nil {
			// Disk found
			if disk.VMName != "" && disk.VMName != d8vm.Name {
				return fmt.Errorf("disk already attached to another VirtualMachine: %v", disk.VMName)
			}
			patchMetadata := map[string]interface{}{}
			patchSpec := map[string]interface{}{}
			patchStatus := map[string]interface{}{}
			// ensure size is set correctly
			if d8vm.Spec.BootDisk.Size.CmpInt64(disk.Size.Value()) == 1 {
				patchSpec["size"] = d8vm.Spec.BootDisk.Size.String()
			}
			// ensure ephemeral is set
			if d8vm.Spec.BootDisk.AutoDelete != disk.Ephemeral {
				patchMetadata["ownerReferences"] = ownerReferences
				patchStatus["ephemeral"] = d8vm.Spec.BootDisk.AutoDelete
			}
			if d8vm.Name != disk.VMName {
				patchStatus["vmName"] = d8vm.Name
			}
			if len(patchMetadata) != 0 || len(patchSpec) != 0 {
				patch := map[string]interface{}{"metadata": patchMetadata, "spec": patchSpec}
				input.PatchCollector.MergePatch(patch, gv, "VirtualMachineDisk", d8vm.Namespace, bootVirtualMachineDiskName)
			}
			if len(patchStatus) != 0 {
				patch := map[string]interface{}{"status": patchStatus}
				input.PatchCollector.MergePatch(patch, gv, "VirtualMachineDisk", d8vm.Namespace, bootVirtualMachineDiskName, object_patch.WithSubresource("/status"))
			}
		} else {
			// Disk not found, create a new VirtualMachineDisk
			disk := &v1alpha1.VirtualMachineDisk{
				TypeMeta: metav1.TypeMeta{
					Kind:       "VirtualMachineDisk",
					APIVersion: gv,
				},
				ObjectMeta: v1.ObjectMeta{
					Name:            bootVirtualMachineDiskName,
					Namespace:       d8vm.Namespace,
					OwnerReferences: ownerReferences,
				},
				Spec: v1alpha1.VirtualMachineDiskSpec{
					StorageClassName: d8vm.Spec.BootDisk.StorageClassName,
					Size:             d8vm.Spec.BootDisk.Size,
					Source:           d8vm.Spec.BootDisk.Source,
				},
			}
			input.PatchCollector.Create(disk)
			patch := map[string]interface{}{"status": map[string]interface{}{"vmName": d8vm.Name, "ephemeral": d8vm.Spec.BootDisk.AutoDelete}}
			input.PatchCollector.MergePatch(patch, gv, "VirtualMachineDisk", d8vm.Namespace, bootVirtualMachineDiskName, object_patch.WithSubresource("/status"))
		}
	case "VirtualMachineImage":
		// TODO handle namespaced VirtualMachineImage
		return fmt.Errorf("Not implemented")

	default:
		input.LogEntry.Warnln("Unknown source kind")
	}

	return nil
}

func processIPAddressClaim(input *go_hook.HookInput, d8vm *v1alpha1.VirtualMachine) string {
	ipClaimSnap := input.Snapshots[ipClaimsSnapshot]
	var ipClaimName string
	if d8vm.Spec.IPAddressClaimName != nil {
		ipClaimName = *d8vm.Spec.IPAddressClaimName
	} else {
		ipClaimName = d8vm.Name
	}
	ipClaim := getIPClaim(&ipClaimSnap, d8vm.Namespace, ipClaimName)

	if ipClaim == nil {
		// Claim is not found, create a new one
		claim := &v1alpha1.VirtualMachineIPAddressClaim{
			TypeMeta: metav1.TypeMeta{
				Kind:       "VirtualMachineIPAddressClaim",
				APIVersion: gv,
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      ipClaimName,
				Namespace: d8vm.Namespace,
				OwnerReferences: []v1.OwnerReference{{
					APIVersion:         gv,
					BlockOwnerDeletion: pointer.Bool(true),
					Controller:         pointer.Bool(true),
					Kind:               "VirtualMachine",
					Name:               d8vm.Name,
					UID:                d8vm.UID,
				}},
			},
			Spec: v1alpha1.VirtualMachineIPAddressClaimSpec{
				Static: pointer.Bool(false),
			},
		}
		input.PatchCollector.Create(claim)
		return ""
	}

	if ipClaim.Phase != "Bound" || (ipClaim.VMName != "" && ipClaim.VMName != d8vm.Name) {
		// IPAddressClaim is not in valid state, nothing to do
		if d8vm.Status.Phase != "ErrorIPNotFound" {
			patch := map[string]interface{}{"status": map[string]string{"phase": "ErrorIPNotFound"}}
			input.PatchCollector.MergePatch(patch, gv, "VirtualMachine", d8vm.Namespace, d8vm.Name, object_patch.WithSubresource("/status"))
		}
		return ""
	}
	if ipClaim.Address == "" {
		// IPAddressClaim is not assigned by IPAM, nothing to do
		if d8vm.Status.Phase != "ErrorIPNotAssigned" {
			patch := map[string]interface{}{"status": map[string]string{"phase": "ErrorIPNotAssigned"}}
			input.PatchCollector.MergePatch(patch, gv, "VirtualMachine", d8vm.Namespace, d8vm.Name, object_patch.WithSubresource("/status"))
		}
		return ""
	}
	// Update status of IPAddressClaim
	if ipClaim.VMName != d8vm.Name {
		patch := map[string]interface{}{"status": map[string]interface{}{"vmName": d8vm.Name}}
		input.PatchCollector.MergePatch(patch, gv, "VirtualMachineIPAddressClaim", ipClaim.Namespace, ipClaim.Name, object_patch.WithSubresource("/status"))
	}
	return ipClaim.Address
}

func processKubevirtVM(input *go_hook.HookInput, d8vm *v1alpha1.VirtualMachine, ipAddress string) error {
	kubevirtVMSnap := input.Snapshots[kubevirtVMSnapshot]
	kvvm := getKubevirtVM(&kubevirtVMSnap, d8vm.Namespace, d8vm.Name)
	if kvvm != nil {
		// KubeVirt VirtualMachine found
		apply := func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			vm := &virtv1.VirtualMachine{}
			err := sdk.FromUnstructured(u, vm)
			if err != nil {
				return nil, err
			}
			err = setVMFields(d8vm, vm, ipAddress)
			if err != nil {
				return nil, err
			}
			return sdk.ToUnstructured(&vm)
		}
		input.PatchCollector.Filter(apply, "kubevirt.io/v1", "VirtualMachine", d8vm.Namespace, d8vm.Name)
	} else {
		// KubeVirt VirtualMachine not found, needs to create a new one
		kvvm = &virtv1.VirtualMachine{}
		err := setVMFields(d8vm, kvvm, ipAddress)
		if err != nil {
			return err
		}
		input.PatchCollector.Create(kvvm)
	}

	// Update status of VirtualMachine
	patchStatus := map[string]interface{}{}
	if d8vm.Status.IPAddress != ipAddress {
		patchStatus["ipAddress"] = ipAddress
	}
	if string(d8vm.Status.Phase) != string(kvvm.Status.PrintableStatus) {
		patchStatus["phase"] = kvvm.Status.PrintableStatus
	}
	if len(patchStatus) != 0 {
		patch := map[string]interface{}{"status": patchStatus}
		input.PatchCollector.MergePatch(patch, gv, "VirtualMachine", d8vm.Namespace, d8vm.Name, object_patch.WithSubresource("/status"))
	}
	return nil
}
