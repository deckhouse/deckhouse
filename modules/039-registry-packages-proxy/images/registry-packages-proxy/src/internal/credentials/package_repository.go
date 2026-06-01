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

// from https://github.com/deckhouse/deckhouse/blob/main/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1/package_repository.go

package credentials

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	PackageRepositoryResource = "packagerepositories"
	PackageRepositoryKind     = "PackageRepository"
)

var (
	PackageRepositoryGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: PackageRepositoryResource,
	}
	PackageRepositoryGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    PackageRepositoryKind,
	}
)

var _ runtime.Object = (*PackageRepository)(nil)

// PackageRepository is a source of packages for Deckhouse.
type PackageRepository struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageRepositorySpec   `json:"spec"`
	Status PackageRepositoryStatus `json:"status,omitempty"`
}

type PackageRepositorySpec struct {
	// +optional
	ScanInterval *metav1.Duration              `json:"scanInterval,omitempty"`
	Registry     PackageRepositorySpecRegistry `json:"registry"`
}

type PackageRepositorySpecRegistry struct {
	// +optional
	Scheme string `json:"scheme,omitempty"`

	Repo string `json:"repo"`

	// +optional
	DockerCFG string `json:"dockerCfg,omitempty"`

	// +optional
	CA string `json:"ca,omitempty"`

	// +optional
	Login string `json:"login,omitempty"`

	// +optional
	Password string `json:"password,omitempty"`
}

type PackageRepositoryStatus struct {
	// +optional
	SyncTime metav1.Time `json:"syncTime,omitempty"`

	// +optional
	Packages []PackageRepositoryStatusPackage `json:"packages,omitempty"`

	// +optional
	PackagesCount int `json:"packagesCount,omitempty"`

	// +optional
	Phase string `json:"phase,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// +optional
	PartialScanAvailable bool `json:"partialScanAvailable"`
}

type PackageRepositoryStatusPackage struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// PackageRepositoryList is a list of PackageRepository resources
type PackageRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PackageRepository `json:"items"`
}

// DeepCopyInto copies the receiver into out. in must be non-nil.
func (in *PackageRepository) DeepCopyInto(out *PackageRepository) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy creates a new PackageRepository by copying the receiver.
func (in *PackageRepository) DeepCopy() *PackageRepository {
	if in == nil {
		return nil
	}
	out := new(PackageRepository)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a new runtime.Object by copying the receiver.
func (in *PackageRepository) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out. in must be non-nil.
func (in *PackageRepositoryList) DeepCopyInto(out *PackageRepositoryList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PackageRepository, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy creates a new PackageRepositoryList by copying the receiver.
func (in *PackageRepositoryList) DeepCopy() *PackageRepositoryList {
	if in == nil {
		return nil
	}
	out := new(PackageRepositoryList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a new runtime.Object by copying the receiver.
func (in *PackageRepositoryList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out. in must be non-nil.
func (in *PackageRepositorySpec) DeepCopyInto(out *PackageRepositorySpec) {
	*out = *in
	if in.ScanInterval != nil {
		in, out := &in.ScanInterval, &out.ScanInterval
		*out = new(metav1.Duration)
		**out = **in
	}
	out.Registry = in.Registry
}

// DeepCopy creates a new PackageRepositorySpec by copying the receiver.
func (in *PackageRepositorySpec) DeepCopy() *PackageRepositorySpec {
	if in == nil {
		return nil
	}
	out := new(PackageRepositorySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out. in must be non-nil.
func (in *PackageRepositoryStatus) DeepCopyInto(out *PackageRepositoryStatus) {
	*out = *in
	in.SyncTime.DeepCopyInto(&out.SyncTime)
	if in.Packages != nil {
		in, out := &in.Packages, &out.Packages
		*out = make([]PackageRepositoryStatusPackage, len(*in))
		copy(*out, *in)
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy creates a new PackageRepositoryStatus by copying the receiver.
func (in *PackageRepositoryStatus) DeepCopy() *PackageRepositoryStatus {
	if in == nil {
		return nil
	}
	out := new(PackageRepositoryStatus)
	in.DeepCopyInto(out)
	return out
}
