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

	ctx := context.Background()

	dvpMachine := &infrastructurev1a1.DeckhouseMachine{}
	if err := e.client.Get(ctx, types.NamespacedName{
		Name:      req.Machine.Name,
		Namespace: req.Machine.Namespace,
	}, dvpMachine); err != nil {
		e.log.Error(err, "failed to get DeckhouseMachine")
		writeError(w, http.StatusInternalServerError, "failed to get DeckhouseMachine: "+err.Error())
		return
	}

	newTemplate, err := e.getMachineTemplate(ctx, req.Machine.Spec.InfrastructureRef)
	if err != nil {
		e.log.Error(err, "failed to get new DeckhouseMachineTemplate")
		writeError(w, http.StatusInternalServerError, "failed to get new template: "+err.Error())
		return
	}

	if err := e.performInPlaceUpdate(ctx, dvpMachine, newTemplate); err != nil {
		e.log.Error(err, "in-place update failed", "machine", req.Machine.Name)
		writeError(w, http.StatusInternalServerError, "update failed: "+err.Error())
		return
	}

	resp := UpdateMachineResponse{
		Status:  "Success",
		Message: "in-place update completed successfully",
	}
	e.log.Info("UpdateMachine completed", "machine", req.Machine.Name)
	writeJSON(w, http.StatusOK, resp)
}

// performInPlaceUpdate applies in-place changes to a running DVP VM.
// It builds a "current spec" from the DeckhouseMachine, classifies the diff,
// and executes the appropriate strategy.
func (e *Extension) performInPlaceUpdate(
	ctx context.Context,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
	newTemplate *infrastructurev1a1.DeckhouseMachineTemplate,
) error {
	newSpec := &newTemplate.Spec.Template.Spec

	// Build the "old" spec from current DeckhouseMachine state.
	oldSpec := templateSpecFromMachine(dvpMachine)

	cs := classifyChanges(oldSpec, newSpec)
	vmName := dvpMachine.Name

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

	// ---- Warm update: stop → patch → start ----
	if cs.strategy == updateWarm {
		if err := e.warmUpdate(ctx, vmName, dvpMachine, newSpec, &cs); err != nil {
			return err
		}
	}

	// ---- Hot-plug new disks ----
	if cs.newDisksAdded {
		if err := e.hotPlugNewDisks(ctx, dvpMachine, newSpec); err != nil {
			return err
		}
	}

	// ---- Live-patch policies (no restart needed) ----
	if cs.runPolicyChanged || cs.liveMigrationChanged {
		if err := e.patchVMPolicies(ctx, vmName, newSpec); err != nil {
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
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
	newSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
	cs *changeSet,
) error {
	e.log.Info("Warm update: stopping VM", "vm", vmName)
	if err := e.dvp.ComputeService.StopVM(ctx, vmName); err != nil {
		return fmt.Errorf("stop VM %s: %w", vmName, err)
	}

	// Patch VM spec (CPU, Memory, VMClass).
	if cs.cpuChanged || cs.memoryChanged || cs.vmClassChanged {
		e.log.Info("Warm update: patching VM spec", "vm", vmName)
		if err := e.patchVMSpec(ctx, vmName, newSpec, cs); err != nil {
			// Try to restart VM even if patch failed, to avoid leaving it stopped.
			_ = e.dvp.ComputeService.StartVM(ctx, vmName)
			return fmt.Errorf("patch VM spec: %w", err)
		}
	}

	// Resize root disk if needed.
	if cs.rootDiskResized {
		bootDiskName := dvpMachine.Name + "-boot"
		e.log.Info("Warm update: resizing root disk",
			"vm", vmName,
			"disk", bootDiskName,
			"newSize", newSpec.RootDiskSize.String(),
		)
		if err := e.dvp.DiskService.ResizeDisk(ctx, bootDiskName, newSpec.RootDiskSize.String()); err != nil {
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
	newSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
	cs *changeSet,
) error {
	vm, err := e.dvp.ComputeService.GetVMByName(ctx, vmName)
	if err != nil {
		return fmt.Errorf("get VM %s: %w", vmName, err)
	}

	before := vm.DeepCopy()

	if cs.cpuChanged {
		vm.Spec.CPU.Cores = newSpec.CPU.Cores
		vm.Spec.CPU.CoreFraction = newSpec.CPU.Fraction
	}
	if cs.memoryChanged {
		vm.Spec.Memory.Size = newSpec.Memory
	}
	if cs.vmClassChanged {
		vm.Spec.VirtualMachineClassName = newSpec.VMClassName
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
	newSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
) error {
	vm, err := e.dvp.ComputeService.GetVMByName(ctx, vmName)
	if err != nil {
		return fmt.Errorf("get VM %s: %w", vmName, err)
	}

	before := vm.DeepCopy()

	if newSpec.RunPolicy != "" {
		vm.Spec.RunPolicy = v1alpha2.RunPolicy(newSpec.RunPolicy)
	}
	if newSpec.LiveMigrationPolicy != "" {
		vm.Spec.LiveMigrationPolicy = v1alpha2.LiveMigrationPolicy(newSpec.LiveMigrationPolicy)
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
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
	newSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate,
) error {
	currentDiskCount := len(dvpMachine.Spec.AdditionalDisks)
	newDiskCount := len(newSpec.AdditionalDisks)
	vmHostname := dvpMachine.Name

	e.log.Info("Hot-plugging new disks",
		"vm", vmHostname,
		"current", currentDiskCount,
		"new", newDiskCount,
	)

	for i := currentDiskCount; i < newDiskCount; i++ {
		diskSpec := newSpec.AdditionalDisks[i]
		diskName := fmt.Sprintf("%s-additional-disk-%d", dvpMachine.Name, i)

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

// templateSpecFromMachine builds a DeckhouseMachineSpecTemplate from the
// current DeckhouseMachine spec so we can compare it with the new template.
func templateSpecFromMachine(m *infrastructurev1a1.DeckhouseMachine) *infrastructurev1a1.DeckhouseMachineSpecTemplate {
	return &infrastructurev1a1.DeckhouseMachineSpecTemplate{
		VMClassName:          m.Spec.VMClassName,
		CPU:                  m.Spec.CPU,
		Memory:               m.Spec.Memory,
		AdditionalDisks:      m.Spec.AdditionalDisks,
		RootDiskSize:         m.Spec.RootDiskSize,
		RootDiskStorageClass: m.Spec.RootDiskStorageClass,
		BootDiskImageRef:     m.Spec.BootDiskImageRef,
		Bootloader:           m.Spec.Bootloader,
		RunPolicy:            m.Spec.RunPolicy,
		LiveMigrationPolicy:  m.Spec.LiveMigrationPolicy,
	}
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
