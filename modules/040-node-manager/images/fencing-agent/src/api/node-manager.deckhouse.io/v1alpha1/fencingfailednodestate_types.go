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

// ProfileName selects the SLA timings a NodeGroup is fenced under.
// +kubebuilder:validation:Enum=Critical;Medium;Moderate;Slow
type ProfileName string

const (
	ProfileCritical ProfileName = "Critical"
	ProfileMedium   ProfileName = "Medium"
	ProfileModerate ProfileName = "Moderate"
	ProfileSlow     ProfileName = "Slow"
)

// ProfileNames returns every valid profile, strictest first.
func ProfileNames() []ProfileName {
	return []ProfileName{ProfileCritical, ProfileMedium, ProfileModerate, ProfileSlow}
}

// +kubebuilder:validation:Enum=Healthy;Suspected;FallbackAlive;ReadyToEvict;Evicting;Done;Error
type FencingFailedNodeStatePhase string

const (
	// PhaseHealthy is unreachable: a healthy Node has no object.
	PhaseHealthy       FencingFailedNodeStatePhase = "Healthy"
	PhaseSuspected     FencingFailedNodeStatePhase = "Suspected"
	PhaseFallbackAlive FencingFailedNodeStatePhase = "FallbackAlive"
	PhaseReadyToEvict  FencingFailedNodeStatePhase = "ReadyToEvict"
	PhaseEvicting      FencingFailedNodeStatePhase = "Evicting"
	PhaseDone          FencingFailedNodeStatePhase = "Done"
	PhaseError         FencingFailedNodeStatePhase = "Error"
)

// FailedReason is why the designated writer considers a peer failed.
// +kubebuilder:validation:Enum=MemberlistDead;MemberlistLeft;QuorumLost
type FailedReason string

const (
	FailedReasonMemberlistDead FailedReason = "MemberlistDead"
	FailedReasonMemberlistLeft FailedReason = "MemberlistLeft"
	FailedReasonQuorumLost     FailedReason = "QuorumLost"
)

type ProfileRef struct {
	Name ProfileName `json:"name"`
}

// FencingFailedNodeStateSpec is immutable after creation (CEL self == oldSelf).
type FencingFailedNodeStateSpec struct {
	NodeGroup  string     `json:"nodeGroup"`
	ProfileRef ProfileRef `json:"profileRef"`
}

// FencingFailedNodeStateFailed is written only by the designated writer.
type FencingFailedNodeStateFailed struct {
	DetectedAt metav1.Time  `json:"detectedAt"`
	DetectedBy string       `json:"detectedBy"`
	Reason     FailedReason `json:"reason"`
	// +optional
	MemberlistIncarnation int64 `json:"memberlistIncarnation,omitempty"`
	// +optional
	MemberlistViewID string `json:"memberlistViewID,omitempty"`
	// +kubebuilder:validation:Minimum=0
	AliveCount int32 `json:"aliveCount"`
	// +kubebuilder:validation:Minimum=1
	QuorumSize int32 `json:"quorumSize"`
}

// FencingFailedNodeStateFallback is written only by the affected Node itself.
type FencingFailedNodeStateFallback struct {
	Active bool `json:"active"`
	// +optional
	LastHeartbeatAt *metav1.Time `json:"lastHeartbeatAt,omitempty"`
	// +optional
	QuorumLostAt *metav1.Time `json:"quorumLostAt,omitempty"`
	// +kubebuilder:validation:Minimum=1
	HeartbeatIntervalSeconds int32 `json:"heartbeatIntervalSeconds"`
}

// FencingFailedNodeStateStatus holds the incident state.
type FencingFailedNodeStateStatus struct {
	// +optional
	Phase FencingFailedNodeStatePhase `json:"phase,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Failed and Fallback are pointers so a writer never serializes a section it does not own.
	// +optional
	Failed *FencingFailedNodeStateFailed `json:"failed,omitempty"`
	// +optional
	Fallback *FencingFailedNodeStateFallback `json:"fallback,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// FencingFailedNodeState is a cluster-scoped signal object for fencing-controller,
// one per Node, named after the Node and owned by it. Status is written via
// the /status subresource.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=ffns
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Node,type=string,JSONPath=`.metadata.name`
// +kubebuilder:printcolumn:name=NodeGroup,type=string,JSONPath=`.spec.nodeGroup`
// +kubebuilder:printcolumn:name=Profile,type=string,JSONPath=`.spec.profileRef.name`
// +kubebuilder:printcolumn:name=Phase,type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name=FailedAt,type=date,JSONPath=`.status.failed.detectedAt`
// +kubebuilder:printcolumn:name=FallbackAt,type=date,JSONPath=`.status.fallback.lastHeartbeatAt`
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
