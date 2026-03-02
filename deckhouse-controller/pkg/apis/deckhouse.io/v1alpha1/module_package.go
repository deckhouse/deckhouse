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
	ModulePackageResource = "modulepackages"
	ModulePackageKind     = "ModulePackage"
)

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
// +kubebuilder:printcolumn:name="UsedBy",type=integer,JSONPath=`.status.usedByCount`

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

type ModulePackageStatus struct {
	// Information about modules using this package.
	// +optional
	UsedBy []ModulePackageStatusInstance `json:"usedBy,omitempty"`

	// Number of modules using this package.
	// +optional
	UsedByCount int `json:"usedByCount,omitempty"`

	// List of repository names where this module package is available.
	// +optional
	AvailableRepositories []string `json:"availableRepositories,omitempty"`
}

type ModulePackageStatusInstance struct {
	// Namespace where the module is installed.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the module instance.
	// +optional
	Name string `json:"name,omitempty"`

	// Version of the package used by this module.
	// +optional
	Version string `json:"version,omitempty"`
}

// IsModuleInstalled checks if a specific module is installed in the given namespace.
func (m *ModulePackage) IsModuleInstalled(namespace string, moduleName string) bool {
	if len(m.Status.UsedBy) == 0 {
		return false
	}

	for _, v := range m.Status.UsedBy {
		if v.Namespace == namespace && v.Name == moduleName {
			return true
		}
	}

	return false
}

// GetModuleVersion returns the version of an installed module, or empty string if not found.
func (m *ModulePackage) GetModuleVersion(namespace string, moduleName string) string {
	for _, v := range m.Status.UsedBy {
		if v.Namespace == namespace && v.Name == moduleName {
			return v.Version
		}
	}

	return ""
}

// UpdateModuleVersion updates the version for an installed module. Returns true if updated.
func (m *ModulePackage) UpdateModuleVersion(namespace, moduleName, version string) bool {
	for i := range m.Status.UsedBy {
		if m.Status.UsedBy[i].Namespace == namespace && m.Status.UsedBy[i].Name == moduleName {
			if m.Status.UsedBy[i].Version != version {
				m.Status.UsedBy[i].Version = version
				return true
			}
			return false
		}
	}

	return false
}

// AddInstalledModule adds a module to the list of modules using this package.
func (m *ModulePackage) AddInstalledModule(namespace string, moduleName string, version string) *ModulePackage {
	instance := ModulePackageStatusInstance{
		Namespace: namespace,
		Name:      moduleName,
		Version:   version,
	}

	m.Status.UsedBy = append(m.Status.UsedBy, instance)

	m.Status.UsedByCount++

	return m
}

// RemoveInstalledModule removes a module from the list of modules using this package.
func (m *ModulePackage) RemoveInstalledModule(namespace string, moduleName string) *ModulePackage {
	prevLen := len(m.Status.UsedBy)
	m.Status.UsedBy = slices.DeleteFunc(m.Status.UsedBy, func(v ModulePackageStatusInstance) bool {
		return v.Namespace == namespace && v.Name == moduleName
	})

	if len(m.Status.UsedBy) < prevLen && m.Status.UsedByCount > 0 {
		m.Status.UsedByCount--
	}

	return m
}

// +kubebuilder:object:root=true

// ModulePackageList is a list of ModulePackage resources
type ModulePackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModulePackage `json:"items"`
}
