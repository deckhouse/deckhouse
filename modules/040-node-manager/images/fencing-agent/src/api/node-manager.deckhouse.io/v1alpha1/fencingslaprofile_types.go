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

// FencingSLAProfileSpec mirrors crds/fencingslaprofiles.yaml. Durations stay
// strings, as on the wire: the CRD pattern validates the format only, so the
// agent parses and fully re-validates them when building its runtime
// configuration (fail closed).
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
	ProbeInterval string `json:"probeInterval"`
	ProbeTimeout  string `json:"probeTimeout"`
	// +kubebuilder:validation:Minimum=1
	SuspicionMult int32 `json:"suspicionMult"`
	// +kubebuilder:validation:Minimum=1
	SuspicionMaxTimeoutMult int32 `json:"suspicionMaxTimeoutMult"`
	// +kubebuilder:validation:Minimum=1
	IndirectChecks int32 `json:"indirectChecks"`
	// +kubebuilder:validation:Minimum=1
	AwarenessMaxMultiplier int32  `json:"awarenessMaxMultiplier"`
	GossipInterval         string `json:"gossipInterval"`
	// +kubebuilder:validation:Minimum=1
	RetransmitMult      int32  `json:"retransmitMult"`
	GossipToTheDeadTime string `json:"gossipToTheDeadTime"`
}

// FencingSLAProfileFallback tunes the heartbeat of a node that lost gossip
// quorum but still reaches the Kubernetes API. TTL is consumed by the
// controller, not by the agent.
type FencingSLAProfileFallback struct {
	Heartbeat            string `json:"heartbeat"`
	TTL                  string `json:"ttl"`
	KubernetesAPITimeout string `json:"kubernetesAPITimeout"`
}

// FencingSLAProfileRejoin paces the rejoin loop after quorum loss.
type FencingSLAProfileRejoin struct {
	Interval    string `json:"interval"`
	MaxInterval string `json:"maxInterval"`
}

// FencingSLAProfileEvacuation is consumed by the controller, not by the agent.
type FencingSLAProfileEvacuation struct {
	Delay string `json:"delay"`
}

// FencingSLAProfileWatchdog tunes the watchdog safety net on the node.
type FencingSLAProfileWatchdog struct {
	FeedInterval string `json:"feedInterval"`
	Timeout      string `json:"timeout"`
}

// FencingSLAProfile is a cluster-scoped, state-less set of fencing timings.
// The built-in objects (critical/medium/moderate/slow) are shipped by module
// Helm templates; ProfileName.ObjectName resolves a reference to an object name.
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
