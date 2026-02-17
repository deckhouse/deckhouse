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

package v1alpha2

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto copies receiver into out.
func (in *Instance) DeepCopyInto(out *Instance) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy creates a deep copy.
func (in *Instance) DeepCopy() *Instance {
	if in == nil {
		return nil
	}
	out := new(Instance)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a deep copy runtime object.
func (in *Instance) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies receiver into out.
func (in *InstanceList) DeepCopyInto(out *InstanceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		inItems, outItems := &in.Items, &out.Items
		*outItems = make([]Instance, len(*inItems))
		for i := range *inItems {
			(*inItems)[i].DeepCopyInto(&(*outItems)[i])
		}
	}
}

// DeepCopy creates a deep copy.
func (in *InstanceList) DeepCopy() *InstanceList {
	if in == nil {
		return nil
	}
	out := new(InstanceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a deep copy runtime object.
func (in *InstanceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies receiver into out.
func (in *InstanceStatus) DeepCopyInto(out *InstanceStatus) {
	*out = *in
	if in.Conditions != nil {
		inConditions, outConditions := &in.Conditions, &out.Conditions
		*outConditions = make([]InstanceCondition, len(*inConditions))
		copy(*outConditions, *inConditions)
	}
}

// DeepCopyInto copies receiver into out.
func (in *InstanceSpec) DeepCopyInto(out *InstanceSpec) {
	*out = *in
	if in.MachineRef != nil {
		inMachineRef, outMachineRef := &in.MachineRef, &out.MachineRef
		*outMachineRef = new(MachineRef)
		**outMachineRef = **inMachineRef
	}
	if in.ClassReference != nil {
		inClassReference, outClassReference := &in.ClassReference, &out.ClassReference
		*outClassReference = new(ClassReference)
		**outClassReference = **inClassReference
	}
}
