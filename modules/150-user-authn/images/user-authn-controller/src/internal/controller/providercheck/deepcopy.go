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

package providercheck

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto copies the receiver into out. in must be non-nil.
func (in *DexProviderCheck) DeepCopyInto(out *DexProviderCheck) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy copies the receiver.
func (in *DexProviderCheck) DeepCopy() *DexProviderCheck {
	if in == nil {
		return nil
	}
	out := new(DexProviderCheck)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject copies the receiver as a runtime.Object.
func (in *DexProviderCheck) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out. in must be non-nil.
func (in *DexProviderCheckList) DeepCopyInto(out *DexProviderCheckList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]DexProviderCheck, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy copies the receiver.
func (in *DexProviderCheckList) DeepCopy() *DexProviderCheckList {
	if in == nil {
		return nil
	}
	out := new(DexProviderCheckList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject copies the receiver as a runtime.Object.
func (in *DexProviderCheckList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out. in must be non-nil.
func (in *DexProviderCheckStatus) DeepCopyInto(out *DexProviderCheckStatus) {
	*out = *in
	if in.Checks != nil {
		out.Checks = make([]DexProviderCheckStepStatus, len(in.Checks))
		copy(out.Checks, in.Checks)
	}
	if in.CompletedAt != nil {
		t := *in.CompletedAt
		out.CompletedAt = &t
	}
}
