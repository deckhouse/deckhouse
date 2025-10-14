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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ClusterApplicationPackageResource = "clusterapplicationpackages"
	ClusterApplicationPackageKind     = "ClusterApplicationPackage"
)

var (
	ClusterApplicationPackageGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ClusterApplicationPackageResource,
	}
	ClusterApplicationPackageGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ClusterApplicationPackageKind,
	}
)

var _ runtime.Object = (*ClusterApplicationPackage)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ClusterApplicationPackage represents information about available cluster application package.
type ClusterApplicationPackage struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Status of a ClusterApplicationPackage.
	Status ClusterApplicationPackageStatus `json:"status,omitempty"`
}

type ClusterApplicationPackageStatus struct {
	AvailableRepositories []string `json:"availableRepositories,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterApplicationPackageList is a list of ClusterApplicationPackage resources
type ClusterApplicationPackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterApplicationPackage `json:"items"`
}

