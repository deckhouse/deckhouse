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

package runtimeextension

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"

	infrastructurev1a1 "cluster-api-provider-dvp/api/v1alpha1"
)

// HandleUpdateMachine performs the actual in-place update of a DVP virtual machine.
// Supports three strategies:
//   - Hot: disk hot-plug, policy patches (no downtime)
//   - Warm: stop VM → patch spec → start VM (brief downtime)
//   - Mixed: warm + hot combined
func (e *Extension) HandleUpdateMachine(w http.ResponseWriter, r *http.Request) {
	e.log.Info("UpdateMachine request received")

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateMachineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	var desiredMachine infrastructurev1a1.DeckhouseMachine
	if err := json.Unmarshal(req.Desired.InfrastructureMachine, &desiredMachine); err != nil {
		e.log.Error(err, "failed to unmarshal desired InfrastructureMachine")
		writeError(w, http.StatusBadRequest, "failed to unmarshal desired machine: "+err.Error())
		return
	}

	ctx := context.Background()

	currentMachine := &infrastructurev1a1.DeckhouseMachine{}
	if err := e.client.Get(ctx, types.NamespacedName{
		Name:      desiredMachine.Name,
		Namespace: desiredMachine.Namespace,
	}, currentMachine); err != nil {
		e.log.Error(err, "failed to get current DeckhouseMachine")
		writeError(w, http.StatusInternalServerError, "failed to get current machine: "+err.Error())
		return
	}

	desiredSpec := specFromMachine(&desiredMachine.Spec)

	if err := e.performInPlaceUpdate(ctx, currentMachine, desiredSpec); err != nil {
		e.log.Error(err, "in-place update failed", "machine", desiredMachine.Name)
		resp := UpdateMachineResponse{
			CommonRetryResponse: CommonRetryResponse{
				CommonResponse: CommonResponse{
					Status:  "Failure",
					Message: "update failed: " + err.Error(),
				},
				RetryAfterSeconds: 0,
			},
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp := UpdateMachineResponse{
		CommonRetryResponse: CommonRetryResponse{
			CommonResponse: CommonResponse{
				Status:  "Success",
				Message: "in-place update completed successfully",
			},
			RetryAfterSeconds: 0,
		},
	}
	e.log.Info("UpdateMachine completed", "machine", desiredMachine.Name)
	writeJSON(w, http.StatusOK, resp)
}

// specFromActualVM builds a DeckhouseMachineSpecTemplate by combining:
//   - Mutable fields from the real VM/disk state in DVP (CPU, memory, vmClass, rootDiskSize, runPolicy)
//   - Immutable fields from the DeckhouseMachine K8s object (bootloader, bootDiskImageRef, rootDiskStorageClass)
//
// This is needed because CAPI updates the DeckhouseMachine spec to the desired state
// BEFORE calling UpdateMachine, so we can't use the K8s object for mutable fields.
func (e *Extension) specFromActualVM(ctx context.Context, vmName string, currentMachine *infrastructurev1a1.DeckhouseMachine) (*infrastructurev1a1.DeckhouseMachineSpecTemplate, error) {
	vm, err := e.dvp.ComputeService.GetVMByName(ctx, vmName)
	if err != nil {
		return nil, fmt.Errorf("get VM %s: %w", vmName, err)
	}

	spec := &infrastructurev1a1.DeckhouseMachineSpecTemplate{
		// Mutable fields — from real VM state
		VMClassName: vm.Spec.VirtualMachineClassName,
		CPU: infrastructurev1a1.CPU{
			Cores:    vm.Spec.CPU.Cores,
			Fraction: string(vm.Spec.CPU.CoreFraction),
		},
		Memory:    vm.Spec.Memory.Size,
		RunPolicy: string(vm.Spec.RunPolicy),

		// Immutable fields — safe to take from the K8s object (they never change in-place)
		Bootloader:           currentMachine.Spec.Bootloader,
		BootDiskImageRef:     currentMachine.Spec.BootDiskImageRef,
		RootDiskStorageClass: currentMachine.Spec.RootDiskStorageClass,
		AdditionalDisks:      currentMachine.Spec.AdditionalDisks,
	}

	bootDiskName := vmName + "-boot"
	bootDisk, err := e.dvp.DiskService.GetDiskByName(ctx, bootDiskName)
	if err != nil {
		return nil, fmt.Errorf("get boot disk %s: %w", bootDiskName, err)
	}
	if bootDisk.Spec.PersistentVolumeClaim.Size != nil {
		spec.RootDiskSize = *bootDisk.Spec.PersistentVolumeClaim.Size
	}

	return spec, nil
}

// performInPlaceUpdate applies in-place changes to a running DVP VM.
func (e *Extension) performInPlaceUpdate(
	ctx context.Context,
	currentMachine *infrastructurev1a1.DeckhouseMachine,
	desiredSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
) error {
	vmName := currentMachine.Name

	oldSpec, err := e.specFromActualVM(ctx, vmName, currentMachine)
	if err != nil {
		return fmt.Errorf("get actual VM state: %w", err)
	}
	cs := classifyChanges(oldSpec, desiredSpec)

	e.log.Info("In-place update plan",
		"vm", vmName,
		"strategy", strategyName(cs.strategy),
		"cpu", cs.cpuChanged,
		"memory", cs.memoryChanged,
		"vmClass", cs.vmClassChanged,
		"rootDiskResize", cs.rootDiskResized,
		"newDisks", cs.newDisksAdded,
		"runPolicy", cs.runPolicyChanged,
		"liveMigration", cs.liveMigrationChanged,
	)

	if cs.strategy == updateNone {
		e.log.Info("No changes to apply", "vm", vmName)
		return nil
	}

	if cs.strategy >= updateWarm {
		if err := e.warmUpdate(ctx, vmName, currentMachine, desiredSpec, &cs); err != nil {
			return err
		}
	}

	if cs.rootDiskResized {
		if err := e.resizeRootDisk(ctx, currentMachine, desiredSpec); err != nil {
			return err
		}
	}

	if cs.newDisksAdded {
		if err := e.hotPlugNewDisks(ctx, currentMachine, desiredSpec); err != nil {
			return err
		}
	}

	if cs.runPolicyChanged || cs.liveMigrationChanged {
		if err := e.patchVMPolicies(ctx, vmName, desiredSpec); err != nil {
			return err
		}
	}

	if err := e.waitForVMReady(ctx, vmName, 5*time.Minute); err != nil {
		return fmt.Errorf("VM did not become ready after update: %w", err)
	}

	e.log.Info("In-place update completed", "vm", vmName)
	return nil
}

// deleteVMOperation removes a VirtualMachineOperation by name, ignoring NotFound.
// This is needed because StopVM/StartVM create VMOPs with the same name (the VM name),
// so we must clean up the previous one before creating the next.
func (e *Extension) deleteVMOperation(ctx context.Context, vmName string) {
	vmop := &v1alpha2.VirtualMachineOperation{}
	vmop.Name = vmName
	vmop.Namespace = e.dvp.ProjectNamespace()
	if err := e.dvp.Service.GetClient().Delete(ctx, vmop); err != nil {
		e.log.V(1).Info("Failed to delete VMOperation (may not exist)", "vm", vmName, "error", err.Error())
	}
}

func (e *Extension) getMachineNodeName(ctx context.Context, machineName, machineNamespace string) (string, error) {
	machine := &clusterv1.Machine{}
	if err := e.client.Get(ctx, types.NamespacedName{Name: machineName, Namespace: machineNamespace}, machine); err != nil {
		return "", err
	}
	if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
		return "", nil
	}
	return machine.Status.NodeRef.Name, nil
}

func (e *Extension) setNodeUnschedulable(ctx context.Context, nodeName string, unschedulable bool) error {
	node := &corev1.Node{}
	if err := e.client.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		return err
	}
	if node.Spec.Unschedulable == unschedulable {
		return nil
	}
	before := node.DeepCopy()
	node.Spec.Unschedulable = unschedulable
	return e.client.Patch(ctx, node, client.MergeFrom(before))
}

