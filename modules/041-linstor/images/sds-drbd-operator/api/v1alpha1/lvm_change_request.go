/*
Copyright 2023 Flant JSC

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

// LVMChangeRequest, request to change lvm
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LVMChangeRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	PVCreate `json:"pvcreate,omitempty"`
	PVAlign  `json:"pvalign,omitempty"`
	VGCreate `json:"vgcreate,omitempty"`
	VGExtend `json:"vgextend,omitempty"`
	Status   `json:"status,omitempty"`
}

// LVMChangeRequestList contains a list of empty block device
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LVMChangeRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []LVMChangeRequest `json:"items"`
}

type PVCreate struct {
	ConsumableBlockDeviceName string `json:"consumableblockbevicename"`
}

type PVAlign struct {
	NodeName string `json:"nodename"`
	Path     string `json:"path"`
}

type VGCreate struct {
	NodeName string   `json:"nodename"`
	Name     string   `json:"name"`
	Shared   bool     `json:"shared"`
	Paths    []string `json:"paths"`
}

type VGExtend struct {
	NodeName string `json:"nodename"`
	Name     string `json:"name"`
	Path     string `json:"path"`
}

type Status struct {
	Phase  string `json:"phase"`
	Reason string `json:"reason"`
}
