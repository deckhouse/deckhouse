/*
Copyright (c) 2024 SAP SE or an SAP affiliate company and Gardener contributors

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MachineTemplateSpec describes the data a machine should have when created from a template.
type MachineTemplateSpec struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec MachineSpec `json:"spec,omitempty"`
}

// MachineConfiguration describes configurations useful for machine-controller.
type MachineConfiguration struct {
	// +optional
	MachineDrainTimeout *metav1.Duration `json:"drainTimeout,omitempty"`
	// +optional
	MachineHealthTimeout *metav1.Duration `json:"healthTimeout,omitempty"`
	// +optional
	MachineCreationTimeout *metav1.Duration `json:"creationTimeout,omitempty"`
	// +optional
	MachineInPlaceUpdateTimeout *metav1.Duration `json:"inPlaceUpdateTimeout,omitempty"`
	// +optional
	DisableHealthTimeout *bool `json:"disableHealthTimeout,omitempty"`
	// +optional
	MaxEvictRetries *int32 `json:"maxEvictRetries,omitempty"`
	// +optional
	NodeConditions *string `json:"nodeConditions,omitempty"`
}

// MachineSummary stores a machine summary.
type MachineSummary struct {
	Name string `json:"name,omitempty"`
	// +optional
	ProviderID string `json:"providerID,omitempty"`
	// +optional
	LastOperation LastOperation `json:"lastOperation,omitempty"`
	// +optional
	OwnerRef string `json:"ownerRef,omitempty"`
}
