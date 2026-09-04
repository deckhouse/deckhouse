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
	"k8s.io/apimachinery/pkg/runtime"
)

// FencingAgentEventType is the kind of membership transition reported.
// +kubebuilder:validation:Enum=Join;Suspect;Dead;Left;Alive;Recovered
type FencingAgentEventType string

const (
	FencingAgentEventJoin      FencingAgentEventType = "Join"
	FencingAgentEventSuspect   FencingAgentEventType = "Suspect"
	FencingAgentEventDead      FencingAgentEventType = "Dead"
	FencingAgentEventLeft      FencingAgentEventType = "Left"
	FencingAgentEventAlive     FencingAgentEventType = "Alive"
	FencingAgentEventRecovered FencingAgentEventType = "Recovered"
)

// FencingAgentEventSpec carries the whole event; this kind has no status.
type FencingAgentEventSpec struct {
	NodeName  string                `json:"nodeName"`
	NodeGroup string                `json:"nodeGroup"`
	EventType FencingAgentEventType `json:"eventType"`
	EventTime metav1.Time           `json:"eventTime"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

// FencingAgentEvent is a read-only record of one membership transition, served over the local socket.
// +kubebuilder:object:root=true
type FencingAgentEvent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FencingAgentEventSpec `json:"spec"`
}

// FencingAgentEventList contains a list of FencingAgentEvent.
// +kubebuilder:object:root=true
type FencingAgentEventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []FencingAgentEvent `json:"items"`
}

var (
	_ runtime.Object = (*FencingAgentEvent)(nil)
	_ runtime.Object = (*FencingAgentEventList)(nil)
)

func init() {
	SchemeBuilder.Register(&FencingAgentEvent{}, &FencingAgentEventList{})
}
