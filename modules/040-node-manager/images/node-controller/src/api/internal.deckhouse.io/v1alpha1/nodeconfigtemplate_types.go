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

// NodeConfigTemplate is what an operator takes to add a machine by hand. It is
// never stored: the cluster fills in what only it knows at the moment of the
// read, the operator fills in the network and the disks of the machine in front
// of them, and pushes the result to it.
//
// Served by the aggregated apiserver, never by a CRD: a stored object would
// hold a live bootstrap token in etcd and go stale against its NodeGroup.
//
// +kubebuilder:object:root=true
type NodeConfigTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NodeSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
type NodeConfigTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeConfigTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeConfigTemplate{}, &NodeConfigTemplateList{})
}
