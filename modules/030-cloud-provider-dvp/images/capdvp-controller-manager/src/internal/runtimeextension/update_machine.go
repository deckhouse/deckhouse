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

	"k8s.io/apimachinery/pkg/types"
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

// performInPlaceUpdate applies in-place changes to a running DVP VM.
func (e *Extension) performInPlaceUpdate(
	ctx context.Context,
	currentMachine *infrastructurev1a1.DeckhouseMachine,
	desiredSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
) error {
	oldSpec := specFromMachine(&currentMachine.Spec)
	cs := classifyChanges(oldSpec, desiredSpec)
	vmName := currentMachine.Name

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

// warmUpdate stops the VM, patches its spec, and starts it again.
func (e *Extension) warmUpdate(
	ctx context.Context,
	vmName string,
	currentMachine *infrastructurev1a1.DeckhouseMachine,
	desiredSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
	cs *changeSet,
) error {
	e.log.Info("Warm update: stopping VM", "vm", vmName)
	if err := e.dvp.ComputeService.StopVM(ctx, vmName); err != nil {
		return fmt.Errorf("stop VM %s: %w", vmName, err)
	}

	if cs.cpuChanged || cs.memoryChanged || cs.vmClassChanged {
		e.log.Info("Warm update: patching VM spec", "vm", vmName)
		if err := e.patchVMSpec(ctx, vmName, desiredSpec, cs); err != nil {
			_ = e.dvp.ComputeService.StartVM(ctx, vmName)
			return fmt.Errorf("patch VM spec: %w", err)
		}
	}

	if cs.rootDiskResized {
		bootDiskName := currentMachine.Name + "-boot"
		e.log.Info("Warm update: resizing root disk",
			"vm", vmName,
			"disk", bootDiskName,
			"newSize", desiredSpec.RootDiskSize.String(),
		)
		if err := e.dvp.DiskService.ResizeDisk(ctx, bootDiskName, desiredSpec.RootDiskSize.String()); err != nil {
			_ = e.dvp.ComputeService.StartVM(ctx, vmName)
			return fmt.Errorf("resize root disk %s: %w", bootDiskName, err)
		}
	}

	e.log.Info("Warm update: starting VM", "vm", vmName)
	if err := e.dvp.ComputeService.StartVM(ctx, vmName); err != nil {
		return fmt.Errorf("start VM %s: %w", vmName, err)
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
