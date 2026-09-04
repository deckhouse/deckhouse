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

// FencingSLAProfileSpec mirrors crds/fencingslaprofiles.yaml. metav1.Duration
// keeps the wire format the string the CRD pattern validates, while consumers get
// a time.Duration. The schema and CEL rules only guard admission and an object
// may predate them, so Validate re-checks every value before the agent runs.
type FencingSLAProfileSpec struct {
	// ReactionGoal is documentation-only and is not parsed as a duration.
	ReactionGoal string `json:"reactionGoal"`
	// DetectionWindowTarget is documentation-only and is not parsed as a duration.
	DetectionWindowTarget string `json:"detectionWindowTarget"`

	Memberlist FencingSLAProfileMemberlist `json:"memberlist"`
	Fallback   FencingSLAProfileFallback   `json:"fallback"`
	Rejoin     FencingSLAProfileRejoin     `json:"rejoin"`
	Evacuation FencingSLAProfileEvacuation `json:"evacuation"`
	Watchdog   FencingSLAProfileWatchdog   `json:"watchdog"`
}

// FencingSLAProfileMemberlist controls peer probing, suspicion and gossip.
type FencingSLAProfileMemberlist struct {
	ProbeInterval metav1.Duration `json:"probeInterval"`
	ProbeTimeout  metav1.Duration `json:"probeTimeout"`
	// +kubebuilder:validation:Minimum=1
	SuspicionMult int32 `json:"suspicionMult"`
	// +kubebuilder:validation:Minimum=1
	SuspicionMaxTimeoutMult int32 `json:"suspicionMaxTimeoutMult"`
	// +kubebuilder:validation:Minimum=1
	IndirectChecks int32 `json:"indirectChecks"`
	// +kubebuilder:validation:Minimum=1
	AwarenessMaxMultiplier int32           `json:"awarenessMaxMultiplier"`
	GossipInterval         metav1.Duration `json:"gossipInterval"`
	// +kubebuilder:validation:Minimum=1
	RetransmitMult      int32           `json:"retransmitMult"`
	GossipToTheDeadTime metav1.Duration `json:"gossipToTheDeadTime"`
}

// FencingSLAProfileFallback tunes the heartbeat of a node that lost gossip quorum
// but still reaches the API. TTL belongs to the controller.
type FencingSLAProfileFallback struct {
	Heartbeat            metav1.Duration `json:"heartbeat"`
	TTL                  metav1.Duration `json:"ttl"`
	KubernetesAPITimeout metav1.Duration `json:"kubernetesAPITimeout"`
}

// FencingSLAProfileRejoin paces the rejoin loop after quorum loss.
type FencingSLAProfileRejoin struct {
	Interval    metav1.Duration `json:"interval"`
	MaxInterval metav1.Duration `json:"maxInterval"`
}

// FencingSLAProfileEvacuation belongs to the controller, not the agent.
type FencingSLAProfileEvacuation struct {
	Delay metav1.Duration `json:"delay"`
}

// FencingSLAProfileWatchdog tunes the watchdog safety net on the node.
type FencingSLAProfileWatchdog struct {
	FeedInterval metav1.Duration `json:"feedInterval"`
	Timeout      metav1.Duration `json:"timeout"`
}

// FencingSLAProfile is a cluster-scoped, stateless set of fencing timings. The
// built-in objects (critical/medium/moderate/slow) ship with the module Helm
// templates; ProfileName.ObjectName resolves a reference to an object name.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=fsp
type FencingSLAProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FencingSLAProfileSpec `json:"spec"`
}

// FencingSLAProfileList contains a list of FencingSLAProfile.
// +kubebuilder:object:root=true
type FencingSLAProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []FencingSLAProfile `json:"items"`
}

var (
	_ runtime.Object = (*FencingSLAProfile)(nil)
	_ runtime.Object = (*FencingSLAProfileList)(nil)
)

func init() {
	SchemeBuilder.Register(&FencingSLAProfile{}, &FencingSLAProfileList{})
}
