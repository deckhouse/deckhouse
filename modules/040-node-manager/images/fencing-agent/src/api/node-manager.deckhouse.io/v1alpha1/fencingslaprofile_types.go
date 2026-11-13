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
	// ControlPlaneLostAfter is the largest age of the last ready probe of the control plane.
	ControlPlaneLostAfter metav1.Duration `json:"controlPlaneLostAfter"`

	Memberlist FencingSLAProfileMemberlist `json:"memberlist"`
	Rejoin     FencingSLAProfileRejoin     `json:"rejoin"`
	Poll       FencingSLAProfilePoll       `json:"poll"`
	Watchdog   FencingSLAProfileWatchdog   `json:"watchdog"`
	Heartbeat  FencingSLAProfileHeartbeat  `json:"heartbeat"`
	Eviction   FencingSLAProfileEviction   `json:"eviction"`
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
	GossipNodes int32 `json:"gossipNodes"`
	// +kubebuilder:validation:Minimum=1
	RetransmitMult      int32           `json:"retransmitMult"`
	DeadNodeReclaimTime metav1.Duration `json:"deadNodeReclaimTime"`
	GossipToTheDeadTime metav1.Duration `json:"gossipToTheDeadTime"`
}

// FencingSLAProfileRejoin paces the join attempts: the startup join and the
// rejoin loop.
type FencingSLAProfileRejoin struct {
	Interval metav1.Duration `json:"interval"`
}

// FencingSLAProfilePoll holds the timings of the poll of the control plane
// state. The timeouts of the poll are common timings of the agent
// ConfigMap, not profile fields.
type FencingSLAProfilePoll struct {
	// WitnessFresherBy is how much fresher the probe of the answering node must be than the own probe.
	WitnessFresherBy metav1.Duration `json:"witnessFresherBy"`
}

// FencingSLAProfileWatchdog tunes the watchdog safety net on the node.
type FencingSLAProfileWatchdog struct {
	// Timeout is the timeout of the watchdog device.
	Timeout      metav1.Duration `json:"timeout"`
	FeedInterval metav1.Duration `json:"feedInterval"`
	Margin       metav1.Duration `json:"margin"`
}

// FencingSLAProfileHeartbeat tunes the heartbeat a node writes through the API to
// prove it is alive.
type FencingSLAProfileHeartbeat struct {
	// Interval is the interval between the heartbeat writes.
	Interval   metav1.Duration `json:"interval"`
	APITimeout metav1.Duration `json:"apiTimeout"`
}

// FencingSLAProfileEviction holds the timings of the creation of
// FencingFailedNodeState objects.
type FencingSLAProfileEviction struct {
	// WriterWaveInterval is the interval between waves of writers.
	WriterWaveInterval metav1.Duration `json:"writerWaveInterval"`
}

// FencingSLAProfile is a cluster-scoped, stateless set of fencing timings. The
// built-in objects (critical/normal/slow) ship with the module Helm templates;
// ProfileName.ObjectName resolves a reference to an object name.
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
