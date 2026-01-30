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
	scheme.AddKnownTypes(
		schema.GroupVersion{
			Group:   "deckhouse.io",
			Version: "v1",
		},
		&NodeGroup{},
		&NodeGroupList{},
	)
	return nil
}

type NodeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            NodeGroupStatus `json:"status,omitempty"`
}

type NodeGroupStatus struct {
	Ready int32 `json:"ready,omitempty"`
}

type NodeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeGroup `json:"items"`
}

func (in *NodeGroup) DeepCopyInto(out *NodeGroup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *NodeGroup) DeepCopy() *NodeGroup {
	if in == nil {
		return nil
	}
	out := new(NodeGroup)
	in.DeepCopyInto(out)
	return out
}

func (in *NodeGroup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *NodeGroupStatus) DeepCopyInto(out *NodeGroupStatus) {
	*out = *in
}

func (in *NodeGroupStatus) DeepCopy() *NodeGroupStatus {
	if in == nil {
		return nil
	}
	out := new(NodeGroupStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *NodeGroupList) DeepCopyInto(out *NodeGroupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NodeGroup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *NodeGroupList) DeepCopy() *NodeGroupList {
	if in == nil {
		return nil
	}
	out := new(NodeGroupList)
	in.DeepCopyInto(out)
	return out
}

func (in *NodeGroupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
