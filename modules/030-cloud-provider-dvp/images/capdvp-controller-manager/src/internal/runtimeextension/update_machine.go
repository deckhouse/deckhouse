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

	"github.com/deckhouse/virtualization/api/core/v1alpha2"

	infrastructurev1a1 "cluster-api-provider-dvp/api/v1alpha1"
)

// HandleUpdateMachine performs the actual in-place update of a DVP virtual machine.
// Currently the only supported in-place operation is hot-plugging additional disks.
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

// performInPlaceUpdate hot-plugs new additional disks to a running VM.
func (e *Extension) performInPlaceUpdate(
	ctx context.Context,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
	newTemplate *infrastructurev1a1.DeckhouseMachineTemplate,
) error {
	newSpec := newTemplate.Spec.Template.Spec
	currentDiskCount := len(dvpMachine.Spec.AdditionalDisks)
	newDiskCount := len(newSpec.AdditionalDisks)

	if newDiskCount <= currentDiskCount {
		e.log.Info("No new disks to add", "vm", dvpMachine.Name)
		return nil
	}

	e.log.Info("Adding disks via hot-plug",
		"vm", dvpMachine.Name,
		"current", currentDiskCount,
		"new", newDiskCount,
	)

	vmHostname := dvpMachine.Name

	for i := currentDiskCount; i < newDiskCount; i++ {
		diskSpec := newSpec.AdditionalDisks[i]
		diskName := fmt.Sprintf("%s-additional-disk-%d", dvpMachine.Name, i)

		_, err := e.dvp.DiskService.CreateDisk(
			ctx,
			e.clusterUUID,
			vmHostname,
			diskName,
			diskSpec.Size.Value(),
			diskSpec.StorageClass,
		)
		if err != nil {
			return fmt.Errorf("create disk %s: %w", diskName, err)
		}

		if err := e.dvp.ComputeService.AttachDiskToVM(ctx, diskName, vmHostname); err != nil {
			return fmt.Errorf("attach disk %s to VM %s: %w", diskName, vmHostname, err)
		}

		e.log.Info("Disk hot-plugged", "disk", diskName, "vm", vmHostname)
	}

	if err := e.waitForVMReady(ctx, dvpMachine.Name, 5*time.Minute); err != nil {
		return fmt.Errorf("VM did not become ready after update: %w", err)
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