func isEvictablePod(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return false
	}
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false
	}
	if _, ok := pod.Annotations[corev1.MirrorPodAnnotationKey]; ok {
		return false
	}
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" && owner.Controller != nil && *owner.Controller {
			return false
		}
	}
	return true
}

func (e *Extension) drainNode(ctx context.Context, nodeName string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout draining node %s", nodeName)
		case <-ticker.C:
			podList := &corev1.PodList{}
			if err := e.client.List(ctx, podList); err != nil {
				return fmt.Errorf("list pods for drain: %w", err)
			}

			var evictable []*corev1.Pod
			for i := range podList.Items {
				pod := &podList.Items[i]
				if pod.Spec.NodeName != nodeName {
					continue
				}
				if !isEvictablePod(pod) {
					continue
				}
				evictable = append(evictable, pod)
			}

			if len(evictable) == 0 {
				e.log.Info("Node drain completed", "node", nodeName)
				return nil
			}

			for _, pod := range evictable {
				eviction := &policyv1.Eviction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					},
				}
				err := e.client.SubResource("eviction").Create(ctx, pod, eviction)
				if err == nil || apierrors.IsNotFound(err) {
					continue
				}
				if apierrors.IsTooManyRequests(err) || apierrors.IsConflict(err) {
					continue
				}
				return fmt.Errorf("evict pod %s/%s: %w", pod.Namespace, pod.Name, err)
			}
		}
	}
}

