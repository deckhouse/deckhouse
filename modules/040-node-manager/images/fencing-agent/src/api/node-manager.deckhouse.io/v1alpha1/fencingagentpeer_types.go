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

// PeerState is the memberlist state of a peer as seen from this Node.
// +kubebuilder:validation:Enum=alive;suspect;dead;left
type PeerState string

const (
	PeerStateAlive   PeerState = "alive"
	PeerStateSuspect PeerState = "suspect"
	PeerStateDead    PeerState = "dead"
	PeerStateLeft    PeerState = "left"
)

type FencingAgentPeerSpec struct {
	NodeName  string `json:"nodeName"`
	NodeGroup string `json:"nodeGroup"`
}

type FencingAgentPeerAddress struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

type FencingAgentPeerStatus struct {
	// +optional
	State PeerState `json:"state,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=type
	Addresses []FencingAgentPeerAddress `json:"addresses,omitempty"`
	// +optional
	LastSeenAt *metav1.Time `json:"lastSeenAt,omitempty"`
	// +optional
	MemberlistIncarnation int64 `json:"memberlistIncarnation,omitempty"`
	// +optional
	MemberlistViewID string `json:"memberlistViewID,omitempty"`
}

// FencingAgentPeer is a read-only projection of one memberlist peer, served over the local socket.
// +kubebuilder:object:root=true
type FencingAgentPeer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FencingAgentPeerSpec   `json:"spec"`
	Status FencingAgentPeerStatus `json:"status,omitempty"`
}

// FencingAgentPeerList contains a list of FencingAgentPeer.
// +kubebuilder:object:root=true
type FencingAgentPeerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []FencingAgentPeer `json:"items"`
}

var (
	_ runtime.Object = (*FencingAgentPeer)(nil)
	_ runtime.Object = (*FencingAgentPeerList)(nil)
)

func init() {
	SchemeBuilder.Register(&FencingAgentPeer{}, &FencingAgentPeerList{})
}
