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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CAPI Runtime SDK wire types.
// See https://cluster-api.sigs.k8s.io/tasks/experimental-features/runtime-sdk/

// DiscoveryResponse is returned by the Discovery endpoint.
type DiscoveryResponse struct {
	metav1.TypeMeta `json:",inline"`
	Status          string    `json:"status"`
	Handlers        []Handler `json:"handlers"`
}

// Handler describes a single hook the extension serves.
type Handler struct {
	Name           string      `json:"name"`
	RequestHook    RequestHook `json:"requestHook"`
	TimeoutSeconds int         `json:"timeoutSeconds,omitempty"`
	FailurePolicy  string      `json:"failurePolicy,omitempty"`
}

// RequestHook identifies the hook by apiVersion + hook name.
type RequestHook struct {
	APIVersion string `json:"apiVersion"`
	Hook       string `json:"hook"`
}

// CanUpdateMachineSetRequest is sent by CAPI MachineDeployment controller
// as a fast pre-check before iterating individual Machines.
type CanUpdateMachineSetRequest struct {
	metav1.TypeMeta `json:",inline"`
	CommonRequest   `json:",inline"`
	MachineSet      MachineSetRef `json:"machineSet"`
	OldMachineSet   MachineSetRef `json:"oldMachineSet"`
}

// CanUpdateMachineSetResponse tells CAPI whether the whole MachineSet can
// potentially be updated in-place.
type CanUpdateMachineSetResponse struct {
	metav1.TypeMeta `json:",inline"`
	Status          string `json:"status"`
	CanUpdate       bool   `json:"canUpdate"`
	Message         string `json:"message,omitempty"`
}

// MachineSetRef references a CAPI MachineSet and its infrastructure template.
type MachineSetRef struct {
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	Spec      MachineSetSpec `json:"spec"`
}

// MachineSetSpec carries the infrastructure template reference.
type MachineSetSpec struct {
	InfrastructureRef ObjectRef `json:"infrastructureRef"`
}

// CanUpdateMachineRequest is sent by CAPI to check whether in-place update is possible.
type CanUpdateMachineRequest struct {
	metav1.TypeMeta `json:",inline"`
	CommonRequest   `json:",inline"`
	Machine         MachineRef `json:"machine"`
	OldMachine      MachineRef `json:"oldMachine"`
}

// CanUpdateMachineResponse tells CAPI whether in-place update is possible.
type CanUpdateMachineResponse struct {
	metav1.TypeMeta `json:",inline"`
	Status          string `json:"status"`
	CanUpdate       bool   `json:"canUpdate"`
	Message         string `json:"message,omitempty"`
}

// UpdateMachineRequest is sent by CAPI to perform the actual in-place update.
type UpdateMachineRequest struct {
	metav1.TypeMeta `json:",inline"`
	CommonRequest   `json:",inline"`
	Machine         MachineRef `json:"machine"`
}

// UpdateMachineResponse reports the result of in-place update.
type UpdateMachineResponse struct {
	metav1.TypeMeta `json:",inline"`
	Status          string `json:"status"`
	Message         string `json:"message,omitempty"`
}

// CommonRequest contains fields shared by all hook requests.
type CommonRequest struct {
	Settings runtime.RawExtension `json:"settings,omitempty"`
}

// MachineRef references a CAPI Machine and its infrastructure object.
type MachineRef struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Spec      MachineSpec `json:"spec"`
}

// MachineSpec carries the infrastructure reference.
type MachineSpec struct {
	InfrastructureRef ObjectRef `json:"infrastructureRef"`
}

// ObjectRef is a typed reference to a Kubernetes object.
type ObjectRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
}
