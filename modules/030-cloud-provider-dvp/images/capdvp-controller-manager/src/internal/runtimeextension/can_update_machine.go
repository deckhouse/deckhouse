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
	"encoding/json"
	"net/http"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch/v5"

	infrastructurev1a1 "cluster-api-provider-dvp/api/v1alpha1"
)

// updateStrategy describes what level of disruption is needed to apply changes.
type updateStrategy int

const (
	updateNone     updateStrategy = iota
	updateHot                     // No VM restart: disk hot-plug, policy patch
	updateWarm                    // Stop → patch spec → start
	updateRecreate                // Full VM replacement required
)

// changeSet describes what changed between two template specs and which
// strategy is required to apply those changes.
type changeSet struct {
	strategy updateStrategy
	reason   string

	// Hot-plug changes (no downtime)
	newDisksAdded        bool
	runPolicyChanged     bool
	liveMigrationChanged bool

	// Warm changes (stop → patch → start)
	cpuChanged      bool
	memoryChanged   bool
	vmClassChanged  bool
	rootDiskResized bool // only increase
}

// classifyChanges compares old and new DeckhouseMachineSpecTemplate and
// returns a changeSet describing what changed and how to apply it.
func classifyChanges(oldSpec, newSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate) changeSet {
	cs := changeSet{strategy: updateNone, reason: "no changes detected"}

	// --- Fields that require full recreate ---

	if oldSpec.Bootloader != newSpec.Bootloader {
		return changeSet{strategy: updateRecreate, reason: "bootloader changed — requires VM replacement"}
	}

	if !reflect.DeepEqual(oldSpec.BootDiskImageRef, newSpec.BootDiskImageRef) {
		return changeSet{strategy: updateRecreate, reason: "boot disk image changed — requires VM replacement"}
	}

	if oldSpec.RootDiskStorageClass != newSpec.RootDiskStorageClass {
		return changeSet{strategy: updateRecreate, reason: "root disk storage class changed — requires VM replacement"}
	}

	// Root disk shrink is impossible.
	if newSpec.RootDiskSize.Cmp(oldSpec.RootDiskSize) < 0 {
		return changeSet{strategy: updateRecreate, reason: "root disk size decreased — requires VM replacement"}
	}

	// Existing additional disks removed or modified → recreate.
	if len(newSpec.AdditionalDisks) < len(oldSpec.AdditionalDisks) {
		return changeSet{strategy: updateRecreate, reason: "additional disks removed — requires VM replacement"}
	}
	for i := range oldSpec.AdditionalDisks {
		if !reflect.DeepEqual(oldSpec.AdditionalDisks[i], newSpec.AdditionalDisks[i]) {
			return changeSet{strategy: updateRecreate, reason: "existing additional disk modified — requires VM replacement"}
		}
	}

	// --- Warm changes (stop → patch → start) ---

	if oldSpec.CPU.Cores != newSpec.CPU.Cores || oldSpec.CPU.Fraction != newSpec.CPU.Fraction {
		cs.cpuChanged = true
		cs.strategy = updateWarm
		cs.reason = "CPU changed — warm update (stop → patch → start)"
	}

	if oldSpec.Memory.Cmp(newSpec.Memory) != 0 {
		cs.memoryChanged = true
		cs.strategy = updateWarm
		cs.reason = "memory changed — warm update (stop → patch → start)"
	}

	if oldSpec.VMClassName != newSpec.VMClassName {
		cs.vmClassChanged = true
		cs.strategy = updateWarm
		cs.reason = "VM class changed — warm update (stop → patch → start)"
	}

	if newSpec.RootDiskSize.Cmp(oldSpec.RootDiskSize) > 0 {
		cs.rootDiskResized = true
		if cs.strategy < updateHot {
			cs.strategy = updateHot
			cs.reason = "root disk size increased — online resize"
		}
	}

	// --- Hot changes (no downtime) ---

	if len(newSpec.AdditionalDisks) > len(oldSpec.AdditionalDisks) {
		cs.newDisksAdded = true
		if cs.strategy < updateHot {
			cs.strategy = updateHot
			cs.reason = "new additional disks — hot-plug"
		}
	}

	if oldSpec.RunPolicy != newSpec.RunPolicy {
		cs.runPolicyChanged = true
		if cs.strategy < updateHot {
			cs.strategy = updateHot
			cs.reason = "run policy changed — live patch"
		}
	}

	if oldSpec.LiveMigrationPolicy != newSpec.LiveMigrationPolicy {
		cs.liveMigrationChanged = true
		if cs.strategy < updateHot {
			cs.strategy = updateHot
			cs.reason = "live migration policy changed — live patch"
		}
	}

	// Summarize reason when multiple things changed.
	if cs.strategy == updateWarm {
		cs.reason = "spec changes detected — warm update (stop → patch → start)"
	}

	return cs
}

