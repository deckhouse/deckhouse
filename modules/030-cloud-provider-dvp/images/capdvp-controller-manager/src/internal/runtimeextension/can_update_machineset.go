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
)

// HandleCanUpdateMachineSet is called by the CAPI MachineDeployment controller
// as a fast pre-check before it evaluates individual Machines.
// If this returns false, CAPI immediately falls back to rolling update without
// calling CanUpdateMachine for each Machine.
//
// The logic mirrors canUpdateInPlace: we compare old and new templates at the
// MachineSet level, which carry the same infrastructure template references.
func (e *Extension) HandleCanUpdateMachineSet(w http.ResponseWriter, r *http.Request) {
	e.log.Info("CanUpdateMachineSet request received")

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CanUpdateMachineSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ctx := context.Background()

	oldTemplate, err := e.getMachineTemplate(ctx, req.OldMachineSet.Spec.InfrastructureRef)
	if err != nil {
		e.log.Error(err, "failed to get old DeckhouseMachineTemplate")
		writeError(w, http.StatusInternalServerError, "failed to get old template: "+err.Error())
		return
	}

	newTemplate, err := e.getMachineTemplate(ctx, req.MachineSet.Spec.InfrastructureRef)
	if err != nil {
		e.log.Error(err, "failed to get new DeckhouseMachineTemplate")
		writeError(w, http.StatusInternalServerError, "failed to get new template: "+err.Error())
		return
	}

	canUpdate, message := canUpdateInPlace(
		&oldTemplate.Spec.Template.Spec,
		&newTemplate.Spec.Template.Spec,
	)

	resp := CanUpdateMachineSetResponse{
		Status:    "Success",
		CanUpdate: canUpdate,
		Message:   message,
	}

	e.log.Info("CanUpdateMachineSet response",
		"machineSet", req.MachineSet.Name,
		"canUpdate", canUpdate,
		"message", message,
	)
	writeJSON(w, http.StatusOK, resp)
}
