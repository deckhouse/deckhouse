/*
Copyright 2025 Flant JSC

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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ApplicationPackageResource = "applicationpackages"
	ApplicationPackageKind     = "ApplicationPackage"
)

var (
	ApplicationPackageGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ApplicationPackageResource,
	}
	ApplicationPackageGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ApplicationPackageKind,
	}
)

var _ runtime.Object = (*ApplicationPackage)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ApplicationPackage represents information about available application package.
type ApplicationPackage struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Status of an ApplicationPackage.
	Status ApplicationPackageStatus `json:"status,omitempty"`
}

type ApplicationPackageStatus struct {
	Instances             *ApplicationPackageStatusInstances `json:"instances,omitempty"`
	AvailableRepositories []string                           `json:"availableRepositories,omitempty"`
}

type ApplicationPackageStatusInstances struct {
	Count      int      `json:"count,omitempty"`
	Namespaces []string `json:"namespaces,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationPackageList is a list of ApplicationPackage resources
type ApplicationPackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ApplicationPackage `json:"items"`
}
