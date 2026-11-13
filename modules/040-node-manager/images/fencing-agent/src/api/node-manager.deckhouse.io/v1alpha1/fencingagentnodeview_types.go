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
	NodeName string `json:"nodeName"`
}

// FencingAgentKubernetesAPIView is how the agent sees the Kubernetes API
// (the control plane). The agent reads it every few hundred milliseconds, so
// the timestamps carry microseconds.
type FencingAgentKubernetesAPIView struct {
	// CPSource is where the agent takes the control plane state from: "proxy"
	// is the local kubernetes-api-proxy, "self" is the own probes of the agent.
	// Empty until the first read.
	CPSource string `json:"cpSource"`
	// LastReadAt is the time of the last read; nil until the first one.
	LastReadAt *metav1.MicroTime `json:"lastReadAt"`
	// LastReadOK is the result of the last read: at least one API server was ready.
	LastReadOK bool `json:"lastReadOK"`
	// LastVisibleAt is the time of the last read after which the control plane
	// was visible; nil until then.
	LastVisibleAt *metav1.MicroTime `json:"lastVisibleAt"`
}

// FencingAgentNodeViewStatus is the local view of the agent. The section is a value, not a pointer.
type FencingAgentNodeViewStatus struct {
	KubernetesAPI FencingAgentKubernetesAPIView `json:"kubernetesAPI"`
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
