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
	"net/http"
)

const runtimeHookAPIVersion = "hooks.runtime.cluster.x-k8s.io/v1alpha1"

// HandleDiscovery responds to the CAPI Runtime SDK discovery request.
// CAPI calls this at startup to learn which hooks this extension serves.
func (e *Extension) HandleDiscovery(w http.ResponseWriter, r *http.Request) {
	e.log.Info("Discovery request received")

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := DiscoveryResponse{
		Status: "Success",
		Handlers: []Handler{
			{
				Name: HandlerNameCanUpdateMachineSet,
				RequestHook: RequestHook{
					APIVersion: runtimeHookAPIVersion,
					Hook:       "CanUpdateMachineSet",
				},
				TimeoutSeconds: 10,
				FailurePolicy:  "Fail",
			},
			{
				Name: HandlerNameCanUpdateMachine,
				RequestHook: RequestHook{
					APIVersion: runtimeHookAPIVersion,
					Hook:       "CanUpdateMachine",
				},
				TimeoutSeconds: 10,
				FailurePolicy:  "Fail",
			},
			{
				Name: HandlerNameUpdateMachine,
				RequestHook: RequestHook{
					APIVersion: runtimeHookAPIVersion,
					Hook:       "UpdateMachine",
				},
				TimeoutSeconds: 30,
				FailurePolicy:  "Fail",
			},
		},
	}

	writeJSON(w, http.StatusOK, resp)
}
