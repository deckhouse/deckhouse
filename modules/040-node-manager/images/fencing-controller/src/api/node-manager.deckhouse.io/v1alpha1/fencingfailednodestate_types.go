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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ProfileName identifies an SLA profile; NodeGroup spec.fencing.mode carries it.
// +kubebuilder:validation:Enum=Critical;Normal;Slow
type ProfileName string

const (
	ProfileCritical ProfileName = "Critical"
	ProfileNormal   ProfileName = "Normal"
	ProfileSlow     ProfileName = "Slow"
)

// ObjectName returns the metadata.name of the FencingSLAProfile this profile refers to.
func (p ProfileName) ObjectName() string {
	return strings.ToLower(string(p))
}

// ProfileNames returns every valid profile, strictest first.
func ProfileNames() []ProfileName {
	return []ProfileName{ProfileCritical, ProfileNormal, ProfileSlow}
}

// FencingFailedNodeStatePhase is set by fencing-controller only. An empty phase
// means the controller did not process the object yet: read it as Suspected.
// +kubebuilder:validation:Enum=Suspected;FallbackAlive;Proven;Unproven;Queued;Evicting;Evicted
type FencingFailedNodeStatePhase string

const (
	PhaseSuspected     FencingFailedNodeStatePhase = "Suspected"
	PhaseFallbackAlive FencingFailedNodeStatePhase = "FallbackAlive"
	PhaseProven        FencingFailedNodeStatePhase = "Proven"
	PhaseUnproven      FencingFailedNodeStatePhase = "Unproven"
	PhaseQueued        FencingFailedNodeStatePhase = "Queued"
	PhaseEvicting      FencingFailedNodeStatePhase = "Evicting"
	PhaseEvicted       FencingFailedNodeStatePhase = "Evicted"
)

// FencingFailedNodeStateSource tells who created the object.
// +kubebuilder:validation:Enum=MemberlistDead;DeclareSuicide;NodeLeaseStale
type FencingFailedNodeStateSource string

const (
	SourceMemberlistDead FencingFailedNodeStateSource = "MemberlistDead"
	SourceDeclareSuicide FencingFailedNodeStateSource = "DeclareSuicide"
	SourceNodeLeaseStale FencingFailedNodeStateSource = "NodeLeaseStale"
)

type ProfileRef struct {
	Name ProfileName `json:"name"`
}

// FencingFailedNodeStateSpec is immutable after creation (CEL self == oldSelf).
type FencingFailedNodeStateSpec struct {
	Source     FencingFailedNodeStateSource `json:"source"`
	DetectedBy string                       `json:"detectedBy"`
	// DetectedAt is for reference only. It has microsecond precision.
	DetectedAt metav1.MicroTime `json:"detectedAt"`
	BootID     string           `json:"bootID"`
	Armed      bool             `json:"armed"`
	// ProfileRef is set if and only if Armed is true (CEL rule).
	// +optional
	ProfileRef *ProfileRef `json:"profileRef,omitempty"`
}

// FencingFailedNodeStateAttestation is written by the agent of the witness Node.
type FencingFailedNodeStateAttestation struct {
	Witness string           `json:"witness"`
	At      metav1.MicroTime `json:"at"`
}

// FencingFailedNodeStateContact is written by the agent that received a report
// of a contact with the failed Node.
type FencingFailedNodeStateContact struct {
	At         metav1.MicroTime `json:"at"`
	ReportedBy string           `json:"reportedBy"`
}

// FencingFailedNodeStateFallback is written by the affected Node only, and only
// through the API, so the section existing at all says the Node reached the API.
type FencingFailedNodeStateFallback struct {
	LastHeartbeatAt *metav1.MicroTime `json:"lastHeartbeatAt"`
}

// FencingFailedNodeStateCorrelation is written by fencing-controller.
type FencingFailedNodeStateCorrelation struct {
	// +optional
	Episode string `json:"episode,omitempty"`
}

// FencingFailedNodeStateStatus holds the incident state. Each section has one
// owner. The sections are pointers, so a writer never serializes a section it
// does not own.
type FencingFailedNodeStateStatus struct {
	// +optional
	Phase FencingFailedNodeStatePhase `json:"phase,omitempty"`
	// +optional
	Attestation *FencingFailedNodeStateAttestation `json:"attestation,omitempty"`
	// +optional
	Contact *FencingFailedNodeStateContact `json:"contact,omitempty"`
	// +optional
	Fallback *FencingFailedNodeStateFallback `json:"fallback,omitempty"`
	// +optional
	Correlation *FencingFailedNodeStateCorrelation `json:"correlation,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// FencingFailedNodeState stores an active fencing signal for a Node.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=ffns
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Phase,type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name=Source,type=string,JSONPath=`.spec.source`
// +kubebuilder:printcolumn:name=Armed,type=boolean,JSONPath=`.spec.armed`
// +kubebuilder:printcolumn:name=Profile,type=string,JSONPath=`.spec.profileRef.name`
// +kubebuilder:printcolumn:name=Witness,type=string,JSONPath=`.status.attestation.witness`
// +kubebuilder:printcolumn:name=DetectedAt,type=date,JSONPath=`.spec.detectedAt`
// +kubebuilder:printcolumn:name=Heartbeat,type=date,JSONPath=`.status.fallback.lastHeartbeatAt`
type FencingFailedNodeState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FencingFailedNodeStateSpec   `json:"spec"`
	Status FencingFailedNodeStateStatus `json:"status,omitempty"`
}

// FencingFailedNodeStateList contains a list of FencingFailedNodeState.
// +kubebuilder:object:root=true
type FencingFailedNodeStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []FencingFailedNodeState `json:"items"`
}

var (
	_ runtime.Object = (*FencingFailedNodeState)(nil)
	_ runtime.Object = (*FencingFailedNodeStateList)(nil)
)

func init() {
	SchemeBuilder.Register(&FencingFailedNodeState{}, &FencingFailedNodeStateList{})
}
