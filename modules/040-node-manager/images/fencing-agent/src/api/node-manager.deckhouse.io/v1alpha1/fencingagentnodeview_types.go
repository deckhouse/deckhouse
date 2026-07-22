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

// FencingAgentNodeViewSpec is the identity of the local Node.
type FencingAgentNodeViewSpec struct {
	NodeName   string     `json:"nodeName"`
	NodeUID    string     `json:"nodeUID"`
	NodeGroup  string     `json:"nodeGroup"`
	ProfileRef ProfileRef `json:"profileRef"`
}

// FencingAgentQuorumView is the local quorum picture.
type FencingAgentQuorumView struct {
	QuorumSize   int32        `json:"quorumSize"`
	AliveCount   int32        `json:"aliveCount"`
	HasQuorum    bool         `json:"hasQuorum"`
	QuorumLostAt *metav1.Time `json:"quorumLostAt"`
}

// FencingAgentWatchdogView is the state of the local watchdog feed.
type FencingAgentWatchdogView struct {
	FeedActive bool         `json:"feedActive"`
	LastFeedAt *metav1.Time `json:"lastFeedAt"`
	StopReason string       `json:"stopReason"`
}

// FencingAgentFallbackView is the local view of fallback mode.
type FencingAgentFallbackView struct {
	Active          bool         `json:"active"`
	APIReachable    bool         `json:"apiReachable"`
	LastHeartbeatAt *metav1.Time `json:"lastHeartbeatAt"`
}

// FencingAgentClusterStateRef points at the FencingNodeState for this Node.
type FencingAgentClusterStateRef struct {
	Name  string                `json:"name"`
	Phase FencingNodeStatePhase `json:"phase"`
}

// FencingAgentNodeViewStatus is the agent's local view; sections are values, not pointers.
type FencingAgentNodeViewStatus struct {
	Quorum          FencingAgentQuorumView      `json:"quorum"`
	Watchdog        FencingAgentWatchdogView    `json:"watchdog"`
	Fallback        FencingAgentFallbackView    `json:"fallback"`
	ClusterStateRef FencingAgentClusterStateRef `json:"clusterStateRef"`
}

// FencingAgentNodeView is a read-only projection of the agent's own Node, served over the local socket.
// +kubebuilder:object:root=true
type FencingAgentNodeView struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FencingAgentNodeViewSpec   `json:"spec"`
	Status FencingAgentNodeViewStatus `json:"status,omitempty"`
}

// FencingAgentNodeViewList contains a list of FencingAgentNodeView.
// +kubebuilder:object:root=true
type FencingAgentNodeViewList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []FencingAgentNodeView `json:"items"`
}

var (
	_ runtime.Object = (*FencingAgentNodeView)(nil)
	_ runtime.Object = (*FencingAgentNodeViewList)(nil)
)

func init() {
	SchemeBuilder.Register(&FencingAgentNodeView{}, &FencingAgentNodeViewList{})
}
