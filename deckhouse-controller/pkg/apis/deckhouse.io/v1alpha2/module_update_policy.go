/*
Copyright 2024 Flant JSC

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

package v1alpha2

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
)

const (
	ModuleUpdatePolicyResource = "moduleupdatepolicies"
	ModuleUpdatePolicyKind     = "ModuleUpdatePolicy"

	ModuleUpdatePolicyModeIgnore = "Ignore"
)

var (
	ModuleUpdatePolicyGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModuleUpdatePolicyResource,
	}
	ModuleUpdatePolicyGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModuleUpdatePolicyKind,
	}
)

var _ runtime.Object = (*ModuleUpdatePolicy)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// ModuleUpdatePolicy source
type ModuleUpdatePolicy struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleUpdatePolicySpec `json:"spec"`
}

type ModuleUpdatePolicySpec struct {
	Update         ModuleUpdatePolicySpecUpdate `json:"update"`
	ReleaseChannel string                       `json:"releaseChannel"`
}

type ModuleUpdatePolicySpecUpdate struct {
	Mode    string         `json:"mode"`
	Windows update.Windows `json:"windows"`
}

// +kubebuilder:object:root=true

// ModuleUpdatePolicyList is a list of ModuleUpdatePolicy resources
type ModuleUpdatePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleUpdatePolicy `json:"items"`
}

// Update mode consts

type UpdateMode string

const (
	// UpdateModeAutoPatch is default mode for updater,
	// deckhouse automatically applies patch releases, but asks for approval of minor releases
	UpdateModeAutoPatch UpdateMode = "AutoPatch"
	// UpdateModeAuto is updater mode when deckhouse automatically applies all releases
	UpdateModeAuto UpdateMode = "Auto"
	// UpdateModeManual is updater mode when deckhouse downloads releases info, but does not apply them
	UpdateModeManual UpdateMode = "Manual"
)

var updateModeMap = map[UpdateMode]string{
	UpdateModeAutoPatch: string(UpdateModeAutoPatch),
	UpdateModeAuto:      string(UpdateModeAuto),
	UpdateModeManual:    string(UpdateModeManual),
}

// String implements the Stringer interface.
func (x UpdateMode) String() string {
	if str, ok := updateModeMap[x]; ok {
		return str
	}
	return fmt.Sprintf("UpdateMode(%s)", string(x))
}

// IsValid provides a quick way to determine if the typed value is
// part of the allowed enumerated values
func (x UpdateMode) IsValid() bool {
	_, ok := updateModeMap[x]
	return ok
}

var updateModeValue = map[string]UpdateMode{
	string(UpdateModeAutoPatch): UpdateModeAutoPatch,
	string(UpdateModeAuto):      UpdateModeAuto,
	string(UpdateModeManual):    UpdateModeManual,
}

// ParseUpdateMode attempts to convert a string to a UpdateMode.
//
// AutoPatch used by default
func ParseUpdateMode(name string) UpdateMode {
	if x, ok := updateModeValue[name]; ok {
		return x
	}

	return UpdateModeAutoPatch
}
