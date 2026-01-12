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
// +kubebuilder:printcolumn:name="Installed",type=integer,JSONPath=`.status.installedOverall`

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

type NamespaceName string

type ApplicationPackageStatus struct {
	// Information about applications installed in each namespace.
	// The key is the namespace name, and the value is a list of installed applications.
	// +optional
	Installed map[NamespaceName][]ApplicationPackageStatusInstalled `json:"installed,omitempty"`

	// Total number of applications installed across all namespaces.
	// +optional
	InstalledOverall int `json:"installedOverall,omitempty"`

	// List of repository names where this application package is available.
	// +optional
	AvailableRepositories []string `json:"availableRepositories,omitempty"`
}

type ApplicationPackageStatusInstalled struct {
	// Name of the installed application.
	// +optional
	Name string `json:"name,omitempty"`
}

// IsAppInstalled checks if a specific application is installed in the given namespace.
func (a *ApplicationPackage) IsAppInstalled(namespace string, appName string) bool {
	if len(a.Status.Installed) == 0 {
		return false
	}

	for _, v := range a.Status.Installed[NamespaceName(namespace)] {
		if v.Name == appName {
			return true
		}
	}

	return false
}

// AddInstalledApp adds an application to the list of installed applications in the specified namespace.
func (a *ApplicationPackage) AddInstalledApp(namespace string, appName string) *ApplicationPackage {
	apStatusInstalledApp := ApplicationPackageStatusInstalled{Name: appName}

	// initialize map if it is nil or empty
	if len(a.Status.Installed) == 0 {
		a.Status.Installed = make(map[NamespaceName][]ApplicationPackageStatusInstalled)
	}

	a.Status.Installed[NamespaceName(namespace)] = append(a.Status.Installed[NamespaceName(namespace)], apStatusInstalledApp)

	a.Status.InstalledOverall++

	return a
}

// RemoveInstalledApp removes an application from the list of installed applications in the specified namespace.
func (a *ApplicationPackage) RemoveInstalledApp(namespace string, appName string) *ApplicationPackage {
	if len(a.Status.Installed) == 0 {
		return a
	}

	newSlice := slices.DeleteFunc(a.Status.Installed[NamespaceName(namespace)], func(v ApplicationPackageStatusInstalled) bool {
		return v.Name == appName
	})

	a.Status.Installed[NamespaceName(namespace)] = newSlice

	if len(a.Status.Installed[NamespaceName(namespace)]) == 0 {
		delete(a.Status.Installed, NamespaceName(namespace))
	}

	if a.Status.InstalledOverall > 0 {
		a.Status.InstalledOverall--
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