// canUpdateInPlace returns true when the diff between old and new specs
// can be applied without full VM replacement.
func canUpdateInPlace(oldSpec, newSpec *infrastructurev1a1.DeckhouseMachineSpecTemplate) (bool, string) {
	cs := classifyChanges(oldSpec, newSpec)
	if cs.strategy == updateRecreate {
		return false, cs.reason
	}
	return true, cs.reason
}

// specFromMachine extracts DeckhouseMachineSpecTemplate from DeckhouseMachineSpec
// so we can reuse classifyChanges for both MachineTemplate and Machine comparisons.
func specFromMachine(m *infrastructurev1a1.DeckhouseMachineSpec) *infrastructurev1a1.DeckhouseMachineSpecTemplate {
	return &infrastructurev1a1.DeckhouseMachineSpecTemplate{
		VMClassName:          m.VMClassName,
		CPU:                  m.CPU,
		Memory:               m.Memory,
		AdditionalDisks:      m.AdditionalDisks,
		RootDiskSize:         m.RootDiskSize,
		RootDiskStorageClass: m.RootDiskStorageClass,
		BootDiskImageRef:     m.BootDiskImageRef,
		Bootloader:           m.Bootloader,
		RunPolicy:            m.RunPolicy,
		LiveMigrationPolicy:  m.LiveMigrationPolicy,
	}
}

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

	var currentMachine, desiredMachine infrastructurev1a1.DeckhouseMachine
	if err := json.Unmarshal(req.Current.InfrastructureMachine, &currentMachine); err != nil {
		e.log.Error(err, "failed to unmarshal current InfrastructureMachine")
		writeError(w, http.StatusBadRequest, "failed to unmarshal current machine: "+err.Error())
		return
	}
	if err := json.Unmarshal(req.Desired.InfrastructureMachine, &desiredMachine); err != nil {
		e.log.Error(err, "failed to unmarshal desired InfrastructureMachine")
		writeError(w, http.StatusBadRequest, "failed to unmarshal desired machine: "+err.Error())
		return
	}

	oldSpec := specFromMachine(&currentMachine.Spec)
	newSpec := specFromMachine(&desiredMachine.Spec)

	canUpdate, message := canUpdateInPlace(oldSpec, newSpec)

	resp := CanUpdateMachineResponse{
		CommonResponse: CommonResponse{Status: "Success", Message: message},
	}

	if canUpdate {
		patchBytes, err := jsonpatch.CreateMergePatch(
			req.Current.InfrastructureMachine,
			req.Desired.InfrastructureMachine,
		)
		if err != nil {
			e.log.Error(err, "failed to compute merge patch for InfrastructureMachine")
			writeError(w, http.StatusInternalServerError, "failed to compute patch: "+err.Error())
			return
		}
		resp.InfrastructureMachinePatch = &Patch{
			PatchType: JSONMergePatchType,
			Patch:     patchBytes,
		}
	}

	e.log.Info("CanUpdateMachine response",
		"machine", currentMachine.Name,
		"canUpdate", canUpdate,
		"message", message,
	)
	writeJSON(w, http.StatusOK, resp)
}