// warmUpdate stops the VM, patches its spec, and starts it again.
func (e *Extension) warmUpdate(
	ctx context.Context,
	vmName string,
	currentMachine *infrastructurev1a1.DeckhouseMachine,
	desiredSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
	cs *changeSet,
) error {
	nodeName, err := e.getMachineNodeName(ctx, currentMachine.Name, currentMachine.Namespace)
	if err != nil {
		return fmt.Errorf("get machine node name: %w", err)
	}
	if nodeName != "" {
		e.log.Info("Warm update: cordoning node", "node", nodeName, "vm", vmName)
		if err := e.setNodeUnschedulable(ctx, nodeName, true); err != nil {
			return fmt.Errorf("cordon node %s: %w", nodeName, err)
		}
		defer func() {
			e.log.Info("Warm update: uncordoning node", "node", nodeName, "vm", vmName)
			if err := e.setNodeUnschedulable(ctx, nodeName, false); err != nil {
				e.log.Error(err, "failed to uncordon node after warm update", "node", nodeName, "vm", vmName)
			}
		}()

		e.log.Info("Warm update: draining node", "node", nodeName, "vm", vmName)
		if err := e.drainNode(ctx, nodeName, 10*time.Minute); err != nil {
			return fmt.Errorf("drain node %s: %w", nodeName, err)
		}
	}

	e.deleteVMOperation(ctx, vmName)

	e.log.Info("Warm update: stopping VM", "vm", vmName)
	if err := e.dvp.ComputeService.StopVM(ctx, vmName); err != nil {
		return fmt.Errorf("stop VM %s: %w", vmName, err)
	}

	if cs.cpuChanged || cs.memoryChanged || cs.vmClassChanged {
		e.log.Info("Warm update: patching VM spec", "vm", vmName)
		if err := e.patchVMSpec(ctx, vmName, desiredSpec, cs); err != nil {
			e.deleteVMOperation(ctx, vmName)
			_ = e.dvp.ComputeService.StartVM(ctx, vmName)
			return fmt.Errorf("patch VM spec: %w", err)
		}
	}

	e.deleteVMOperation(ctx, vmName)

	e.log.Info("Warm update: starting VM", "vm", vmName)
	if err := e.dvp.ComputeService.StartVM(ctx, vmName); err != nil {
		return fmt.Errorf("start VM %s: %w", vmName, err)
	}

	return nil
}

func (e *Extension) resizeRootDisk(
	ctx context.Context,
	currentMachine *infrastructurev1a1.DeckhouseMachine,
	desiredSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
) error {
	bootDiskName := currentMachine.Name + "-boot"
	e.log.Info("Hot update: resizing root disk",
		"vm", currentMachine.Name,
		"disk", bootDiskName,
		"newSize", desiredSpec.RootDiskSize.String(),
	)
	if err := e.dvp.DiskService.ResizeDisk(ctx, bootDiskName, desiredSpec.RootDiskSize.String()); err != nil {
		return fmt.Errorf("resize root disk %s: %w", bootDiskName, err)
	}
	return nil
}

