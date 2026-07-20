/*
Copyright 2026 Flant JSC

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

type FencingNodeStatePhase string

const (
	PhaseHealthy       FencingNodeStatePhase = "Healthy"
	PhaseSuspected     FencingNodeStatePhase = "Suspected"
	PhaseFallbackAlive FencingNodeStatePhase = "FallbackAlive"
	PhaseReadyToEvict  FencingNodeStatePhase = "ReadyToEvict"
	PhaseEvicting      FencingNodeStatePhase = "Evicting"
	PhaseDone          FencingNodeStatePhase = "Done"
	PhaseError         FencingNodeStatePhase = "Error"
)

type ProfileRef struct {
	Name string `json:"name"`
}

type FencingNodeStateSpec struct {
	NodeGroup  string     `json:"nodeGroup"`
	ProfileRef ProfileRef `json:"profileRef"`
}

// FencingNodeStateStatus holds the incident state.
type FencingNodeStateStatus struct {
	Phase              FencingNodeStatePhase `json:"phase,omitempty"`
	ObservedGeneration int64                 `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition    `json:"conditions,omitempty"`
}

// FencingNodeState is a cluster-scoped signal object for fencing-controller.
// One CR corresponds to one Node and exists only while there is an active
// problem with that Node.
// +kubebuilder:object:root=true
type FencingNodeState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FencingNodeStateSpec   `json:"spec"`
	Status FencingNodeStateStatus `json:"status,omitempty"`
}

// FencingNodeStateList contains a list of FencingNodeState.
// +kubebuilder:object:root=true
type FencingNodeStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []FencingNodeState `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FencingNodeState{}, &FencingNodeStateList{})
}
