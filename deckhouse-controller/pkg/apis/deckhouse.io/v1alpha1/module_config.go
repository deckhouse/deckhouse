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
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ModuleConfigResource = "moduleconfigs"
	ModuleConfigKind     = "ModuleConfig"

	ModuleConfigAnnotationAllowDisable = "modules.deckhouse.io/allow-disabling"

	// TODO: remove after 1.73+
	ModuleConfigFinalizerOld = "modules.deckhouse.io/module-config"
	ModuleConfigFinalizer    = "modules.deckhouse.io/module-registered"

	ModuleConfigMessageUnknownModule = "Ignored: unknown module name"
)

var (
	// ModuleConfigGVR GroupVersionResource
	ModuleConfigGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModuleConfigResource,
	}
	ModuleConfigGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModuleConfigKind,
	}
)

var _ runtime.Object = (*ModuleConfig)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=mc
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=.spec.enabled,description="Module enabled state"
// +kubebuilder:printcolumn:name="UpdatePolicy",type=string,JSONPath=.spec.updatePolicy,description="The update policy of the module.",priority=1
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=.spec.source,description="The source of the module.",priority=1
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=.status.version,description="Version of settings schema in use"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=.metadata.creationTimestamp,description="CreationTimestamp is a timestamp representing the server time when this object was created. It is not guaranteed to be set in happens-before order across separate operations. Clients may not set this value. It is represented in RFC3339 form and is in UTC. Populated by the system. Read-only. Null for lists. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata"
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=.status.message,description="Additional information"
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/name=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/part-of=deckhouse"
// +kubebuilder:metadata:labels="backup.deckhouse.io/cluster-config=true"

// ModuleConfig defines the configuration of the Deckhouse Kubernetes Platform module (module parameters).
// The name of the ModuleConfig resource must match the name of the module (for example, `control-plane-manager` for the `control-plane-manager` module).
type ModuleConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec ModuleConfigSpec `json:"spec"`

	Status ModuleConfigStatus `json:"status,omitempty"`
}

type ModuleConfigSpec struct {
	// Version of settings schema.
	// +optional
	Version int `json:"version,omitempty"`
	// Enables or disables the module.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Module update policy.
	// +optional
	UpdatePolicy string `json:"updatePolicy,omitempty"`
	// The source of the module it provided by one (otherwise empty).
	// +optional
	Source string `json:"source,omitempty"`
	// Defines the module maintenance mode.
	//
	// - `NoResourceReconciliation`: A mode for developing or tweaking the module.
	//
	//   In this mode:
	//
	//   - Configuration or hook changes are not reconciled, which prevents resources from being updated automatically.
	//   - Resource monitoring is disabled, which prevents deleted resources from being restored.
	//   - All the module's resources are labeled with `maintenance: NoResourceReconciliation`.
	//   - The `ModuleIsInMaintenanceMode` alert is triggered.
	// +optional
	// +kubebuilder:validation:Enum=NoResourceReconciliation
	Maintenance string `json:"maintenance,omitempty"`
	// Module settings.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Settings *MappedFields `json:"settings,omitempty"`
}

type ModuleConfigStatus struct {
	// Version of settings schema in use
	Version string `json:"version"`
	// Additional information
	Message string `json:"message"`
}

func (m *ModuleConfig) IsEnabled() bool {
	if m.Spec.Enabled != nil {
		return *m.Spec.Enabled
	}
	return false
}

// +kubebuilder:object:root=true

// ModuleConfigList is a list of ModuleConfig resources
type ModuleConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleConfig `json:"items"`
}

// MappedFields handles arbitrary JSON settings as a map[string]any.
// +kubebuilder:pruning:XPreserveUnknownFields
type MappedFields runtime.RawExtension // map[string]any

// MarshalJSON implements json.Marshaler
func (v MappedFields) MarshalJSON() ([]byte, error) {
	if v.Raw != nil {
		return v.Raw, nil
	}
	return []byte("{}"), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (v *MappedFields) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	v.Raw = make([]byte, len(data))
	copy(v.Raw, data)
	return nil
}

func (v *MappedFields) DeepCopy() *MappedFields {
	if v == nil {
		return nil
	}
	out := new(MappedFields)
	v.DeepCopyInto(out)
	return out
}

func (v *MappedFields) DeepCopyInto(out *MappedFields) {
	if v.Raw != nil {
		out.Raw = make([]byte, len(v.Raw))
		copy(out.Raw, v.Raw)
	} else {
		out.Raw = nil
	}
	if v.Object != nil {
		out.Object = v.Object.DeepCopyObject()
	} else {
		out.Object = nil
	}
}

func (v *MappedFields) IsEmpty() bool {
	return v == nil || (len(v.Raw) == 0)
}

func (v *MappedFields) GetMap() map[string]any {
	if v.IsEmpty() {
		return map[string]any{}
	}

	var result map[string]any
	err := json.Unmarshal(v.Raw, &result)
	if err != nil {
		return map[string]any{}
	}

	return result
}

func MakeMappedFields(settings map[string]any) *MappedFields {
	if len(settings) == 0 {
		return nil
	}

	raw, err := json.Marshal(settings)
	if err != nil {
		return &MappedFields{Raw: []byte("{\"error\": \"failed to marshal settings\"}")}
	}

	return &MappedFields{Raw: raw}
}
