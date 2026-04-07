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
	"slices"

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
// +kubebuilder:printcolumn:name="UsedBy",type=integer,JSONPath=`.status.usedByCount`

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
	// Information about applications using this package.
	// +optional
	UsedBy []ApplicationPackageStatusInstance `json:"usedBy,omitempty"`

	// Number of applications using this package.
	// +optional
	UsedByCount int `json:"usedByCount,omitempty"`

	// List of repository names where this application package is available.
	// +optional
	AvailableRepositories []string `json:"availableRepositories,omitempty"`
}

type ApplicationPackageStatusInstance struct {
	// Namespace where the application is installed.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the application instance.
	// +optional
	Name string `json:"name,omitempty"`

	// Version of the package used by this application.
	// +optional
	Version string `json:"version,omitempty"`
}

// IsAppInstalled checks if a specific application is installed in the given namespace.
func (a *ApplicationPackage) IsAppInstalled(namespace string, appName string) bool {
	if len(a.Status.UsedBy) == 0 {
		return false
	}

	for _, v := range a.Status.UsedBy {
		if v.Namespace == namespace && v.Name == appName {
			return true
		}
	}

	return false
}

// GetAppVersion returns the version of an installed app, or empty string if not found.
func (a *ApplicationPackage) GetAppVersion(namespace string, appName string) string {
	for _, v := range a.Status.UsedBy {
		if v.Namespace == namespace && v.Name == appName {
			return v.Version
		}
	}

	return ""
}

// UpdateAppVersion updates the version for an installed app. Returns true if updated.
func (a *ApplicationPackage) UpdateAppVersion(namespace, appName, version string) bool {
	for i := range a.Status.UsedBy {
		if a.Status.UsedBy[i].Namespace == namespace && a.Status.UsedBy[i].Name == appName {
			if a.Status.UsedBy[i].Version != version {
				a.Status.UsedBy[i].Version = version
				return true
			}
			return false
		}
	}

	return false
}

// AddInstalledApp adds an application to the list of applications using this package.
func (a *ApplicationPackage) AddInstalledApp(namespace string, appName string, version string) *ApplicationPackage {
	apStatusInstalledApp := ApplicationPackageStatusInstance{
		Namespace: namespace,
		Name:      appName,
		Version:   version,
	}

	a.Status.UsedBy = append(a.Status.UsedBy, apStatusInstalledApp)

	a.Status.UsedByCount++

	return a
}

// RemoveInstalledApp removes an application from the list of applications using this package.
func (a *ApplicationPackage) RemoveInstalledApp(namespace string, appName string) *ApplicationPackage {
	prevLen := len(a.Status.UsedBy)
	a.Status.UsedBy = slices.DeleteFunc(a.Status.UsedBy, func(v ApplicationPackageStatusInstance) bool {
		return v.Namespace == namespace && v.Name == appName
	})

	if len(a.Status.UsedBy) < prevLen && a.Status.UsedByCount > 0 {
		a.Status.UsedByCount--
	}

	return a
}

// +kubebuilder:object:root=true

// ApplicationPackageList is a list of ApplicationPackage resources
type ApplicationPackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ApplicationPackage `json:"items"`
}
