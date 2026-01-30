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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func AddToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{
			Group:   "deckhouse.io",
			Version: "v1",
			Kind:    "NodeGroup",
		},
		&NodeGroup{},
	)
	return nil
}

// NodeGroup - упрощенная версия, только нужные поля
type NodeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            NodeGroupStatus `json:"status,omitempty"`
}

type NodeGroupStatus struct {
	Ready int32 `json:"ready,omitempty"`
}

// type nodeGroupKind struct{}

// func (in *NodeGroup) GetObjectKind() schema.ObjectKind {
// 	return &nodeGroupKind{}
// }

// func (f *nodeGroupKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
// func (f *nodeGroupKind) GroupVersionKind() schema.GroupVersionKind {
// 	return schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"}
// }

// DeepCopyInto - ручная реализация
func (in *NodeGroup) DeepCopyInto(out *NodeGroup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy
func (in *NodeGroup) DeepCopy() *NodeGroup {
	if in == nil {
		return nil
	}
	out := new(NodeGroup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject
func (in *NodeGroup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto для Spec
func (in *NodeGroupStatus) DeepCopyInto(out *NodeGroupStatus) {
	*out = *in
}

// DeepCopy
func (in *NodeGroupStatus) DeepCopy() *NodeGroupStatus {
	if in == nil {
		return nil
	}
	out := new(NodeGroupStatus)
	in.DeepCopyInto(out)
	return out
}