// patchVMSpec patches the VirtualMachine object in the DVP parent cluster.
func (e *Extension) patchVMSpec(
	ctx context.Context,
	vmName string,
	desiredSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
	cs *changeSet,
) error {
	vm, err := e.dvp.ComputeService.GetVMByName(ctx, vmName)
	if err != nil {
		return fmt.Errorf("get VM %s: %w", vmName, err)
	}

	before := vm.DeepCopy()

	if cs.cpuChanged {
		vm.Spec.CPU.Cores = desiredSpec.CPU.Cores
		vm.Spec.CPU.CoreFraction = desiredSpec.CPU.Fraction
	}
	if cs.memoryChanged {
		vm.Spec.Memory.Size = desiredSpec.Memory
	}
	if cs.vmClassChanged {
		vm.Spec.VirtualMachineClassName = desiredSpec.VMClassName
	}

	dvpClient := e.dvp.Service.GetClient()
	if err := dvpClient.Patch(ctx, vm, client.MergeFrom(before)); err != nil {
		return fmt.Errorf("patch VM %s: %w", vmName, err)
	}

	return nil
}

// patchVMPolicies live-patches runPolicy and liveMigrationPolicy on a running VM.
func (e *Extension) patchVMPolicies(
	ctx context.Context,
	vmName string,
	desiredSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
) error {
	vm, err := e.dvp.ComputeService.GetVMByName(ctx, vmName)
	if err != nil {
		return fmt.Errorf("get VM %s: %w", vmName, err)
	}

	before := vm.DeepCopy()

	if desiredSpec.RunPolicy != "" {
		vm.Spec.RunPolicy = v1alpha2.RunPolicy(desiredSpec.RunPolicy)
	}
	if desiredSpec.LiveMigrationPolicy != "" {
		vm.Spec.LiveMigrationPolicy = v1alpha2.LiveMigrationPolicy(desiredSpec.LiveMigrationPolicy)
	}

	dvpClient := e.dvp.Service.GetClient()
	if err := dvpClient.Patch(ctx, vm, client.MergeFrom(before)); err != nil {
		return fmt.Errorf("patch VM policies %s: %w", vmName, err)
	}

	e.log.Info("VM policies patched", "vm", vmName)
	return nil
}

// hotPlugNewDisks creates and attaches new additional disks via VMBDA.
func (e *Extension) hotPlugNewDisks(
	ctx context.Context,
	currentMachine *infrastructurev1a1.DeckhouseMachine,
	desiredSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
) error {
	currentDiskCount := len(currentMachine.Spec.AdditionalDisks)
	newDiskCount := len(desiredSpec.AdditionalDisks)
	vmHostname := currentMachine.Name

	e.log.Info("Hot-plugging new disks",
		"vm", vmHostname,
		"current", currentDiskCount,
		"new", newDiskCount,
	)

	for i := currentDiskCount; i < newDiskCount; i++ {
		diskSpec := desiredSpec.AdditionalDisks[i]
		diskName := fmt.Sprintf("%s-additional-disk-%d", currentMachine.Name, i)

		if _, err := e.dvp.DiskService.CreateDisk(
			ctx,
			e.clusterUUID,
			vmHostname,
			diskName,
			diskSpec.Size.Value(),
			diskSpec.StorageClass,
		); err != nil {
			return fmt.Errorf("create disk %s: %w", diskName, err)
		}

		if err := e.dvp.ComputeService.AttachDiskToVM(ctx, diskName, vmHostname); err != nil {
			return fmt.Errorf("attach disk %s to VM %s: %w", diskName, vmHostname, err)
		}

		e.log.Info("Disk hot-plugged", "disk", diskName, "vm", vmHostname)
	}

	return nil
}

func (e *Extension) waitForVMReady(ctx context.Context, vmName string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for VM %s to become Running", vmName)
		case <-ticker.C:
			vm, err := e.dvp.ComputeService.GetVMByName(ctx, vmName)
			if err != nil {
				e.log.Error(err, "error checking VM status", "vm", vmName)
				continue
			}
			if vm.Status.Phase == v1alpha2.MachineRunning {
				return nil
			}
			e.log.V(1).Info("Waiting for VM", "vm", vmName, "phase", vm.Status.Phase)
		}
	}
}

func strategyName(s updateStrategy) string {
	switch s {
	case updateNone:
		return "none"
	case updateHot:
		return "hot"
	case updateWarm:
		return "warm"
	case updateRecreate:
		return "recreate"
	default:
		return "unknown"
	}
}
