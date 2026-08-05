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

// Resource and kind names of ModulePackage.
const (
	ModulePackageResource = "modulepackages"
	ModulePackageKind     = "ModulePackage"
)

// Group-version identifiers of ModulePackage.
var (
	ModulePackageGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModulePackageResource,
	}
	ModulePackageGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModulePackageKind,
	}
)

var _ runtime.Object = (*ModulePackage)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +crd-enricher:raw:properties.apiVersion.description="APIVersion defines the versioned schema of this representation of an object.\nServers should convert recognized schemas to the latest internal value, and\nmay reject unrecognized values.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)"
// +crd-enricher:raw:properties.kind.description="Kind is a string value representing the REST resource this object represents.\nServers may infer this from the endpoint the client submits requests to.\nCannot be updated.\nIn CamelCase.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)"

// ModulePackage represents information about available module package.
type ModulePackage struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Status of a ModulePackage.
	Status ModulePackageStatus `json:"status,omitempty"`
}

// ModulePackageStatus reports the repositories offering the package.
type ModulePackageStatus struct {
	// List of repository names where this module package is available.
	// +optional
	AvailableRepositories []string `json:"availableRepositories,omitempty"`
}

// +kubebuilder:object:root=true

// ModulePackageList is a list of ModulePackage resources
type ModulePackageList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ModulePackage `json:"items"`
}
