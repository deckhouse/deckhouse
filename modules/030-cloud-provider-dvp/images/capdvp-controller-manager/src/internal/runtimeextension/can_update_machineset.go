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

	jsonpatch "github.com/evanphx/json-patch/v5"

	infrastructurev1a1 "cluster-api-provider-dvp/api/v1alpha1"
)

// HandleCanUpdateMachineSet is called by the CAPI MachineDeployment controller
// as a fast pre-check. If this returns empty patches, CAPI falls back to rolling update.
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

	var currentTmpl, desiredTmpl infrastructurev1a1.DeckhouseMachineTemplate
	if err := json.Unmarshal(req.Current.InfrastructureMachineTemplate, &currentTmpl); err != nil {
		e.log.Error(err, "failed to unmarshal current InfrastructureMachineTemplate")
		writeError(w, http.StatusBadRequest, "failed to unmarshal current template: "+err.Error())
		return
	}
	if err := json.Unmarshal(req.Desired.InfrastructureMachineTemplate, &desiredTmpl); err != nil {
		e.log.Error(err, "failed to unmarshal desired InfrastructureMachineTemplate")
		writeError(w, http.StatusBadRequest, "failed to unmarshal desired template: "+err.Error())
		return
	}

	canUpdate, message := canUpdateInPlace(
		&currentTmpl.Spec.Template.Spec,
		&desiredTmpl.Spec.Template.Spec,
	)

	resp := CanUpdateMachineSetResponse{
		CommonResponse: CommonResponse{Status: "Success", Message: message},
	}

	if canUpdate {
		patchBytes, err := jsonpatch.CreateMergePatch(
			req.Current.InfrastructureMachineTemplate,
			req.Desired.InfrastructureMachineTemplate,
		)
		if err != nil {
			e.log.Error(err, "failed to compute merge patch for InfrastructureMachineTemplate")
			writeError(w, http.StatusInternalServerError, "failed to compute patch: "+err.Error())
			return
		}
		resp.InfrastructureMachineTemplatePatch = &Patch{
			PatchType: JSONMergePatchType,
			Patch:     patchBytes,
		}
	}

	e.log.Info("CanUpdateMachineSet response",
		"canUpdate", canUpdate,
		"message", message,
	)
	writeJSON(w, http.StatusOK, resp)
}
