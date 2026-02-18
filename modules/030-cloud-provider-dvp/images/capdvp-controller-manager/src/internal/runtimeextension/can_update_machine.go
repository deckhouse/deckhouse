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
	"net/http"
	"reflect"

	"k8s.io/apimachinery/pkg/types"

	infrastructurev1a1 "cluster-api-provider-dvp/api/v1alpha1"
)

// HandleCanUpdateMachine decides whether the requested Machine spec change
// can be applied in-place (without VM replacement).
func (e *Extension) HandleCanUpdateMachine(w http.ResponseWriter, r *http.Request) {
	e.log.Info("CanUpdateMachine request received")

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CanUpdateMachineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ctx := context.Background()

	oldTemplate, err := e.getMachineTemplate(ctx, req.OldMachine.Spec.InfrastructureRef)
	if err != nil {
		e.log.Error(err, "failed to get old DeckhouseMachineTemplate")
		writeError(w, http.StatusInternalServerError, "failed to get old template: "+err.Error())
		return
	}

	newTemplate, err := e.getMachineTemplate(ctx, req.Machine.Spec.InfrastructureRef)
	if err != nil {
		e.log.Error(err, "failed to get new DeckhouseMachineTemplate")
		writeError(w, http.StatusInternalServerError, "failed to get new template: "+err.Error())
		return
	}

	canUpdate, message := canUpdateInPlace(
		&oldTemplate.Spec.Template.Spec,
		&newTemplate.Spec.Template.Spec,
	)

	resp := CanUpdateMachineResponse{
		Status:    "Success",
		CanUpdate: canUpdate,
		Message:   message,
	}

	e.log.Info("CanUpdateMachine response",
		"machine", req.Machine.Name,
		"canUpdate", canUpdate,
		"message", message,
	)
	writeJSON(w, http.StatusOK, resp)
}

func (e *Extension) getMachineTemplate(ctx context.Context, ref ObjectRef) (*infrastructurev1a1.DeckhouseMachineTemplate, error) {
	tmpl := &infrastructurev1a1.DeckhouseMachineTemplate{}
	err := e.client.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}, tmpl)
	return tmpl, err
}

// canUpdateInPlace compares old and new DeckhouseMachineSpecTemplate and
// returns true only when the diff can be applied without VM replacement.
//
// DVP (KubeVirt-based) supports:
//   - Disk hot-plug (adding new additional disks)
//
// DVP does NOT support:
//   - CPU hot-plug
//   - Memory hot-plug
func canUpdateInPlace(oldSpec, newSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate) (bool, string) {
	if oldSpec.CPU.Cores != newSpec.CPU.Cores {
		return false, "CPU cores changed — requires VM replacement (DVP does not support CPU hot-plug)"
	}
	if oldSpec.CPU.Fraction != newSpec.CPU.Fraction {
		return false, "CPU fraction changed — requires VM replacement"
	}

	if oldSpec.Memory.Cmp(newSpec.Memory) != 0 {
		return false, "memory changed — requires VM replacement (DVP does not support memory hot-plug)"
	}

	if !reflect.DeepEqual(oldSpec.BootDiskImageRef, newSpec.BootDiskImageRef) {
		return false, "boot disk image changed — requires VM replacement"
	}

	if oldSpec.RootDiskSize.Cmp(newSpec.RootDiskSize) != 0 {
		return false, "root disk size changed — requires VM replacement"
	}

	if oldSpec.RootDiskStorageClass != newSpec.RootDiskStorageClass {
		return false, "root disk storage class changed — requires VM replacement"
	}

	if oldSpec.Bootloader != newSpec.Bootloader {
		return false, "bootloader changed — requires VM replacement"
	}

	if oldSpec.VMClassName != newSpec.VMClassName {
		return false, "VM class changed — requires VM replacement"
	}

	if oldSpec.RunPolicy != newSpec.RunPolicy {
		return false, "run policy changed — requires VM replacement"
	}

	if oldSpec.LiveMigrationPolicy != newSpec.LiveMigrationPolicy {
		return false, "live migration policy changed — requires VM replacement"
	}

	// Existing disks removed or modified → replacement required.
	if len(newSpec.AdditionalDisks) < len(oldSpec.AdditionalDisks) {
		return false, "additional disks removed — requires VM replacement"
	}
	for i := range oldSpec.AdditionalDisks {
		if !reflect.DeepEqual(oldSpec.AdditionalDisks[i], newSpec.AdditionalDisks[i]) {
			return false, "existing additional disk modified — requires VM replacement"
		}
	}

	// New disks appended — hot-plug is supported.
	if len(newSpec.AdditionalDisks) > len(oldSpec.AdditionalDisks) {
		return true, "additional disks can be hot-plugged"
	}

	// No changes detected.
	if reflect.DeepEqual(oldSpec, newSpec) {
		return true, "no changes detected"
	}

	return false, "unknown changes detected — requires VM replacement for safety"
}
