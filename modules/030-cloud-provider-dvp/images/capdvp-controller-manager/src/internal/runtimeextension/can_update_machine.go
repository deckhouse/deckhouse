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

// updateStrategy describes what level of disruption is needed to apply changes.
type updateStrategy int

const (
	updateNone    updateStrategy = iota
	updateHot                    // No VM restart: disk hot-plug, policy patch
	updateWarm                   // Stop → patch spec → start
	updateRecreate               // Full VM replacement required
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
		if cs.strategy < updateWarm {
			cs.strategy = updateWarm
			cs.reason = "root disk size increased — warm update (stop → resize → start)"
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
