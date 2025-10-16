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
	ApplicationPackageVersionResource = "applicationpackageversions"
	ApplicationPackageVersionKind     = "ApplicationPackageVersion"

	ApplicationPackageVersionLabelDraft      = "draft"
	ApplicationPackageVersionLabelPackage    = "package"
	ApplicationPackageVersionLabelRepository = "repository"
)

var (
	ApplicationPackageVersionGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ApplicationPackageVersionResource,
	}
	ApplicationPackageVersionGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ApplicationPackageVersionKind,
	}
)

var _ runtime.Object = (*ApplicationPackageVersion)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ApplicationPackageVersion represents a version of an application package.
type ApplicationPackageVersion struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Status of an ApplicationPackageVersion.
	Status ApplicationPackageVersionStatus `json:"status,omitempty"`
}

type ApplicationPackageVersionStatus struct {
	PackageName string                                     `json:"packageName,omitempty"`
	Version     string                                     `json:"version,omitempty"`
	Metadata    *ApplicationPackageVersionStatusMetadata   `json:"metadata,omitempty"`
}

type ApplicationPackageVersionStatusMetadata struct {
	Description   map[string]string                 `json:"description,omitempty"`
	Category      string                            `json:"category,omitempty"`
	Stage         string                            `json:"stage,omitempty"`
	Requirements  *PackageRequirements              `json:"requirements,omitempty"`
	Licensing     *PackageLicensing                 `json:"licensing,omitempty"`
	Changelog     *PackageChangelog                 `json:"changelog,omitempty"`
	Compatibility *PackageVersionCompatibilityRules `json:"versionCompatibilityRules,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationPackageVersionList is a list of ApplicationPackageVersion resources
type ApplicationPackageVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ApplicationPackageVersion `json:"items"`
}

