package v1alpha1

import (
	"fmt"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

const (
	ModuleSettingsResource = "modulesettings"
	ModuleSettingsKind     = "ModuleSettings"
)

var (
	// ModuleSettingsGVR GroupVersionResource
	ModuleSettingsGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModuleSettingsResource,
	}
	ModuleSettingsGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModuleSettingsKind,
	}
)

var _ runtime.Object = (*ModuleConfig)(nil)

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleSettingsList is a list of ModuleSettings resources
type ModuleSettingsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleSettings `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleSettings is a configuration for module or for global config values.
type ModuleSettings struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleSettingsSpec `json:"spec"`
}

type ModuleSettingsSpec struct {
	Versions []ModuleSettingsVersion `json:"versions"`
}

type ModuleSettingsVersion struct {
	Name   string                                    `json:"name"`
	Schema *apiextensionsv1.CustomResourceValidation `json:"schema,omitempty"`
}

// SetVersion adds or updates a version in the ModuleSettingsSpec.
func (s *ModuleSettings) SetVersion(rawSchema []byte) error {
	if rawSchema == nil {
		return nil
	}

	type schemaVersion struct {
		Version string `json:"x-config-version"`
		apiextensionsv1.JSONSchemaProps
	}

	jsonSchema := &schemaVersion{
		Version: "1",
	}
	if err := yaml.Unmarshal(rawSchema, jsonSchema); err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}

	version := ModuleSettingsVersion{
		Name:   jsonSchema.Version,
		Schema: &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: &jsonSchema.JSONSchemaProps},
	}

	for i, v := range s.Spec.Versions {
		if v.Name == jsonSchema.Version {
			s.Spec.Versions[i] = version
			return nil
		}
	}

	s.Spec.Versions = append(s.Spec.Versions, version)
	return nil
}
