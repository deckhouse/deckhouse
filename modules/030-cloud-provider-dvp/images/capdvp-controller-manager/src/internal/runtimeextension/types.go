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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CAPI Runtime SDK wire types (v1.12-compatible).
// See https://cluster-api.sigs.k8s.io/tasks/experimental-features/runtime-sdk/implement-in-place-update-hooks

// --- Discovery ---

type DiscoveryResponse struct {
	metav1.TypeMeta `json:",inline"`
	Status          string    `json:"status"`
	Handlers        []Handler `json:"handlers"`
}

type Handler struct {
	Name           string      `json:"name"`
	RequestHook    RequestHook `json:"requestHook"`
	TimeoutSeconds int         `json:"timeoutSeconds,omitempty"`
	FailurePolicy  string      `json:"failurePolicy,omitempty"`
}

type RequestHook struct {
	APIVersion string `json:"apiVersion"`
	Hook       string `json:"hook"`
}

// --- Common ---

type CommonRequest struct {
	Settings map[string]string `json:"settings,omitempty"`
}

type CommonResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type CommonRetryResponse struct {
	CommonResponse      `json:",inline"`
	RetryAfterSeconds int32 `json:"retryAfterSeconds"`
}

// --- Patch ---

type PatchType string

const (
	JSONPatchType      PatchType = "JSONPatch"
	JSONMergePatchType PatchType = "JSONMergePatch"
)

type Patch struct {
	PatchType PatchType `json:"patchType,omitempty"`
	Patch     []byte    `json:"patch,omitempty"`
}

func (p *Patch) IsDefined() bool {
	return p.PatchType != "" || len(p.Patch) > 0
}

// --- CanUpdateMachineSet ---

type CanUpdateMachineSetRequest struct {
	metav1.TypeMeta `json:",inline"`
	CommonRequest   `json:",inline"`
	Current         CanUpdateMachineSetRequestObjects `json:"current"`
	Desired         CanUpdateMachineSetRequestObjects `json:"desired"`
}

type CanUpdateMachineSetRequestObjects struct {
	MachineSet                      json.RawMessage `json:"machineSet"`
	InfrastructureMachineTemplate   json.RawMessage `json:"infrastructureMachineTemplate"`
	BootstrapConfigTemplate         json.RawMessage `json:"bootstrapConfigTemplate,omitempty"`
}

type CanUpdateMachineSetResponse struct {
	metav1.TypeMeta                   `json:",inline"`
	CommonResponse                    `json:",inline"`
	MachineSetPatch                   *Patch `json:"machineSetPatch,omitempty"`
	InfrastructureMachineTemplatePatch *Patch `json:"infrastructureMachineTemplatePatch,omitempty"`
	BootstrapConfigTemplatePatch      *Patch `json:"bootstrapConfigTemplatePatch,omitempty"`
}

// --- CanUpdateMachine ---

type CanUpdateMachineRequest struct {
	metav1.TypeMeta `json:",inline"`
	CommonRequest   `json:",inline"`
	Current         CanUpdateMachineRequestObjects `json:"current"`
	Desired         CanUpdateMachineRequestObjects `json:"desired"`
}

type CanUpdateMachineRequestObjects struct {
	Machine               json.RawMessage `json:"machine"`
	InfrastructureMachine json.RawMessage `json:"infrastructureMachine"`
	BootstrapConfig       json.RawMessage `json:"bootstrapConfig,omitempty"`
}

type CanUpdateMachineResponse struct {
	metav1.TypeMeta              `json:",inline"`
	CommonResponse               `json:",inline"`
	MachinePatch                 *Patch `json:"machinePatch,omitempty"`
	InfrastructureMachinePatch   *Patch `json:"infrastructureMachinePatch,omitempty"`
	BootstrapConfigPatch         *Patch `json:"bootstrapConfigPatch,omitempty"`
}

// --- UpdateMachine ---

type UpdateMachineRequest struct {
	metav1.TypeMeta `json:",inline"`
	CommonRequest   `json:",inline"`
	Desired         UpdateMachineRequestObjects `json:"desired"`
}

type UpdateMachineRequestObjects struct {
	Machine               json.RawMessage `json:"machine"`
	InfrastructureMachine json.RawMessage `json:"infrastructureMachine"`
	BootstrapConfig       json.RawMessage `json:"bootstrapConfig,omitempty"`
}

type UpdateMachineResponse struct {
	metav1.TypeMeta     `json:",inline"`
	CommonRetryResponse `json:",inline"`
}
