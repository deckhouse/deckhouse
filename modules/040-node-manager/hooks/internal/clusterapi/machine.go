/*
Copyright 2023 Flant JSC

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

package clusterapi

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	capi "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/capi/v1beta1"
)

type Machine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineSpec   `json:"spec,omitempty"`
	Status MachineStatus `json:"status,omitempty"`
}

type MachineSpec struct{}

type MachineStatus struct {
	NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

	LastUpdated *metav1.Time             `json:"lastUpdated,omitempty"`
	Phase       string                   `json:"phase,omitempty"`
	Deletion    *MachineDeletionStatus   `json:"deletion,omitempty"`
	Deprecated  *MachineStatusDeprecated `json:"deprecated,omitempty"`
	// FailureReason will be set in the event that there is a terminal problem reconciling the Machine.
	FailureReason *string `json:"failureReason,omitempty"`
	// FailureMessage will be set in the event that there is a terminal problem reconciling the Machine.
	FailureMessage *string `json:"failureMessage,omitempty"`
	// Conditions defines current service state of the Machine.
	Conditions capi.Conditions `json:"conditions,omitempty"`
}

type MachineDeletionStatus struct {
	NodeDrainStartTime               *metav1.Time `json:"nodeDrainStartTime,omitempty"`
	WaitForNodeVolumeDetachStartTime *metav1.Time `json:"waitForNodeVolumeDetachStartTime,omitempty"`
}

type MachineStatusDeprecated struct {
	V1Beta1 *MachineStatusDeprecatedV1Beta1 `json:"v1beta1,omitempty"`
}

type MachineStatusDeprecatedV1Beta1 struct {
	Conditions capi.Conditions `json:"conditions,omitempty"`
}
